package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/rock619/gbh"
	"github.com/rock619/gbh/savefile"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	if len(os.Args) < 2 {
		return errors.New("Usage: gbh <ROM file> [save file]")
	}
	romPath := os.Args[1]
	savePath := resolveSavePath(romPath, os.Args)
	rom, err := os.ReadFile(romPath)
	if err != nil {
		return fmt.Errorf("failed to open ROM file: %w", err)
	}
	logger.Info("ROM loaded successfully", "size", len(rom))

	ram, err := savefile.Read(savePath)
	if err != nil {
		return err
	}

	audioCtx := audio.NewContext(44100)
	game := gbh.NewGame(rom, ram, audioCtx, logger)
	if err := validateSaveRAM(ram, game.HasBattery(), game.RAMSize()); err != nil {
		return fmt.Errorf("validate save file %q: %w", savePath, err)
	}
	game.SetROMFilename(romPath)

	ebiten.SetWindowSize(1260, 648)
	logger.Info("Starting game", "title", game.Title())
	ebiten.SetWindowTitle(fmt.Sprintf("%s - GBH", game.Title()))
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetRunnableOnUnfocused(true)
	runErr := ebiten.RunGame(game)
	saveErr := saveBatteryRAM(savePath, game)
	return errors.Join(runErr, saveErr)
}

func resolveSavePath(romPath string, args []string) string {
	if len(args) >= 3 {
		return args[2]
	}

	return savefile.DefaultPath(romPath)
}

func validateSaveRAM(ram []byte, hasBattery bool, ramSize int) error {
	if !hasBattery {
		return nil
	}
	if ramSize <= 0 {
		return fmt.Errorf("invalid cartridge RAM size: %d", ramSize)
	}
	if ram == nil {
		return nil
	}
	if len(ram) != ramSize {
		return fmt.Errorf("invalid save size: got %d bytes, want %d", len(ram), ramSize)
	}
	return nil
}

type batteryRAM interface {
	HasBattery() bool
	RAMSize() int
	SaveRAM() []byte
}

func saveBatteryRAM(path string, ram batteryRAM) error {
	if !ram.HasBattery() || ram.RAMSize() <= 0 {
		return nil
	}
	return savefile.Write(path, ram.SaveRAM())
}
