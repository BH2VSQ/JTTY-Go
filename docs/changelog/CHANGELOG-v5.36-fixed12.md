# JTTY-Go v5.36-fixed12

## Realtime performance optimization

This release focuses on reducing CPU usage, GC pressure, and UI-induced receiver latency without changing the main UI layout.

### Receiver / DSP
- Removed the redundant full PCM copy in `receiver.Pipeline.Process`.
- Reworked 48 kHz -> 12 kHz decimation in `JTTYStream` to use reusable scratch storage and in-place averaging.
- Reused detector noise-floor scratch storage instead of allocating/copying the full FFT power vector on every scan.
- Reduced detector scan cadence from every other hop to every third hop; the JTTY forward decoder remains on its independent 1/4-frame schedule.
- Reduced the display-only waterfall FFT from 16384/4096 to 8192/2048. The visual update cadence stays about the same while FFT cost is lower.
- Rate-limited high-frequency detector/waterfall events sent to the WebView to at most about 8.3 updates/s per stream; dropped UI frames never block the receiver DSP path.

### JTTY scheduler
- Automatic decoder worker count is now 2 on normal CPUs and 1 on dual-core/2-thread systems; explicit thread settings remain available.
- Wideband fallback no longer runs every fourth window. It is now a low-rate periodic sweep (roughly every twelfth forward window) and is skipped when the realtime queue is already under load.
- Retro refinement is limited to one optional previous-window revisit and is skipped when the forward queue is busy.
- Realtime forward work keeps priority over optional refinement work.

### Expected effect
- Lower steady-state CPU usage.
- Lower allocation rate and GC frequency.
- Less chance that a decode burst blocks the realtime audio pipeline long enough to create approximately one-second UI/receiver lag.
- No intentional change to the chronological decode ordering, QSO TX-only list behavior, Hamlib configuration UI, CAT/PTT test workflow, or shortcut-name startup loading.
