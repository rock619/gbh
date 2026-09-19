# Repository guide

## Overview

gbh is a DMG-oriented Game Boy emulator written in Go with an Ebiten frontend.
ROM-only, MBC1, and MBC5 cartridges are implemented; full CGB support and the
boot ROM are not. Battery-backed cartridge RAM saves (`.sav`) are separate
from emulator save states.

## Code map

- `game.go`: component wiring, real-time pacing, input, rendering, and audio.
- `cmd/gbh/`: CLI startup and shutdown saving; `savefile/`: save-file I/O.
- `cpu/`: instructions and machine-cycle timing; `memory/`: MMU and DMA.
- `graphic/`: PPU; `audio/`: APU and PCM mixer; `timer/`: timers and DIV edges.
- `cartridge/`: ROM/RAM banking; `interrupt/`, `joypad/`, `serial/`: devices.
- `machine/` is an unfinished abstraction; current execution uses `gbh.Game`.

## Commands

Use the Go toolchain specified in `go.mod` (currently Go 1.27.1).

- Build: `go build ./cmd/gbh`
- Run: `go run ./cmd/gbh path/to/game.gb [path/to/game.sav]`
- Quick tests: `go test -short ./...` (skips test ROM regressions).
- Full tests: `go test ./...`
- Single ROM regression: `go test -run '^TestROM/instr_timing$' .`

Makefile shortcuts: `make test` runs quick tests, `make test-all` runs all
tests, and `make test-rom [ROM=instr_timing]` runs ROM regressions. The latter
two disable test-result caching and set a 10-minute per-package timeout.
Run `make` for help. These targets use the host environment; they do not
set up Docker or platform dependencies.

Without `-short`, root-package tests automatically download test ROMs to
`testdata/game-boy-test-roms` if absent. Failed image comparisons write PNGs
to `testdata/actual`. Headless games skip UI/audio playback but still import
Ebiten, so platform graphics dependencies may be needed for tests.

## Read before changing

- Component responsibilities, CPU/PPU timing, or OAM corruption:
  [docs/architecture.md](docs/architecture.md).
- Audio, playback speed, or real-time pacing:
  [docs/audio-pacing.md](docs/audio-pacing.md).
- Cartridge banking or save lifecycle:
  [docs/save-data.md](docs/save-data.md), including known compatibility issues.

Keep hardware timing fixed: 4,194,304 CPU cycles/second, 70,224 cycles/frame,
and 4 CPU cycles per machine cycle. Playback speed changes must update both
the execution budget and APU sampling. Preserve OAM corruption trigger points
relative to `TickM`; generic bus helpers can shift them by a machine cycle.

For code changes, run `gofmt` on changed Go files and the relevant tests; run
affected ROM regressions for timing or rendering changes. Report checks that
could not run. Update related documentation when behavior or scope changes,
and keep implemented behavior distinct from proposed follow-up work.
