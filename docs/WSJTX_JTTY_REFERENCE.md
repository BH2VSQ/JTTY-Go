# WSJT-X JTTY implementation reference

JTTY-Go uses the JTTY implementation in the user-provided WSJT-X 3.2.0-rc1
source archive as the protocol/algorithm reference. The selected reference
sources are preserved under `reference/wsjtx-jtty/`.

## Runtime mapping

| WSJT-X source responsibility | JTTY-Go implementation |
| --- | --- |
| `rjtty_sub.f90` / `rjtty_core` | `internal/receiver/jtty_stream.go` |
| `jtty_mdecode_step` | stream window ordering + retro-window scheduling boundary |
| `jtty_mdecode` | `internal/jtty/decoder.go` candidate search, validation and subtraction |
| `jtty_peakup` | `internal/jtty/decoder.go` peak-up |
| `jtty_payload_correlators.f90` | `internal/jtty/tbcc.go` correlations |
| `jtty_tbcc_decoder.f90` / list decoder | `internal/jtty/tbcc.go` |
| `jtty_source_codec.f90` | `internal/jtty/source_codec.go` |
| `jtty_mod.f90` / `gen_jttywave.f90` | `internal/jtty/profile.go`, `waveform.go` |
| `subtract_jtty.f90` | `internal/jtty/decoder.go` |
| `JttyMessages.hpp` | `internal/jtty/profile.go` + macro engine |
| `mainwindow_jtty.cpp` | Wails frontend + `internal/app` |

## Source-derived timing constants

At the normal 12 kHz audio rate used by the JTTY reference:

- samples/symbol: `384`
- symbols/frame: `59`
- samples/frame: `22656`
- search step: `nframe/4 = 5664` samples
- processing window: `nframe + nframe/4 = 28320` samples
- retro depth: 3 quarter-frame steps

The Go realtime stream now uses these 12 kHz quantities before entering the
same analytic 6 kHz domain used by the WSJT-X decoder.

## Search strategy

The Go decoder follows the source geometry:

1. analytic conversion to the 6 kHz complex domain;
2. 8192-point sync search;
3. 12-sample search grid (2 ms);
4. five-bin weighted smoothing `1,2,3,2,1`;
5. local suppression around accepted peaks;
6. JTTY peak-up for primary-channel candidates;
7. payload correlation and TBCC/WAVA + CRC validation;
8. decoded-signal subtraction and follow-up search.

The live receiver additionally assigns an application-level `Sequence` when a
decode result is accepted. This deliberately differs from `SignalUTC`, so a
late-decoded older signal cannot move above a newer displayed result.
