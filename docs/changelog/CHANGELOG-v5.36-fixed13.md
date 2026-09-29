# JTTY-Go v5.36-fixed13

## Realtime performance / stability correction

v5.36-fixed13 backs away from the most aggressive scheduler throttling introduced in fixed12. The goal is to reduce latency and resource spikes **without reducing the normal JTTY decode opportunity**.

### Decode scheduler
- Restored automatic decoder worker selection to the available logical CPU count capped at 3.
- Forward jobs are no longer discarded after dequeue merely because a newer sequence has arrived; only queued backlog is dropped when the realtime queue is full.
- Detector-hinted wideband fallback runs once every 8 forward windows; when there are no detector hints it remains once every 4 windows.
- Retro refinement remains limited to one nearest previous-window revisit and only runs when the realtime queue is idle.
- JTTY PHY decoder context cancellation remains enabled so receiver shutdown can interrupt in-flight optional work promptly.

### TX / RX lifecycle
- Simplex TX no longer tears down and recreates the complete WASAPI + detector + JTTY decoder stack for every transmission.
- During TX, the existing receiver pipeline is paused and resumes in place after TX. This removes repeated worker/thread/device creation and destruction that could accumulate latency and resource pressure after multiple transmissions.
- TX waveform generation uses reusable caller-owned scratch buffers.
- Windows audio output now scales the TX PCM buffer in place instead of creating a second full-size PCM allocation.
- TX progress UI event cadence is reduced from 20 Hz to 10 Hz; frame-boundary progress events are retained.

### Capture / DSP allocation
- 48 kHz -> 12 kHz decimation now consumes input samples directly instead of copying every capture packet into a second scratch buffer.
- The JTTY TBCC/WAVA hot path continues to use fixed storage / scratch pooling; its decoded candidates are preserved against the fixed11 behavior.
- Retro-seen bookkeeping is bounded to avoid unbounded map growth during long-running sessions.

### Functional behavior preserved
- Decode list remains UTC old -> new, newest at the bottom.
- QSO frequency list remains TX-only and UTC ordered.
- Startup shortcut names/remarks are loaded automatically.
- Hamlib manufacturer + model ordering and None-first behavior remain unchanged.
- Device capability based settings enable/disable behavior and CAT/PTT test workflow remain unchanged.
