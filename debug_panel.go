package gbh

import (
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/ebitenui/ebitenui"
	uiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/rock619/gbh/joypad"
	"github.com/rock619/gbh/register"
)

var gameBackgroundColor = color.NRGBA{R: 0x09, G: 0x0b, B: 0x0f, A: 0xff}

type debugPanel struct {
	game *Game
	ui   *ebitenui.UI

	panel *widget.Container
	face  *text.Face

	perfLabel   *widget.Text
	videoLabel  *widget.Text
	joypadLabel *widget.Text
	serialLabel *widget.Text
	audioLabel  *widget.Text
	speedLabel  *widget.Text
	volumeLabel *widget.Text
	fileLabel   *widget.Text

	dialogOpen    bool
	dialogResult  chan fileDialogResult
	dialogMessage string
}

type fileDialogResult struct {
	filename string
	err      error
}

func newDebugPanel(g *Game) *debugPanel {
	p := &debugPanel{
		game: g,
		face: debugPanelFace(14),
	}

	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(0),
		)),
	)

	spacer := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(gameViewW, screenH)),
	)

	p.panel = widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(debugPanelW, screenH),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
		),
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{R: 0x15, G: 0x19, B: 0x20, A: 0xff})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 14, Left: 16, Right: 16, Bottom: 14}),
			widget.RowLayoutOpts.Spacing(10),
		)),
	)

	p.perfLabel = p.newText("")
	p.videoLabel = p.newText("")
	p.joypadLabel = p.newText("")
	p.serialLabel = p.newText("")
	p.audioLabel = p.newText("")
	p.speedLabel = p.newText("")
	p.volumeLabel = p.newText("")
	p.fileLabel = p.newText("ROM\nNo file selected")

	p.panel.AddChild(
		p.perfLabel,
		p.videoLabel,
		p.joypadLabel,
		p.serialLabel,
		p.audioLabel,
		p.fileLabel,
		p.newButton("Open ROM...", p.openROMDialog),
		p.speedLabel,
		p.newSlider(25, 800, int(g.speed*100), func(value int) {
			g.SetSpeed(float64(value) / 100)
		}),
		p.volumeLabel,
		p.newSlider(0, 100, int(g.volume*100), func(value int) {
			g.SetVolume(float64(value) / 100)
		}),
	)
	root.AddChild(spacer, p.panel)

	p.ui = &ebitenui.UI{
		Container:           root,
		DisableDefaultFocus: true,
	}
	p.refresh()
	return p
}

func debugPanelFace(size float64) *text.Face {
	face := text.Face(&text.GoTextFace{
		Source: mplusFaceSource,
		Size:   size,
	})
	return &face
}

func (p *debugPanel) Update() {
	p.receiveDialogResult()
	p.refresh()
	p.ui.Update()
}

func (p *debugPanel) Draw(screen *ebiten.Image) {
	p.ui.Draw(screen)
}

func (p *debugPanel) refresh() {
	g := p.game
	p.perfLabel.Label = fmt.Sprintf("FPS %.1f  TPS %.1f", ebiten.ActualFPS(), ebiten.ActualTPS())
	p.videoLabel.Label = fmt.Sprintf(
		"Video\nLY %03d  LYC %03d  Mode %s\nLCDC %02X  STAT %02X\nSCX %02X  SCY %02X  WX %02X  WY %02X",
		g.ppu.ReadRegister(register.LY),
		g.ppu.ReadRegister(register.LYC),
		ppuModeName(g.ppu.PPUMode()),
		g.ppu.ReadRegister(register.LCDC),
		g.ppu.ReadRegister(register.STAT),
		g.ppu.ReadRegister(register.SCX),
		g.ppu.ReadRegister(register.SCY),
		g.ppu.ReadRegister(register.WX),
		g.ppu.ReadRegister(register.WY),
	)
	p.joypadLabel.Label = fmt.Sprintf(
		"Joypad\nPressed %s\nP1 %02X",
		joypadKeysLabel(g.keys),
		g.j.Read(register.P1JOYP),
	)
	serialState := g.s.State()
	p.serialLabel.Label = fmt.Sprintf(
		"Serial\nSB %02X  SC %02X  Active %t  Internal %t\nBit %d",
		serialState.SB,
		serialState.SC,
		serialState.Transferring,
		serialState.InternalClock,
		serialState.BitIndex,
	)
	p.audioLabel.Label = fmt.Sprintf(
		"Audio\nAPU %t  Buffer %d\nNR50 %02X  NR51 %02X  NR52 %02X",
		g.apu.Enabled(),
		g.apu.BufferedSamples(),
		g.apu.Read(register.NR50),
		g.apu.Read(register.NR51),
		g.apu.Read(register.NR52),
	)
	if p.dialogMessage != "" {
		p.fileLabel.Label = fmt.Sprintf("ROM\n%s", p.dialogMessage)
	} else if p.game.romFilename == "" {
		p.fileLabel.Label = "ROM\nNo file selected"
	} else {
		p.fileLabel.Label = fmt.Sprintf("ROM\n%s", filepath.Base(p.game.romFilename))
	}
	p.speedLabel.Label = fmt.Sprintf("Speed %.2fx", g.speed)
	p.volumeLabel.Label = fmt.Sprintf("Volume %.0f%%", g.volume*100)
	p.panel.RequestRelayout()
}

func ppuModeName(mode uint8) string {
	switch mode {
	case 0:
		return "HBlank"
	case 1:
		return "VBlank"
	case 2:
		return "OAM"
	case 3:
		return "Transfer"
	default:
		return "Unknown"
	}
}

func joypadKeysLabel(keys []joypad.KeyCode) string {
	if len(keys) == 0 {
		return "None"
	}
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, joypadKeyName(key))
	}
	return strings.Join(names, " ")
}

func joypadKeyName(key joypad.KeyCode) string {
	switch key {
	case joypad.KeyA:
		return "A"
	case joypad.KeyB:
		return "B"
	case joypad.KeySelect:
		return "Select"
	case joypad.KeyStart:
		return "Start"
	case joypad.KeyRight:
		return "Right"
	case joypad.KeyLeft:
		return "Left"
	case joypad.KeyUp:
		return "Up"
	case joypad.KeyDown:
		return "Down"
	default:
		return "?"
	}
}

func (p *debugPanel) newText(label string) *widget.Text {
	return widget.NewText(
		widget.TextOpts.Text(label, p.face, color.NRGBA{R: 0xee, G: 0xf2, B: 0xf8, A: 0xff}),
		widget.TextOpts.MaxWidth(float64(debugPanelW-32)),
		widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionStart),
		widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
			Stretch: false,
		})),
	)
}

func (p *debugPanel) newButton(label string, onClick func()) *widget.Button {
	buttonImage := &widget.ButtonImage{
		Idle:    uiimage.NewNineSliceColor(color.NRGBA{R: 0x2b, G: 0x34, B: 0x42, A: 0xff}),
		Hover:   uiimage.NewNineSliceColor(color.NRGBA{R: 0x38, G: 0x45, B: 0x57, A: 0xff}),
		Pressed: uiimage.NewNineSliceColor(color.NRGBA{R: 0xff, G: 0xc8, B: 0x57, A: 0xff}),
	}
	return widget.NewButton(
		widget.ButtonOpts.Image(buttonImage),
		widget.ButtonOpts.Text(label, p.face, &widget.ButtonTextColor{
			Idle:     color.NRGBA{R: 0xee, G: 0xf2, B: 0xf8, A: 0xff},
			Disabled: color.NRGBA{R: 0x7d, G: 0x87, B: 0x94, A: 0xff},
		}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 12, Right: 12, Top: 6, Bottom: 6}),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(debugPanelW-32, 30),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: false}),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			onClick()
		}),
	)
}

func (p *debugPanel) openROMDialog() {
	if p.dialogOpen {
		return
	}
	p.dialogOpen = true
	p.dialogMessage = "Opening..."
	p.dialogResult = make(chan fileDialogResult, 1)
	result := p.dialogResult
	go func() {
		filename, err := chooseROMFile()
		result <- fileDialogResult{filename: filename, err: err}
	}()
}

func (p *debugPanel) receiveDialogResult() {
	if p.dialogResult == nil {
		return
	}
	select {
	case result := <-p.dialogResult:
		p.dialogOpen = false
		p.dialogResult = nil
		p.dialogMessage = ""
		if result.err != nil {
			if !errors.Is(result.err, errFileDialogCancelled) {
				p.dialogMessage = "Open failed"
				p.game.logger.Error("Failed to open file dialog", "error", result.err)
			}
			return
		}
		p.game.SetROMFilename(result.filename)
		p.game.logger.Info("ROM file selected", "path", result.filename)
	default:
	}
}

func (p *debugPanel) newSlider(minValue, maxValue, current int, onChange func(value int)) *widget.Slider {
	track := &widget.SliderTrackImage{
		Idle:  uiimage.NewNineSliceColor(color.NRGBA{R: 0x4a, G: 0x54, B: 0x63, A: 0xff}),
		Hover: uiimage.NewNineSliceColor(color.NRGBA{R: 0x61, G: 0x6d, B: 0x7d, A: 0xff}),
	}
	handle := &widget.ButtonImage{
		Idle:    uiimage.NewNineSliceColor(color.NRGBA{R: 0x69, G: 0xc3, B: 0xff, A: 0xff}),
		Hover:   uiimage.NewNineSliceColor(color.NRGBA{R: 0x93, G: 0xd6, B: 0xff, A: 0xff}),
		Pressed: uiimage.NewNineSliceColor(color.NRGBA{R: 0xff, G: 0xc8, B: 0x57, A: 0xff}),
	}

	return widget.NewSlider(
		widget.SliderOpts.Orientation(widget.DirectionHorizontal),
		widget.SliderOpts.MinMax(minValue, maxValue),
		widget.SliderOpts.InitialCurrent(current),
		widget.SliderOpts.Images(track, handle),
		widget.SliderOpts.FixedHandleSize(8),
		widget.SliderOpts.TrackOffset(8),
		widget.SliderOpts.PageSizeFunc(func() int { return 5 }),
		widget.SliderOpts.DisableDefaultKeys(true),
		widget.SliderOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(debugPanelW-32, 24),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: false}),
		),
		widget.SliderOpts.ChangedHandler(func(args *widget.SliderChangedEventArgs) {
			onChange(args.Current)
		}),
	)
}
