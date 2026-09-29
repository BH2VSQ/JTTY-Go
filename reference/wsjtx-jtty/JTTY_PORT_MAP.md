# JTTY-Go port map to WSJT-X 3.2.0-rc1

The Go implementation is being developed against the supplied WSJT-X source
rather than inferred solely from the user guide.

## Receive chain

`rjtty_sub` / `rjtty_core`
→ 59-symbol frame timing and 1.25-frame input chunk
→ `jtty_mdecode_step`
→ normal decode of the current quarter-frame search window
→ up to three retro sweeps after successful subtraction
→ persistent active-message history and update delivery.

Inside `jtty_mdecode`:

1. `ana64a` converts the 12 kHz real input to a 6 kHz complex analytic signal.
2. An 8192-point sync search scans the quarter-frame time range at 12-sample
   (2 ms) increments.
3. Sync power is smoothed with the five-bin `1,2,3,2,1` kernel.
4. Candidate peaks are locally masked before the next candidate is selected.
5. Primary-channel candidates use `jtty_peakup` for timing/frequency refinement.
6. Sync tone hard decisions and SNR gates reject weak/non-JTTY candidates.
7. Payload symbols are correlated into full- and half-symbol metrics.
8. The TBCC/WAVA decoder produces candidate payloads, which are checked by CRC
   and the reserved-bit rule.
9. A valid decoded signal is re-encoded and passed to `subtract_jtty` before
   additional searches so overlapping weaker signals can be recovered.
10. Accepted frames are merged by frequency/time into active messages, with
    duplicate and retro-window suppression.

## Transmit chain

`JttyMessages.hpp` / `prepareTransmitText` / `transmitFrame`
→ normalized 80-character message
→ source grammar packing
→ 34-bit JTTY payload
→ 46 information bits after CRC
→ rate-1/2 K=10 tail-biting convolutional code
→ 46 coded tones plus 13 sync tones
→ `gen_jttywave` 4-GFSK waveform.

## Deliberate Go-side differences

The JTTY-Go application assigns a presentation `Sequence` when a decode is
first accepted. This sequence is never changed by a continuation update. The
source signal's UTC start time remains separate. This is intentional so the
newest accepted decode stays at the bottom of the receive window even when an
older signal is recovered during a late/retro decode pass.
