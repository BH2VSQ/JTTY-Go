# JTTY-Go Alpha v5

- Added the user-supplied WSJT-X 3.2.0-rc1 JTTY reference sources under `reference/wsjtx-jtty/`.
- Added upstream GPL-3.0 text and a SHA-256 record for the supplied source archive.
- Corrected the realtime JTTY frame geometry at 12 kHz: 384 samples/symbol, 22656 samples/frame, 28320 samples/window, 5664 samples/quarter-frame.
- Added source-oriented `DecodedFrame`/`KnownInterferer` APIs so retro rescans can subtract the exact error-corrected 59-tone signal.
- Connected up to three retro quarter-frame rescans to the realtime receiver.
- Reordered multi-channel decoding to follow the WSJT-X JTTY phase ordering: primary QSO channel first, side channels after subtraction.
- Kept primary-channel peak-up separate from side-channel peak search, matching `jtty_mdecode`.
- Added stable `decode:updated` event handling in the frontend for incremental JTTY message assembly.
- Updated default F1-F8 presets to the current WSJT-X JTTY source defaults.
- Added a realtime stream decode test using arbitrarily sized 48 kHz input packets.


## v5.2

- Fixed Windows `go-wca@v0.4.1` API type usage.
- Fixed COM initialization handling for `S_FALSE`.
- Fixed `EDataFlow` conversion for `EnumAudioEndpoints`.
- Replaced the unusable error-only `go-wca.CreateEventExA` call with native `kernel32!CreateEventW`.
- Restored `go-ole` import for COM task-memory/free and uninitialization.
