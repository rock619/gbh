package graphic

import (
	"image"
	"image/color"
	"log/slog"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/cpu"
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/register"
)

// PPU (Pixel Processing Unit) is responsible for rendering graphics
type PPU struct {
	// VRAM is 8KB
	vram [0x2000]uint8
	// OAM is 160 bytes
	oam [0xA0]uint8

	// hardware registers
	lcdc uint8
	// STAT register contains mode flags and interrupt enable bits
	// Bit 6: LYC=LY coincidence interrupt enable
	// Bit 5: Mode 2 OAM interrupt enable
	// Bit 4: Mode 1 VBlank interrupt enable
	// Bit 3: Mode 0 HBlank interrupt enable
	// Bit 2: LYC=LY coincidence flag (0: LY!=LYC, 1: LY=LYC)
	// Bit 1-0: Mode flag (0: HBlank, 1: VBlank, 2: OAM search, 3: Pixel transfer)
	stat    uint8
	scy     uint8
	scx     uint8
	ly      uint8
	lyc     uint8
	dma     uint8
	bgp     uint8
	obp0    uint8
	obp1    uint8
	wy      uint8
	wx      uint8
	key0sys uint8
	key1spd uint8
	vbk     uint8

	cycles            int
	renderedLine      [144]bool
	windowLine        uint8
	statIRQLine       bool
	spritesOnScanline []Sprite
	front             *image.RGBA // Read by Draw
	back              *image.RGBA // Written by RenderPixel

	ir     interrupt.Requester
	logger *slog.Logger
}

func NewPPU(ir interrupt.Requester, logger *slog.Logger) *PPU {
	return &PPU{
		front:  image.NewRGBA(image.Rect(0, 0, 160, 144)),
		back:   image.NewRGBA(image.Rect(0, 0, 160, 144)),
		lcdc:   0x91, // LCD enabled, BG & Window tile data at 0x8000, BG tile map at 0x9800, 8x8 sprites, sprites enabled, window disabled
		ir:     ir,
		logger: logger,
	}
}

func (p *PPU) ReadVRAM(addr uint16) uint8 {
	// VRAM is mapped to 0x8000-0x9FFF, but we store it in a 0-based array, so we need to subtract 0x8000 from the address
	return p.vram[addr-0x8000]
}

func (p *PPU) WriteVRAM(addr uint16, value uint8) {
	// VRAM is mapped to 0x8000-0x9FFF, but we store it in a 0-based array, so we need to subtract 0x8000 from the address
	p.vram[addr-0x8000] = value
}

func (p *PPU) ReadOAM(offset uint16) uint8 {
	return p.oam[offset]
}

func (p *PPU) WriteOAM(offset uint16, value uint8) {
	p.oam[offset] = value
}

func (p *PPU) ReadRegister(addr uint16) uint8 {
	switch addr {
	case register.LCDC:
		return p.lcdc
	case register.STAT:
		return p.stat | 0x80
	case register.SCY:
		return p.scy
	case register.SCX:
		return p.scx
	case register.LY:
		return p.ly
	case register.LYC:
		return p.lyc
	case register.DMA:
		return p.dma
	case register.BGP:
		return p.bgp
	case register.OBP0:
		return p.obp0
	case register.OBP1:
		return p.obp1
	case register.WY:
		return p.wy
	case register.WX:
		return p.wx
	case register.KEY0SYS:
		return p.key0sys
	case register.KEY1SPD:
		return p.key1spd
	case register.VBK:
		return p.vbk
	default:
		return 0xFF // Unused registers return 0xFF
	}
}

func (p *PPU) WriteRegister(addr uint16, value uint8) {
	switch addr {
	case register.LCDC:
		p.WriteLCDC(value)
	case register.STAT:
		p.stat = (p.stat & 0b00000111) | (value & 0b01111000)
		p.updateSTATInterrupt()
	case register.SCY:
		p.scy = value
	case register.SCX:
		p.scx = value
	case register.LY:
		// LY is read-only, ignore writes
	case register.LYC:
		p.lyc = value
		p.updateLYC()
		p.updateSTATInterrupt()
	case register.DMA:
		p.dma = value
	case register.BGP:
		p.bgp = value
	case register.OBP0:
		p.obp0 = value
	case register.OBP1:
		p.obp1 = value
	case register.WY:
		p.wy = value
	case register.WX:
		p.wx = value
	case register.KEY0SYS:
		p.key0sys = value
	case register.KEY1SPD:
		p.key1spd = value
	case register.VBK:
		p.vbk = value
	}
}

func (p *PPU) WriteLCDC(value uint8) {
	enabledBefore := p.LCDEnabled()
	enabledAfter := value&(1<<7) != 0
	p.lcdc = value

	switch {
	case enabledBefore && !enabledAfter:
		p.cycles = 0
		p.ly = 0
		p.windowLine = 0
		p.SetPPUMode(0)
		clear(p.renderedLine[:])
		p.updateLYC()
		p.updateSTATInterrupt()
	case !enabledBefore && enabledAfter:
		p.cycles = 4
		p.ly = 0
		p.windowLine = 0
		p.SetPPUMode(2)
		clear(p.renderedLine[:])
		p.SearchSprites()
		p.updateLYC()
		p.updateSTATInterrupt()
	}
}

func (p *PPU) Draw(screen *ebiten.Image) {
	screen.WritePixels(p.front.Pix)
}

func (p *PPU) Framebuffer() *image.RGBA {
	return p.front
}

// Tick advances PPU state by the given number of cycles.
func (p *PPU) Tick(cycles int) {
	if !p.LCDEnabled() {
		p.cycles = 0
		p.ly = 0
		p.SetPPUMode(0)
		return
	}

	p.cycles += cycles
	if p.cycles >= 70224 {
		p.cycles -= 70224
		p.ly = 0
		p.windowLine = 0
		clear(p.renderedLine[:])
	}

	// Update LY and STAT registers based on the current cycle count
	p.ly = uint8(p.cycles / 456)
	p.updateLYC()

	cyclesInLine := p.cycles % 456
	switch {
	case p.ly >= 144:
		// Mode 1: VBlank
		if p.PPUMode() != 1 {
			p.SetPPUMode(1)
			p.swapBuffers()
			p.ir.Request(interrupt.VBlank)
		}

	case cyclesInLine < 80:
		if p.PPUMode() != 2 {
			p.SetPPUMode(2)
			p.SearchSprites()
		}

	case cyclesInLine < 252:
		// Mode 3: Pixel transfer
		if p.PPUMode() != 3 {
			p.SetPPUMode(3)
			// No STAT interrupt for Mode 3
		}
		if !p.renderedLine[p.ly] {
			p.RenderLine()
			p.renderedLine[p.ly] = true
		}

	default:
		// Mode 0: HBlank
		if p.PPUMode() != 0 {
			p.SetPPUMode(0)
		}
	}
	p.updateSTATInterrupt()
}

func (p *PPU) SearchSprites() {
	p.spritesOnScanline = p.spritesOnScanline[:0]
	for i := range uint8(40) {
		// OAM stores OBJ Y as screenY+16, so convert before scanline tests.
		spriteY := int(p.ReadOAM(uint16(i*4))) - 16
		if spriteY <= int(p.ly) && int(p.ly) < spriteY+int(p.ObjSize()) {
			p.spritesOnScanline = append(p.spritesOnScanline, Sprite{
				id: i,
				data: [4]uint8{
					p.ReadOAM(uint16(i * 4)),
					p.ReadOAM(uint16(i*4 + 1)),
					p.ReadOAM(uint16(i*4 + 2)),
					p.ReadOAM(uint16(i*4 + 3)),
				},
			})
			if len(p.spritesOnScanline) >= 10 {
				break // Only 10 sprites can be rendered on a single scanline
			}
		}
	}
	sort.Slice(p.spritesOnScanline, func(i, j int) bool {
		if p.spritesOnScanline[i].ScreenX() == p.spritesOnScanline[j].ScreenX() {
			return p.spritesOnScanline[i].id < p.spritesOnScanline[j].id // Lower OAM index has higher priority
		}
		return p.spritesOnScanline[i].ScreenX() < p.spritesOnScanline[j].ScreenX() // Sort by X coordinate
	})
}

func (p *PPU) RenderLine() {
	if p.ly >= 144 {
		return // No rendering during VBlank
	}
	windowVisible := p.windowVisibleOnLine(p.ly)
	for x := range 160 {
		p.RenderPixel(uint8(x), p.ly, windowVisible)
	}
	if windowVisible {
		p.windowLine++
	}
}

func (p *PPU) RenderPixel(x, y uint8, windowVisible bool) {
	bgColorID := uint8(0)
	v := uint8(0)
	if p.BGEnabled() {
		bgColorID = p.BGColorIDAt(x, y)
		if windowVisible && int(x) >= p.windowStartX() {
			bgColorID = p.WindowColorIDAt(x)
		}
		v = p.BGColorOf(bgColorID)
	}
	if p.ObjEnabled() {
		for _, sprite := range p.spritesOnScanline {
			// OAM stores OBJ X as screenX+8; ScreenX converts it for pixel tests.
			spriteX := sprite.ScreenX()
			if spriteX <= int(x) && int(x) < spriteX+8 {
				spriteColor := p.SpriteColorAt(sprite, x, y)
				if spriteColor == 0 {
					continue // Color 0 is transparent for sprites
				}
				if sprite.Priority() && p.BGEnabled() && bgColorID != 0 {
					continue // Sprite is behind background/window color IDs 1-3.
				}
				// Apply sprite palette
				if sprite.Palette() == 0 {
					v = (p.obp0 >> (spriteColor * 2)) & 0b11
				} else {
					v = (p.obp1 >> (spriteColor * 2)) & 0b11
				}
				break // Stop after the first visible sprite
			}
		}
	}
	i := (int(y)*160 + int(x)) * 4
	c := p.ToRGBA(v)
	p.back.Pix[i] = c.R
	p.back.Pix[i+1] = c.G
	p.back.Pix[i+2] = c.B
	p.back.Pix[i+3] = c.A
}

func (p *PPU) BGColorIDAt(x uint8, y uint8) uint8 {
	bgX, bgY := p.MapX(x), p.MapY(y)
	tileIndex := p.ReadVRAM(p.BGTileMapBase() + uint16(bgY/8)*32 + uint16(bgX/8))
	xInTile, yInTile := bgX%8, bgY%8
	return p.TileColorIDAt(tileIndex, xInTile, yInTile)
}

func (p *PPU) WindowColorIDAt(x uint8) uint8 {
	windowX := int(x) - p.windowStartX()
	tileIndex := p.ReadVRAM(p.WindowTileMapBase() + uint16(p.windowLine/8)*32 + uint16(windowX/8))
	xInTile, yInTile := uint8(windowX%8), p.windowLine%8
	return p.TileColorIDAt(tileIndex, xInTile, yInTile)
}

func (p *PPU) TileColorIDAt(index, x, y uint8) uint8 {
	addr := p.tileDataAddr(index)
	low := p.ReadVRAM(addr + uint16(y)*2)
	high := p.ReadVRAM(addr + uint16(y)*2 + 1)
	return (high>>uint(7-x))&1<<1 | (low>>uint(7-x))&1
}

func (p *PPU) BGColorOf(id uint8) uint8 {
	return (p.bgp >> (id * 2)) & 0b11
}

func (p *PPU) PPUMode() uint8 {
	return p.stat & 0b11
}

func (p *PPU) SetPPUMode(mode uint8) {
	p.stat = (p.stat & 0b11111100) | (mode & 0b11)
}

func (p *PPU) updateLYC() {
	if p.ly == p.lyc {
		p.stat |= 0b100
	} else {
		p.stat &^= 0b100
	}
}

func (p *PPU) updateSTATInterrupt() {
	line := false
	switch p.PPUMode() {
	case 0:
		line = p.stat&(1<<3) != 0
	case 1:
		line = p.stat&(1<<4) != 0
	case 2:
		line = p.stat&(1<<5) != 0
	}
	if p.stat&(1<<6) != 0 && p.stat&(1<<2) != 0 {
		line = true
	}
	if line && !p.statIRQLine {
		p.ir.Request(interrupt.LCDStat)
	}
	p.statIRQLine = line
}

func (p *PPU) LCDCBit(bit uint8) bool {
	return p.lcdc&(1<<bit) != 0
}

func (p *PPU) LCDEnabled() bool {
	return p.LCDCBit(7)
}

func (p *PPU) WindowTileMapBase() uint16 {
	// https://gbdev.io/pandocs/LCDC.html#lcdc6--window-tile-map-area
	if p.LCDCBit(6) {
		return 0x9C00
	}
	return 0x9800
}

func (p *PPU) WindowEnabled() bool {
	return p.LCDCBit(5)
}

func (p *PPU) windowVisibleOnLine(y uint8) bool {
	return p.BGEnabled() && p.WindowEnabled() && y >= p.wy && p.wx <= 166
}

func (p *PPU) windowStartX() int {
	return int(p.wx) - 7
}

func (p *PPU) TileDataBase() uint16 {
	// https://gbdev.io/pandocs/LCDC.html#lcdc4--bg-and-window-tile-data-area
	if p.LCDCBit(4) {
		return 0x8000
	}
	return 0x9000
}

// tileDataAddr computes the VRAM address of a tile's data.
// When LCDC bit4=1 (unsigned mode), index 0-255 maps to 0x8000-0x8FF0.
// When LCDC bit4=0 (signed mode), index is treated as int8; 0 maps to 0x9000.
func (p *PPU) tileDataAddr(index uint8) uint16 {
	if p.LCDCBit(4) {
		return 0x8000 + uint16(index)*16
	}
	return uint16(int32(0x9000) + int32(int8(index))*16)
}

func (p *PPU) BGTileMapBase() uint16 {
	// https://gbdev.io/pandocs/LCDC.html#lcdc3--bg-tile-map-area
	if p.LCDCBit(3) {
		return 0x9C00
	}
	return 0x9800
}

func (p *PPU) ObjSize() uint8 {
	// https://gbdev.io/pandocs/LCDC.html#lcdc2--obj-size
	if p.LCDCBit(2) {
		return 16
	}
	return 8
}

func (p *PPU) ObjEnabled() bool {
	return p.LCDCBit(1)
}

func (p *PPU) BGEnabled() bool {
	return p.LCDCBit(0)
}

func (p *PPU) MapX(x uint8) uint8 {
	return x + p.scx
}

func (p *PPU) MapY(y uint8) uint8 {
	return y + p.scy
}

func (p *PPU) ToRGBA(colorID uint8) color.RGBA {
	// colorID is already palette-mapped by BGColorOf or the OBJ palette path.
	// Keep this function as the final shade-to-RGBA conversion only.
	switch colorID {
	case 0:
		return color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	case 1:
		return color.RGBA{R: 0xAA, G: 0xAA, B: 0xAA, A: 0xFF}
	case 2:
		return color.RGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}
	case 3:
		return color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
	default:
		return color.RGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 0xFF} // Magenta for invalid color
	}
}

func (p *PPU) SpriteColorAt(s Sprite, x, y uint8) uint8 {
	// Sprite pixels are addressed in screen coordinates, while OAM coordinates
	// include hardware offsets. Convert to tile-local coordinates first.
	xInTile := int(x) - s.ScreenX()
	yInTile := int(y) - s.ScreenY()
	if s.XFlip() {
		xInTile = 7 - xInTile
	}
	if s.YFlip() {
		yInTile = int(p.ObjSize()) - 1 - yInTile
	}

	tileIndex := s.TileIndex()
	if p.ObjSize() == 16 {
		// In 8x16 mode bit 0 of the tile number is ignored; the pair starts at
		// the even tile and the lower half uses the following tile.
		tileIndex &^= 1
		if yInTile >= 8 {
			tileIndex++
			yInTile -= 8
		}
	}
	addr := 0x8000 + uint16(tileIndex)*16
	low := p.ReadVRAM(addr + uint16(yInTile)*2)
	high := p.ReadVRAM(addr + uint16(yInTile)*2 + 1)
	id := (high>>uint(7-xInTile))&1<<1 | (low>>uint(7-xInTile))&1
	return id
}

func (p *PPU) swapBuffers() {
	p.front, p.back = p.back, p.front
}

func (p *PPU) CorruptOAM(addr uint16, kind cpu.OAMBugKind) {
	switch {
	case !p.LCDEnabled(), p.ly >= 144, addr < 0xFE00, addr > 0xFEFF:
	default:
		if x := p.cycles % 456; x < 4 || x > 76 {
			return
		}

		p.corruptOAMByTiming(kind)
	}
}

func (p *PPU) corruptOAMByTiming(kind cpu.OAMBugKind) {
	x := p.cycles % 456

	// OAM bug window. Only M-cycles 1..19 are affected.
	// M-cycle 0 is excluded because it is the row for objects 0/1.
	if x < 4 || x > 76 {
		return
	}

	row := x / 4 // 1..19

	switch kind {
	case cpu.OAMBugIncDec:
		p.writeCorruptOAMRow(row)
	case cpu.OAMBugPush:
		p.writeCorruptOAMRow(row)
	case cpu.OAMBugPop:
		p.readCorruptOAMRow(row)
	case cpu.OAMBugLdHLReadIncDec:
		p.readIncDecCorruptOAMRow(row)
	case cpu.OAMBugLdHLWriteIncDec:
		p.writeCorruptOAMRow(row)
	}
}

func (p *PPU) readIncDecCorruptOAMRow(row int) {
	if row >= 4 && row < 19 {
		base := row * 8

		a := p.oamWord(base - 16)
		b := p.oamWord(base - 8)
		c := p.oamWord(base + 0)
		d := p.oamWord(base - 4)

		p.setOAMWord(base-8, (b&(a|c|d))|(a&c&d))

		for i := 0; i < 8; i++ {
			v := p.oam[base-8+i]
			p.oam[base-16+i] = v
			p.oam[base+i] = v
		}
	}

	p.readCorruptOAMRow(row)
}

func (p *PPU) readCorruptOAMRow(row int) {
	if row <= 0 || row >= 20 {
		return
	}

	base := row * 8

	// Secondary read corruption.
	// This is needed for rows whose byte offset has bits 0x10 set.
	if base < 0x98 && base&0x18 == 0x10 {
		a := p.oamWord(base - 16) // row - 2, first word
		b := p.oamWord(base - 8)  // row - 1, first word
		c := p.oamWord(base + 0)  // current row, first word
		d := p.oamWord(base - 4)  // row - 1, third word

		v := (b & (a | c | d)) | (a & c & d)
		p.setOAMWord(base-8, v)

		// row - 2 = row - 1
		for i := 0; i < 8; i++ {
			p.oam[base-16+i] = p.oam[base-8+i]
		}

		// current row = row - 1
		for i := 0; i < 8; i++ {
			p.oam[base+i] = p.oam[base-8+i]
		}
		return
	}

	// Default read corruption.
	a := p.oamWord(base + 0)
	b := p.oamWord(base - 8)
	c := p.oamWord(base - 4)

	v := b | (a & c)
	p.setOAMWord(base-8, v)

	for i := 0; i < 8; i++ {
		p.oam[base+i] = p.oam[base-8+i]
	}
}

func (p *PPU) writeCorruptOAMRow(row int) {
	if row <= 0 || row >= 20 {
		return
	}

	base := row * 8
	prev := base - 8

	// a = current row first word
	// b = previous row first word
	// c = previous row third word
	a := p.oamWord(base + 0)
	b := p.oamWord(prev + 0)
	c := p.oamWord(prev + 4)

	// First word = ((a ^ c) & (b ^ c)) ^ c
	p.setOAMWord(base+0, ((a^c)&(b^c))^c)

	// Last three words copied from previous row
	p.setOAMWord(base+2, p.oamWord(prev+2))
	p.setOAMWord(base+4, p.oamWord(prev+4))
	p.setOAMWord(base+6, p.oamWord(prev+6))
}

func (p *PPU) oamWord(offset int) uint16 {
	return bits.Combine(p.oam[offset+1], p.oam[offset])
}

func (p *PPU) setOAMWord(offset int, value uint16) {
	p.oam[offset] = uint8(value)
	p.oam[offset+1] = uint8(value >> 8)
}

type Sprite struct {
	id   uint8
	data [4]byte
}

func (s *Sprite) Y() uint8 {
	return s.data[0]
}

func (s *Sprite) X() uint8 {
	return s.data[1]
}

func (s *Sprite) ScreenY() int {
	return int(s.Y()) - 16
}

func (s *Sprite) ScreenX() int {
	return int(s.X()) - 8
}

func (s *Sprite) TileIndex() uint8 {
	return s.data[2]
}

func (s *Sprite) Attributes() uint8 {
	return s.data[3]
}

func (s *Sprite) Priority() bool {
	return s.Attributes()&0b10000000 != 0
}

func (s *Sprite) YFlip() bool {
	return s.Attributes()&0b01000000 != 0
}

func (s *Sprite) XFlip() bool {
	return s.Attributes()&0b00100000 != 0
}

func (s *Sprite) Palette() uint8 {
	if s.Attributes()&0b00010000 != 0 {
		return 1
	}
	return 0
}
