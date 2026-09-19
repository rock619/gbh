# Audio Pacing

This note documents the audio timing changes made while fixing delayed and
choppy sound effects in real games.

## Hardware Timing

The Game Boy CPU clock and PPU frame length are fixed hardware properties:

```text
CPU clock: 4,194,304 Hz
PPU frame: 70,224 cycles
Refresh:   4,194,304 / 70,224 = 59.7275 Hz
```

The emulator must not change `70,224 cycles/frame` to match the host display
rate. PPU timing, `LY`, `STAT`, VBlank, OAM search, and window behavior depend
on that fixed frame length.

What can change is how many CPU cycles are executed per wall-clock second:

```text
cyclesToRun = elapsedSeconds * 4,194,304 * speed
```

This is why `Game.Update` uses elapsed real time and a floating-point cycle
budget instead of adding one Game Boy frame worth of cycles per Ebiten update.
Running exactly `70,224 cycles` on every 60 Hz host update would execute:

```text
70,224 * 60 = 4,213,440 cycles/sec
```

which is about `0.456%` faster than the Game Boy clock. That small drift creates
more audio samples than the 44.1 kHz device consumes, so the queued audio grows
over time and sound effects become delayed.

## Audio Queue

The APU mixer still exposes an `io.Reader` to Ebiten audio. The emulator thread
generates samples into a ring buffer, and the audio callback reads samples out
of it.

The ring buffer is bounded and synchronized because it is touched from the
emulation thread and the audio thread:

- `Push` overwrites the oldest sample when the buffer is full.
- `Pop` removes the oldest queued sample.
- `DiscardOldest` is used for latency control.
- A mutex protects the buffer state.

When the audio callback asks for bytes, the mixer trims excessive latency before
serving the read. The trim uses hysteresis:

```text
high threshold = targetBufferedSamples * 2 + requestedSamples
keep amount    = targetBufferedSamples + requestedSamples
```

Small queue drift is left alone because frequent sample drops are audible.
Only when the queue grows past the high threshold are old samples discarded back
to the keep amount.

If the callback under-runs, the mixer repeats the last emitted sample instead of
returning hard silence. This does not create correct missing audio, but it avoids
turning a short under-run into an obvious click or gap.

## Speed Changes

The emulator speed and audio output sampling must be changed together.

`Game.SetSpeed` updates both:

```text
Game speed       -> how many CPU cycles are executed per wall-clock second
APU mixer speed  -> how many emulated cycles pass per output audio sample
```

The mixer computes its output sample spacing as:

```text
cyclesPerOutputSample = 4,194,304 * speed / 44,100
```

At `speed = 1.0`, the mixer samples the APU at the normal 44.1 kHz rate relative
to Game Boy time. At `speed = 2.0`, the CPU/APU state advances twice as far per
wall-clock second, and the mixer also waits twice as many emulated cycles before
emitting each host audio sample. The host device still receives 44,100 samples
per second, but those samples represent twice as much Game Boy time, so both
tempo and pitch become 2x.

Without this mixer-side speed adjustment, fast-forwarding creates too many
normal-speed audio samples. The latency control then has to discard the excess,
which sounds like dropped or unstable audio rather than true fast-forward audio.

## Current Tradeoffs

This is still a buffered audio design:

```text
Ebiten Update thread -> emulates CPU/PPU/APU -> pushes audio samples
Audio callback       -> reads queued PCM samples
```

That keeps the audio callback lightweight. It also means the emulator must keep
the queue near a target latency and handle occasional host scheduling jitter.

A larger future change would make audio consumption the main pacing source, or
generate missing samples on demand from the audio callback. That would require
careful ownership of CPU/MMU/APU state so that the audio thread and update
thread do not step the emulator concurrently.
