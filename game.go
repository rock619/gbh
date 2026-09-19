package gbh

import (
	"bytes"
	"errors"
	"image"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	ebitenaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/rock619/gbh/audio"
	"github.com/rock619/gbh/cartridge"
	"github.com/rock619/gbh/cpu"
	"github.com/rock619/gbh/graphic"
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/joypad"
	"github.com/rock619/gbh/memory"
	"github.com/rock619/gbh/serial"
	"github.com/rock619/gbh/timer"
)

var mplusFaceSource *text.GoTextFaceSource

func init() {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.MPlus1pRegular_ttf))
	if err != nil {
		log.Fatal(err)
	}
	mplusFaceSource = s
}

const (
	originalW, originalH = 160, 144
	gameViewScale        = 3
	gameViewW            = originalW * gameViewScale
	gameViewH            = originalH * gameViewScale
	debugPanelW          = 360
	screenW              = gameViewW + debugPanelW
	screenH              = gameViewH
	cyclesPerFrame       = 70224
	cpuClockRate         = 4194304
)

// Game implements ebiten.Game interface
type Game struct {
	cpu  *cpu.CPU
	mmu  *memory.MMU
	cart cartridge.Cartridge
	ppu  *graphic.PPU
	apu  *audio.APU
	j    *joypad.Joypad
	t    *timer.Timer
	s    *serial.Serial

	player *ebitenaudio.Player
	ui     *debugPanel

	sub    *ebiten.Image
	keyMap KeyMap
	keys   []joypad.KeyCode

	cycleBudget float64
	lastUpdate  time.Time
	speed       float64
	volume      float64
	romFilename string

	logger *slog.Logger
}

func NewGame(rom, ram []byte, audioCtx *ebitenaudio.Context, logger *slog.Logger) *Game {
	return newGame(rom, ram, audioCtx, os.Stdout, logger)
}

func NewHeadlessGame(rom, ram []byte, logger *slog.Logger) *Game {
	return newGame(rom, ram, nil, io.Discard, logger)
}

func newGame(rom, ram []byte, audioCtx *ebitenaudio.Context, serialOut io.Writer, logger *slog.Logger) *Game {
	cart := cartridge.New(rom, ram, logger)
	ic := interrupt.New()
	j := joypad.New(ic)
	s := serial.New(ic, serialOut)
	apu := audio.NewAPU(logger)
	t := timer.New(apu, ic, logger)

	var player *ebitenaudio.Player
	if audioCtx != nil {
		var err error
		player, err = audioCtx.NewPlayer(apu.Mix)
		if err != nil {
			logger.Error("Failed to create audio player", "error", err)
		}
	}

	ppu := graphic.NewPPU(ic, logger)
	mmu := memory.NewMMU(
		cart,
		ppu,
		apu,
		j,
		ic,
		s,
		t,
		logger,
	)
	c := cpu.New(mmu, ic, []cpu.Ticker{ppu, t, apu, s}, logger)
	g := &Game{
		cpu:    c,
		mmu:    mmu,
		cart:   cart,
		ppu:    ppu,
		apu:    apu,
		j:      j,
		t:      t,
		s:      s,
		logger: logger,
		player: player,
		keyMap: defaultKeyMap,
		speed:  1.0,
		volume: 0.5,
	}
	g.apu.SetSpeed(g.speed)
	g.apu.SetVolume(g.volume)
	if audioCtx != nil {
		g.ui = newDebugPanel(g)
	}
	return g
}

func (g *Game) SetSpeed(speed float64) {
	if speed <= 0 {
		speed = 1
	}
	g.speed = speed
	g.apu.SetSpeed(speed)
}

func (g *Game) SetVolume(volume float64) {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	g.volume = volume
	g.apu.SetVolume(volume)
}

// SetROMFilename sets the name of the ROM file shown in the debug panel.
func (g *Game) SetROMFilename(filename string) {
	g.romFilename = filename
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	now := time.Now()
	if g.lastUpdate.IsZero() {
		g.lastUpdate = now
		return nil
	}

	g.keys = g.keys[:0]
	for _, keyCode := range joypadKeyOrder {
		if key, ok := g.keyMap[keyCode]; ok && ebiten.IsKeyPressed(key) {
			g.keys = append(g.keys, keyCode)
		}
	}
	g.j.SetPressed(g.keys)
	if len(g.keys) > 0 {
		g.cpu.Wake()
	}

	elapsed := now.Sub(g.lastUpdate).Seconds()
	g.lastUpdate = now
	g.cycleBudget += elapsed * cpuClockRate * g.speed
	maxCycleBudget := float64(cyclesPerFrame) * max(g.speed, 1)
	if g.cycleBudget > maxCycleBudget {
		g.cycleBudget = maxCycleBudget
	}
	for g.cycleBudget >= 4 {
		c, err := g.Step()
		if err != nil {
			if errors.Is(err, cpu.ErrStopped) {
				break // CPU hit STOP; stop advancing this frame but keep rendering
			}
			return err
		}
		g.cycleBudget -= float64(c)
	}

	if g.ui != nil {
		g.ui.Update()
	}

	if g.player != nil && !g.player.IsPlaying() {
		g.player.Play()
	}

	return nil
}

func (g *Game) Title() string {
	return g.cart.Title()
}

func (g *Game) HasBattery() bool {
	return g.cart.HasBattery()
}

func (g *Game) RAMSize() int {
	return g.cart.RAMSize()
}

func (g *Game) SaveRAM() []byte {
	return g.cart.SaveRAM()
}

func (g *Game) RunFrame() error {
	g.cycleBudget += cyclesPerFrame
	for g.cycleBudget > 0 {
		c, err := g.Step()
		if err != nil {
			if errors.Is(err, cpu.ErrStopped) {
				return nil
			}
			return err
		}
		g.cycleBudget -= float64(c)
	}
	return nil
}

func (g *Game) Framebuffer() *image.RGBA {
	return g.ppu.Framebuffer()
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	// ebiten.NewImage is expensive, so create a sub-image only once and reuse it.
	if g.sub == nil {
		g.sub = ebiten.NewImage(originalW, originalH)
	}
	screen.Fill(gameBackgroundColor)
	g.ppu.Draw(g.sub)
	op := &ebiten.DrawImageOptions{}
	scale := float64(max(1, min(gameViewW/originalW, screen.Bounds().Dy()/originalH)))
	op.GeoM.Scale(scale, scale)

	screen.DrawImage(g.sub, op)

	if g.ui != nil {
		g.ui.Draw(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return screenW, screenH
}

func (g *Game) Step() (cycles int, err error) {
	cycles, err = g.cpu.Step()
	if err != nil {
		return 0, err
	}
	return cycles, nil
}

type KeyMap map[joypad.KeyCode]ebiten.Key

var joypadKeyOrder = []joypad.KeyCode{
	joypad.KeyUp,
	joypad.KeyDown,
	joypad.KeyLeft,
	joypad.KeyRight,
	joypad.KeyA,
	joypad.KeyB,
	joypad.KeySelect,
	joypad.KeyStart,
}

var defaultKeyMap = KeyMap{
	joypad.KeyA:      ebiten.KeyX,
	joypad.KeyB:      ebiten.KeyZ,
	joypad.KeySelect: ebiten.KeyBackspace,
	joypad.KeyStart:  ebiten.KeyEnter,
	joypad.KeyRight:  ebiten.KeyArrowRight,
	joypad.KeyLeft:   ebiten.KeyArrowLeft,
	joypad.KeyUp:     ebiten.KeyArrowUp,
	joypad.KeyDown:   ebiten.KeyArrowDown,
}
