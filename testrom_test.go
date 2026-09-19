package gbh_test

import (
	"archive/zip"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rock619/gbh"
)

const (
	testDataURL = "https://github.com/c-sp/game-boy-test-roms/releases/download/v7.0/game-boy-test-roms-v7.0.zip"
	testDataDir = "testdata/game-boy-test-roms"
)

var testDataErr error

type dmgTest struct {
	romPath    string
	goldenPath string
	seconds    float64
}

var dmgTests = map[string]dmgTest{
	"cpu_instrs": {
		romPath:    "blargg/cpu_instrs/cpu_instrs.gb",
		goldenPath: "blargg/cpu_instrs/cpu_instrs-dmg-cgb.png",
		seconds:    55,
	},
	"dmg_sound": {
		romPath:    "blargg/dmg_sound/dmg_sound.gb",
		goldenPath: "blargg/dmg_sound/dmg_sound-dmg.png",
		seconds:    60,
	},
	"instr_timing": {
		romPath:    "blargg/instr_timing/instr_timing.gb",
		goldenPath: "blargg/instr_timing/instr_timing-dmg-cgb.png",
		seconds:    1,
	},
	"interrupt_time": {
		romPath:    "blargg/interrupt_time/interrupt_time.gb",
		goldenPath: "blargg/interrupt_time/interrupt_time-dmg.png",
		seconds:    2,
	},
	"mem_timing": {
		romPath:    "blargg/mem_timing/mem_timing.gb",
		goldenPath: "blargg/mem_timing/mem_timing-dmg-cgb.png",
		seconds:    3,
	},
	"mem_timing-2": {
		romPath:    "blargg/mem_timing-2/mem_timing.gb",
		goldenPath: "blargg/mem_timing-2/mem_timing-dmg-cgb.png",
		seconds:    4,
	},
	"oam_bug": {
		romPath:    "blargg/oam_bug/oam_bug.gb",
		goldenPath: "blargg/oam_bug/oam_bug-dmg.png",
		seconds:    21,
	},
	"halt_bug": {
		romPath:    "blargg/halt_bug.gb",
		goldenPath: "blargg/halt_bug-dmg-cgb.png",
		seconds:    2,
	},
	"dmg-acid2": {
		romPath:    "dmg-acid2/dmg-acid2.gb",
		goldenPath: "dmg-acid2/dmg-acid2-dmg.png",
		seconds:    5,
	},
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		testDataErr = setupTestData()
	}
	os.Exit(m.Run())
}

func TestROM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test ROM regression tests in short mode")
	}
	if testDataErr != nil {
		t.Fatalf("test ROM data is unavailable: %v", testDataErr)
	}

	for name, tc := range dmgTests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rom, err := os.ReadFile(filepath.Join(testDataDir, tc.romPath))
			if err != nil {
				t.Fatalf("test ROM is unavailable: %v", err)
			}

			golden, err := loadPNG(filepath.Join(testDataDir, tc.goldenPath))
			if err != nil {
				t.Fatalf("golden image is unavailable: %v", err)
			}

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			game := gbh.NewHeadlessGame(rom, nil, logger)
			for range framesFor(tc.seconds) {
				if err := game.RunFrame(); err != nil {
					t.Fatalf("run frame: %v", err)
				}
			}

			actual := game.Framebuffer()
			if !imagesEqual(actual, golden) {
				path := saveActual(t, name, actual)
				t.Fatalf("framebuffer differs from golden image; actual saved to %s", path)
			}
		})
	}
}

func framesFor(seconds float64) int {
	const (
		gbClockHz      = 4194304.0
		cyclesPerFrame = 70224.0
	)
	return int(math.Ceil(seconds * gbClockHz / cyclesPerFrame))
}

func setupTestData() error {
	if fileExists(filepath.Join(testDataDir, "blargg/cpu_instrs/cpu_instrs.gb")) {
		return nil
	}

	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(testDataURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", testDataURL, resp.Status)
	}

	tmpZip := filepath.Join(testDataDir, "game-boy-test-roms-v7.0.zip.tmp")
	out, err := os.Create(tmpZip)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpZip)

	zr, err := zip.OpenReader(tmpZip)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if err := extractZipFile(f, testDataDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, dest string) error {
	path := filepath.Join(dest, f.Name)
	cleanDest := filepath.Clean(dest)
	cleanPath := filepath.Clean(path)
	if cleanPath != cleanDest && !strings.HasPrefix(cleanPath, cleanDest+string(os.PathSeparator)) {
		return fmt.Errorf("zip entry escapes destination: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(cleanPath, f.Mode())
	}

	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func imagesEqual(a, b image.Image) bool {
	if !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			ar, ag, ab = normalizeNearWhite(ar, ag, ab)
			br, bg, bb = normalizeNearWhite(br, bg, bb)
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

func normalizeNearWhite(r, g, b uint32) (uint32, uint32, uint32) {
	// Some golden PNGs contain near-white background pixels such as #FDFDFD
	// on otherwise white scanlines. Treat only these tiny background variations
	// as white while preserving real black/text differences.
	const nearWhite = 0xFDFD
	if r >= nearWhite && g >= nearWhite && b >= nearWhite {
		return 0xFFFF, 0xFFFF, 0xFFFF
	}
	return r, g, b
}

func saveActual(t *testing.T, name string, img image.Image) string {
	t.Helper()

	dir := filepath.Join("testdata", "actual")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create actual image dir: %v", err)
	}

	path := filepath.Join(dir, name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create actual image: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode actual image: %v", err)
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
