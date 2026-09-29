# JTTY-Go Architecture

## Realtime RX path

```text
WASAPI event capture
        |
        v
PCMFrame (mono float32)
        |
        v
Sliding window: 4096 samples / 1024 hop @ 48 kHz
        |
        +-------------------> Waterfall event
        |
        v
FFT + robust noise floor
        |
        v
Local spectral candidate detector
        |
        v
Frequency tracker
        |
        v
JTTY Decoder (gated until protocol test vectors are validated)
        |
        v
DecodeStore -> monotonic Sequence -> UI append
```

The sliding hop is about 21.33 ms at 48 kHz. This is the main latency improvement over non-overlapping windows; it does not by itself constitute the complete JTTY decoder.

## Decode ordering invariant

`SignalUTC` is the estimated signal time. `ReceivedUTC` is the time the result is accepted by the application. `Sequence` is assigned only when a decode result is accepted. The GUI sorts only by `Sequence` and therefore never moves a late-decoded message above newer results.

## Audio policy

The realtime audio path must not wait on the decoder or GUI. `RingBuffer` drops oldest samples on overrun instead of blocking, because stale audio is not useful for a realtime decoder.

## QSO logging

QSO logging is operator-driven rather than TX-finish-driven. The first non-empty information send for the current DX callsign records a UTC start timestamp in memory. Clicking **记录通联** opens the QSO confirmation form; confirming it writes one ADIF record using that start time and the UTC timestamp captured when the record action was opened. TX completion no longer creates or finalizes a QSO automatically.

`NotifyTXStarted` is still used to capture the first-send timestamp, while `NotifyTXFinished` only emits the normal TX event. This keeps QSO logging independent of the radio backend and preserves the operator's ability to edit reports and contact details before committing the record.

## Windows audio backend

`internal/audio/manager_windows.go` and `internal/audio/wasapi_windows.go` use the maintained `github.com/degubites/go-wca` fork. This code is Windows-only and must be compiled on Windows with the dependency available. The Linux CI/test path intentionally uses a stub backend.

## JTTY decoder implementation

The Go JTTY decoder is now exercised by generated reference waveforms and source-codec vectors. Runtime decoding is enabled through `receiver.JTTYStream`, while the PHY remains isolated under `internal/jtty` so future WSJT-X parity work does not require GUI changes.
