# Save Data and Follow-up Work

This document tracks cartridge save-data support and related work. Save data
means battery-backed cartridge RAM (`.sav`); emulator save states are a separate
feature.

## Current Support

The minimum shutdown-save path is implemented:

- Cartridge metadata exposes the cartridge type, RAM size, and whether its
  battery-backed storage is supported.
- Cartridge RAM can be exported as a copy in bank order.
- `Game` and `Machine` expose cartridge RAM to their callers.
- The default path replaces the ROM extension with `.sav`; an explicit CLI
  path takes precedence.
- A missing save file starts with empty cartridge RAM, while other read errors
  stop startup.
- Existing save data must exactly match the cartridge RAM size.
- Save files are written to a temporary file, synced, closed, and renamed over
  the destination.
- Battery-backed RAM is saved after `ebiten.RunGame` returns, including when the
  game loop returns an error.

The currently supported battery-backed cartridge types are MBC1+RAM+BATTERY,
MBC5+RAM+BATTERY, and MBC5+RUMBLE+RAM+BATTERY.

## Remaining Save-data Work

### Autosave and RAM Change Tracking

Shutdown saving cannot protect changes from a crash, forced termination, or
power loss. Add change tracking and periodic saving as the next save-data
feature.

Suggested design:

1. Add a monotonically increasing RAM generation to the common cartridge
   implementation.
2. Route external-RAM mutations through a common helper. Increment the
   generation only when a byte actually changes.
3. Expose the generation through `Cartridge` and `Game`.
4. Take the RAM snapshot on the emulation goroutine, then send that immutable
   copy to a background save worker.
5. Debounce writes for a short interval. If the generation changes while a
   write is in progress, schedule another write after it completes.
6. Keep the existing synchronous shutdown save as the final flush.
7. Do not mark a generation as saved when writing fails; report the error and
   allow a later retry.

Completion criteria:

- Repeated writes of the same value do not trigger additional saves.
- Changed battery RAM is saved without closing the emulator.
- A write that races with an in-progress save is included in a later save.
- Non-battery cartridges never create save files.
- Shutdown waits for the latest RAM snapshot to be written.

## Independent Cartridge and Frontend Issues

These issues affect compatibility or future save behavior but are separate
from the save-file lifecycle above.

### MBC1 Bank Selection

The current MBC1 implementation decides whether the secondary bank register
controls ROM or RAM from ROM size alone. It also uses the selected RAM bank
without applying the banking-mode restriction.

Keep the lower ROM bank register, secondary two-bit register, and banking mode
as separate state. Derive the effective fixed ROM bank, switchable ROM bank,
and RAM bank at read/write time. Add tests for mode 0, mode 1, 32 KiB RAM, and
large-ROM wiring before relying on saves from affected games.

### MBC5 Rumble RAM Bank Bits

The current MBC5 implementation always treats all four low bits written at
`0x4000-0x5FFF` as the RAM bank. For rumble cartridge types, bit 3 controls the
rumble motor and only bits 0-2 select RAM. Make the mask depend on cartridge
type and track rumble state separately.

### ROM Switching from the Debug Panel

The current **Open ROM...** action selects a path and changes the displayed
filename, but it does not load or install a new cartridge.

A real switch must:

1. Flush the current cartridge's battery RAM.
2. Read and validate the selected ROM and its save file.
3. Construct the replacement emulator components.
4. Replace the active game only after every preceding step succeeds.
5. Keep the current game running if selection, loading, validation, or saving
   fails.

The save path and save worker must switch together with the cartridge so data
cannot be written under the wrong ROM name.

### MBC3 Real-time Clock Persistence

MBC3 is not implemented yet. When it is added, timer cartridges need more than
the raw RAM bytes stored in `.sav`: RTC registers and enough host-time metadata
to advance the clock while the emulator is not running must also be persisted.

Choose and document a format before implementation. A companion `.rtc` file
keeps `.sav` compatible with raw SRAM tools; a versioned container is more
self-contained but is not a raw-save format.
