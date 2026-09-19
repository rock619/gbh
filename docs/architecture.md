# Architecture

This document describes how the Game Boy emulator is organized in this repository.
It focuses on the current implementation rather than the full Game Boy hardware
surface.

## Overview

`gbh.Game` is the top-level object that wires the emulator components together.
The Ebiten frontend calls `Game.Update` on its update ticks. Each update adds
CPU cycles to a floating-point budget based on elapsed wall-clock time and
playback speed, then repeatedly calls `Game.Step` while at least four cycles
remain. Drawing runs separately through `Game.Draw`.

The CPU is the timing driver. When an instruction consumes machine cycles, the
CPU advances the connected tickers:

- `graphic.PPU`
- `timer.Timer`
- `audio.APU`
- `serial.Serial`

Those devices can request interrupts through the shared interrupt controller.
The MMU sits between the CPU and the rest of the system and routes reads and
writes to cartridges, RAM, video, audio, input, serial, timer, and interrupt
registers.

```text
Ebiten Update
    -> gbh.Game.Update
        -> gbh.Game.Step
            -> cpu.CPU.Step
                -> instruction execution
                    -> cpu.CPU.TickM
                        -> graphic.PPU.Tick
                        -> timer.Timer.Tick
                        -> audio.APU.Tick
                        -> serial.Serial.Tick
```

## Main Components

### `game.go`

`Game` composes the emulator:

- loads a `cartridge.Cartridge` from ROM and optional RAM bytes
- creates the shared `interrupt.Controller`
- creates the `joypad.Joypad`, `serial.Serial`, `audio.APU`, `timer.Timer`,
  `graphic.PPU`, `memory.MMU`, and `cpu.CPU`
- owns the execution cycle budget and playback speed
- connects the APU mixer to Ebiten audio when running with a UI
- draws the PPU framebuffer to the Ebiten screen

There are two constructors:

- `NewGame` for the Ebiten UI/audio path
- `NewHeadlessGame` for tests or non-UI execution

`RunFrame` adds a fixed 70,224-cycle budget without drawing or using wall-clock
time, which is useful for test ROMs and headless checks. It executes whole CPU
steps and carries any budget overshoot into the next call.

`machine/Machine` is an unfinished hardware abstraction; the current frontend
and test ROM runner use `gbh.Game`.

### `cmd/gbh/main.go`

The command-line entry point reads the ROM path from the command line, loads an
optional save file, creates an Ebiten audio context, builds `gbh.Game`, and runs
it with `ebiten.RunGame`.

After the game loop returns, it saves supported battery-backed cartridge RAM.
See [Save data](save-data.md) for the save lifecycle and known banking issues.

### `cpu/`

The CPU package implements the Game Boy CPU execution loop, registers,
instruction table, interrupt handling, HALT/STOP behavior, and cycle accounting.

The CPU starts at the post-boot-ROM state:

- `PC = 0x0100`
- `SP = 0xFFFE`
- registers initialized to the documented DMG post-boot values

The boot ROM itself is not currently executed.

### `memory/`

`memory.MMU` implements the CPU-visible memory map. It delegates reads and
writes to the component that owns each address range:

- cartridge ROM and external RAM
- PPU VRAM, OAM, and LCD registers
- APU sound registers and wave RAM
- joypad register
- serial registers
- timer registers
- interrupt registers
- work RAM and high RAM

OAM DMA writes are handled in the MMU. The current implementation performs the
copy immediately.

### `cartridge/`

The cartridge package parses the cartridge header enough to choose a cartridge
implementation, expose the title, validate the Nintendo logo, and split ROM/RAM
into banks.

Currently implemented cartridge types:

- ROM only
- MBC1 variants
- MBC5 variants (rumble-specific bank masking remains incomplete)

Other MBC families such as MBC2, MBC3, MBC6, MBC7, and specialty
cartridges are recognized but not implemented.

### `graphic/`

The PPU owns:

- 8 KiB VRAM
- 160 bytes OAM
- LCD registers
- front/back framebuffers
- scanline sprite selection
- background, window, and object rendering

`PPU.Tick` advances LCD timing, updates `LY`/`STAT`, changes PPU modes, renders
visible scanlines, swaps framebuffers at VBlank, and requests VBlank/STAT
interrupts.

### `audio/`

The APU owns the four Game Boy audio channels:

- channel 1: square wave with sweep
- channel 2: square wave
- channel 3: wave channel
- channel 4: noise channel

It also owns the frame sequencer and mixer. The mixer samples at 44.1 kHz,
routes channels according to `NR51`, applies output volume from `NR50`, and
serves PCM bytes to Ebiten through an `io.Reader`-style mixer.

The timer drives the APU frame sequencer through DIV edge behavior.

### `timer/`

The timer package implements:

- `DIV`
- `TIMA`
- `TMA`
- `TAC`
- timer overflow interrupt requests
- DIV edge behavior used by the APU frame sequencer

`Timer.Tick` advances the internal counter using CPU cycles supplied by the CPU.

### `interrupt/`

The interrupt controller owns:

- `IE` at `0xFFFF`
- `IF` at `0xFF0F`
- `IME`
- delayed IME enable after `EI`

It exposes interrupt request helpers for PPU, timer, serial, and joypad-like
devices. `CPU.Step` asks the controller for the highest-priority enabled and
requested interrupt before executing the next instruction.

### `joypad/`

The joypad package implements the `P1/JOYP` register and keeps separate state
for button keys and d-pad keys. `Game.Update` maps Ebiten keyboard state into
Game Boy key codes and calls `Joypad.SetPressed`.
`SetPressed` requests a joypad interrupt when a selected input bit falls from
1 to 0. Changing the selection bits through `Joypad.Write` does not currently
perform that edge check.

### `serial/`

The serial package implements `SB` and `SC`. Internal-clock transfers advance
bit by bit through `Serial.Tick`; external-clock transfers use
`ClockExternalBit`. After eight bits, it updates `SB` with the received byte,
clears the transfer-start bit, writes the transmitted byte to the configured
output writer, and requests a serial interrupt. A `LinkCable` interface allows
bit exchange with a connected peer.

This is mainly useful for test ROM output.

## Memory Map

The MMU routes the CPU address space as follows:

| Range | Owner | Notes |
| --- | --- | --- |
| `0x0000-0x3FFF` | cartridge | fixed ROM bank |
| `0x4000-0x7FFF` | cartridge | switchable ROM bank |
| `0x8000-0x9FFF` | PPU | VRAM |
| `0xA000-0xBFFF` | cartridge | external RAM |
| `0xC000-0xDFFF` | MMU RAM | work RAM |
| `0xE000-0xFDFF` | MMU RAM | echo RAM |
| `0xFE00-0xFE9F` | PPU | OAM |
| `0xFEA0-0xFEFF` | MMU | unusable range |
| `0xFF00-0xFF7F` | devices | I/O registers |
| `0xFF80-0xFFFE` | MMU RAM | high RAM |
| `0xFFFF` | interrupt controller | interrupt enable register |

Important I/O register ownership:

| Register area | Owner |
| --- | --- |
| `P1/JOYP` | joypad |
| `SB`, `SC` | serial |
| `DIV`, `TIMA`, `TMA`, `TAC` | timer |
| `IF`, `IE` | interrupt controller |
| `NR10-NR52`, `0xFF30-0xFF3F` | APU |
| `LCDC`, `STAT`, scrolling, palette, window, DMA | PPU |

## Timing Model

The emulator uses CPU cycles as the shared time unit.

- CPU clock: 4,194,304 Hz
- One frame: 70,224 cycles
- One scanline: 456 cycles
- Visible lines: 144

`Game.Update` adds `elapsedSeconds * 4,194,304 * speed` to its cycle budget.
The first update only initializes the clock. To limit catch-up after a host
stall, the budget is capped at `70,224 * max(speed, 1)` cycles. `CPU.Step`
returns the consumed cycles, which are subtracted while at least four cycles
remain. `RunFrame` instead adds a fixed 70,224-cycle budget for tests.

Hardware frame timing stays fixed regardless of host update rate or playback
speed. `Game.SetSpeed` updates both execution pacing and APU output sampling;
see [Audio pacing](audio-pacing.md).

Within a CPU instruction, `CPU.TickM` advances time in 4-cycle machine-cycle
increments and calls each connected ticker. This keeps PPU, timer, APU, and serial
progress tied to instruction execution.

## OAM Corruption Bug

The DMG OAM corruption bug is modeled as a collaboration between the CPU, MMU,
and PPU instead of as a normal memory read/write side effect.

The bug is caused by the CPU's 16-bit increment/decrement unit touching a value
in `0xFE00-0xFEFF` while the PPU is scanning OAM during the first part of a
visible scanline. The important detail for this emulator is that the triggering
operation is not always the visible memory access. Some affected instructions
trigger during an internal machine cycle or during the register update that
follows a read.

The implementation keeps those two responsibilities separate:

- CPU code calls `MaybeCorruptOAM(addr, kind)` at the exact machine-cycle point
  where the affected 16-bit update happens.
- The MMU forwards that request to the PPU through `CorruptOAM`.
- The PPU checks whether the LCD is enabled, `LY` is visible, `addr` is in the
  OAM area, and the current scanline cycle is inside the OAM bug window.
- The PPU then chooses the OAM row from the current scanline timing and applies
  the corruption pattern for the supplied `OAMBugKind`.

This shape is intentional. The CPU knows which instruction micro-operation is
currently executing, while the PPU knows whether that moment overlaps OAM scan.
Keeping the corruption request as an explicit CPU-to-PPU signal avoids treating
every `0xFE00-0xFEFF` access as suspicious and makes the off-by-one M-cycle
cases visible in the instruction implementation.

### Timing Window

The PPU treats the corruption window as scanline cycles `4..76`, inclusive.
This corresponds to OAM rows `1..19`:

```text
row = (ppu.cycles % 456) / 4
```

Row 0 is ignored because it corresponds to the first two objects and does not
produce the row-copy corruption that the test ROMs check. The PPU also ignores
requests while the LCD is disabled or during VBlank. The address check uses the
full `0xFE00-0xFEFF` range, not just writable OAM `0xFE00-0xFE9F`, because the
bug is about what appears on the address bus during the 16-bit operation.

This row calculation is why the CPU must call `MaybeCorruptOAM` before advancing
the relevant M-cycle. Calling it one `TickM` too late selects the next row and
breaks pattern-sensitive tests such as `oam_bug/rom_singles/8-instr_effect.gb`.

### CPU Trigger Points

The CPU does not try to infer OAM corruption from all memory accesses. Instead,
only the instructions that use the affected 16-bit update path call
`MaybeCorruptOAM`.

`INC r16` and `DEC r16` trigger before the register is changed, during the
instruction's internal M-cycle:

```text
M1: opcode fetch
M2: 16-bit inc/dec; possible OAM corruption
```

`POP r16` triggers on each stack read before that read's M-cycle is ticked.
`Read8` cannot be used here because it advances time before the corruption
request would be seen by the PPU:

```text
M1: opcode fetch
M2: read [SP]; possible read corruption; SP++
M3: read [SP]; possible read corruption; SP++
```

`PUSH r16` triggers three effective write corruptions. There is one internal
stack setup cycle and two stack writes; the stack writes overlap with the SP
decrements, so the implementation places corruption requests around those
machine-cycle boundaries:

```text
M1: opcode fetch
M2: internal stack setup; possible write corruption
M3: --SP and write high byte; possible write corruption
M4: --SP and write low byte; possible write corruption
```

`LD A,(HL+)` and `LD A,(HL-)` are split manually into `mem.Read`,
`MaybeCorruptOAM`, `TickM`, and then the HL update. This keeps the OAM read and
the HL increment/decrement in the same M-cycle. Using the ordinary `Read8`
helper would tick first and make the corruption one row late.

`LD (HL+),A` and `LD (HL-),A` use a separate kind from the read forms. The read
and write forms do not produce the same OAM pattern.

When changing these paths, avoid replacing the split operations with the generic
`Read8` or `Write8` helpers unless the corruption request is kept on the same
machine-cycle boundary. Those helpers tick immediately after the bus operation,
which is correct for normal instruction timing but can move the OAM bug request
to the next row.

### Corruption Patterns

OAM is treated as 8-byte rows, where each row contains two objects. The PPU
uses little-endian 16-bit words inside each row when applying the bitwise glitch
formulas.

Write corruption is used by `INC/DEC r16`, `PUSH`, and the write forms of
`LD (HL+/-),A`:

```text
first word = ((a ^ c) & (b ^ c)) ^ c
last three words = previous row's last three words
```

where:

- `a` is the current row's first word
- `b` is the previous row's first word
- `c` is the previous row's third word

Read corruption is used by `POP`. Its default form updates the previous row's
first word and then copies the previous row into the current row:

```text
previous first word = b | (a & c)
current row = previous row
```

Some read rows use a secondary pattern based on nearby rows. This is needed for
the row groups covered by `8-instr_effect.gb`:

```text
previous first word = (b & (a | c | d)) | (a & c & d)
row - 2 = row - 1
current row = row - 1
```

The `LD A,(HL+)` and `LD A,(HL-)` forms combine the read-with-increment pattern
with the normal read corruption. They are the most timing-sensitive case in the
current implementation because the OAM read and HL update must be modeled in
the same M-cycle.

The formulas above are deliberately implemented in the PPU rather than in the
CPU. They depend on the row selected by `ppu.cycles % 456`, and keeping the row
math next to the OAM storage makes it easier to compare the resulting bytes
against the blargg output tables.

### Test Notes

`blargg/oam_bug/rom_singles/7-timing_effect.gb` can print enough detail to
overflow its `0xA004` text buffer when the pattern is wrong. In that case the
screen can appear white or the status at `0xA000` can temporarily read as
`0xFF`, because the test output has run past cartridge RAM and disturbed the
MBC state. That symptom is usually an output overflow, not the root cause.

For debugging individual OAM cases, use the gated debug tests in
`debug_internal_test.go` by setting `GBH_DEBUG_OAM=1`. The normal test run skips
them so they do not affect regular regression tests.

## Current Scope and Limitations

The current implementation is primarily a DMG-oriented emulator core with an
Ebiten frontend.

Known scope boundaries:

- the boot ROM is not executed; CPU registers start from post-boot values
- ROM-only, MBC1, and MBC5 cartridges are implemented, with known banking
  issues documented in [Save data](save-data.md)
- MBC2, MBC3, MBC6, MBC7, and specialty cartridges are not implemented
- OAM DMA currently copies immediately rather than modeling CPU bus timing
- joypad input changes request interrupts, but selection-register writes do
  not check for interrupt-triggering edges
- some CGB-related registers appear in the PPU, but CGB behavior is not modeled
  as a complete feature
