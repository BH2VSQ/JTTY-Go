# JTTY-Go Alpha v5.13

## Waterfall marker fix

- Removed the RX/TX marker canvas layer.
- RX and TX indicators are now exactly two persistent DOM marker elements.
- Repeated frequency changes only update `left`, text and edge state; no canvas bitmap is rewritten.
- Pointer events remain on the waterfall surface, while marker elements use `pointer-events:none`.
- Fixed a feedback loop in the Wails frequency event path: local user changes notify the backend once; backend state updates are applied locally without echoing the event back.

## WSJT-X-style radio configuration

- Reorganized Radio settings around the WSJT-X `Configuration.ui` structure: Rig/Poll Interval, CAT Control, PTT Method, Mode, Split Operation, Transmit Audio Source, Test CAT/Test PTT, and Update Hamlib.
- Reduced duplicate/implementation-detail controls and kept advanced Hamlib/serial information in compact groups.
- Audio settings now follow the same dense WSJT-X Soundcard pattern: Input/Output device + channel, Refresh, buffer and a concise processing note.

## Radio connection convenience

- If `rigctldPath` is empty, the application now searches beside the executable for `rigctld.exe`/`rigctl.exe` before giving up on auto-start.
