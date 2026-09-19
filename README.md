# GBH

DMG-oriented Game Boy emulator written in Go with an Ebitengine frontend.

## Run

Use Go 1.27.1 or later:

```sh
go run ./cmd/gbh path/to/game.gb [path/to/game.sav]
```

The save path defaults to the ROM path with its extension replaced by `.sav`.
Controls: arrow keys for the d-pad, X for A, Z for B, Enter for Start, and
Backspace for Select.

## Tests

Run `make` to list the available test commands:

```sh
make test                       # Quick tests; skips test ROMs
make test-all                   # All tests, including test ROMs
make test-rom                   # Only test ROM regressions
make test-rom ROM=instr_timing   # One test ROM regression
```

ROM tests automatically download the pinned v7.0 test ROM archive to
`testdata/game-boy-test-roms` when absent. Failed image comparisons save
screenshots to `testdata/actual`. Both `test-all` and `test-rom` disable Go's
test-result cache and allow up to 10 minutes per package. `ROM` is a Go test
regular expression matched against the complete ROM test name.

These commands run on the local host; they do not provision Docker or Linux
graphics dependencies. On a Linux CI runner with the required libraries and
Xvfb installed, use `xvfb-run -a make test-all`.

## Documentation

- [Repository guide](AGENTS.md): code map, development commands, and change guidelines shared by coding agents.
- [Architecture](docs/architecture.md): emulator component layout, execution flow, memory map, and current limitations.
- [Audio pacing](docs/audio-pacing.md): real-time execution, audio buffering, and playback speed.
- [Save data](docs/save-data.md): current battery-backed RAM support, remaining work, and related compatibility issues.
