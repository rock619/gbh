package gbh

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDebugOAM7(t *testing.T) {
	skipUnlessOAMDebug(t)

	rom, err := os.ReadFile("testdata/game-boy-test-roms/blargg/oam_bug/rom_singles/7-timing_effect.gb")
	if err != nil {
		t.Fatal(err)
	}

	game := NewHeadlessGame(rom, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	last := ""
	for frame := range 60 * 60 {
		if err := game.RunFrame(); err != nil {
			t.Fatal(err)
		}

		if frame%60 == 0 {
			status := game.mmu.Read(0xA000)
			text := readTestText(game, 0xA004, 4096)

			if text != last {
				t.Logf("frame=%d status=%02X pc=%04X text:\n%s", frame, status, game.cpu.PC(), text)
				last = text
			}

			if strings.Contains(text, "Passed") || strings.Contains(text, "Failed") {
				break
			}
		}
	}
}

func TestDebugOAM7TextPointer(t *testing.T) {
	skipUnlessOAMDebug(t)

	rom, err := os.ReadFile("testdata/game-boy-test-roms/blargg/oam_bug/rom_singles/7-timing_effect.gb")
	if err != nil {
		t.Fatal(err)
	}

	game := NewHeadlessGame(rom, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	seenSignature := false
	for frame := range 60 * 180 {
		if err := game.RunFrame(); err != nil {
			t.Fatal(err)
		}

		status := game.mmu.Read(0xA000)
		sigOK := blarggSignatureOK(game)

		if sigOK {
			seenSignature = true
		}

		textLen := 0
		textPtr := uint16(0)
		var pointerAddrs []uint16
		if sigOK {
			textLen = len(readTestText(game, 0xA004, 0x4000))
			textPtr = 0xA004 + uint16(textLen)
			pointerAddrs = findBlarggTextPointerAddrs(game, textPtr)
		}

		if frame%60 == 0 || sigOK && textPtr >= 0xBE00 {
			t.Logf("frame=%d status=%02X sigOK=%t pc=%04X textPtr=%04X textLen=%d",
				frame, status, sigOK, game.cpu.PC(), textPtr, textLen)
			if len(pointerAddrs) != 0 {
				t.Logf("candidate text pointer variables: %04X", pointerAddrs)
			}
		}

		if seenSignature && !sigOK {
			t.Fatalf("blargg signature disappeared: frame=%d status=%02X pc=%04X textPtr=%04X",
				frame, status, game.cpu.PC(), textPtr)
		}

		if textPtr > 0xBFFF {
			t.Fatalf("text output overflowed cartridge RAM: frame=%d status=%02X pc=%04X textPtr=%04X",
				frame, status, game.cpu.PC(), textPtr)
		}

		if sigOK && status != 0x80 {
			if status == 0x00 {
				t.Logf("ROM passed: frame=%d pc=%04X textPtr=%04X", frame, game.cpu.PC(), textPtr)
				return
			}
			t.Fatalf("ROM failed: frame=%d status=%02X pc=%04X textPtr=%04X text:\n%s",
				frame, status, game.cpu.PC(), textPtr, readTestText(game, 0xA004, 0x4000))
		}
	}

	t.Fatalf("timeout: ROM still running pc=%04X status=%02X sigOK=%t textPtr=%04X",
		game.cpu.PC(), game.mmu.Read(0xA000), blarggSignatureOK(game), 0)
}

func TestDebugOAM8(t *testing.T) {
	skipUnlessOAMDebug(t)

	rom, err := os.ReadFile("testdata/game-boy-test-roms/blargg/oam_bug/rom_singles/8-instr_effect.gb")
	if err != nil {
		t.Fatal(err)
	}

	game := NewHeadlessGame(rom, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for frame := range 60 * 20 {
		if err := game.RunFrame(); err != nil {
			t.Fatal(err)
		}
		if !blarggSignatureOK(game) {
			continue
		}
		status := game.mmu.Read(0xA000)
		if status == 0x80 {
			continue
		}
		text := readTestText(game, 0xA004, 0x1FFC)
		if status == 0x00 {
			t.Logf("ROM passed at frame=%d:\n%s", frame, text)
			return
		}
		t.Fatalf("ROM failed at frame=%d status=%02X pc=%04X:\n%s", frame, status, game.cpu.PC(), text)
	}

	t.Fatalf("timeout: status=%02X pc=%04X text:\n%s",
		game.mmu.Read(0xA000), game.cpu.PC(), readTestText(game, 0xA004, 0x1FFC))
}

func skipUnlessOAMDebug(t *testing.T) {
	t.Helper()
	if os.Getenv("GBH_DEBUG_OAM") == "" {
		t.Skip("set GBH_DEBUG_OAM=1 to run OAM debug tests")
	}
}

func blarggSignatureOK(g *Game) bool {
	return g.mmu.Read(0xA001) == 0xDE &&
		g.mmu.Read(0xA002) == 0xB0 &&
		g.mmu.Read(0xA003) == 0x61
}

func findBlarggTextPointerAddrs(g *Game, ptr uint16) []uint16 {
	var addrs []uint16
	for addr := uint16(0xC000); addr < 0xDFFF; addr++ {
		if uint16(g.mmu.Read(addr))|uint16(g.mmu.Read(addr+1))<<8 == ptr {
			addrs = append(addrs, addr)
		}
	}
	for addr := uint16(0xFF80); addr < 0xFFFE; addr++ {
		if uint16(g.mmu.Read(addr))|uint16(g.mmu.Read(addr+1))<<8 == ptr {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func readTestText(g *Game, addr uint16, max int) string {
	buf := make([]byte, 0, max)
	for i := range max {
		v := g.mmu.Read(addr + uint16(i))
		if v == 0 {
			break
		}
		buf = append(buf, v)
	}
	return string(buf)
}
