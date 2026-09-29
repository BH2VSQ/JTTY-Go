# WSJT-X configuration reference used by JTTY-Go

JTTY-Go v5.9 uses the user-provided WSJT-X 3.2.0-rc1 source archive as the reference for configuration semantics and the radio/audio grouping.

Relevant upstream files:

- `Configuration.ui` - CAT Control, Serial Port Parameters, PTT Method, Update Hamlib, Soundcard Input/Output/Channel layout.
- `Configuration.cpp` - Hamlib update download, backup, replacement and revert behavior.
- `Transceiver/TransceiverFactory.hpp` - CAT serial parameters, PTT methods, split modes, TX audio source and polling settings.
- `Transceiver/HamlibTransceiver.cpp` - Hamlib radio control semantics and VFO/split handling.

JTTY-Go does not copy the Qt UI; it recreates the same functional grouping in the Wails frontend.

## Hamlib update behavior

The Windows updater follows the same operational model used by WSJT-X:

1. Download `libhamlib-4.dll` to `libhamlib-4_new.dll`.
2. Keep the download under a temporary filename until it has completed.
3. Move the current `libhamlib-4.dll` to `libhamlib-4_old.dll`.
4. Install the new DLL under `libhamlib-4.dll`.
5. Keep the old DLL so Revert Update can restore it.
6. A restart is required before a newly installed DLL can be used by an in-process Hamlib backend.

The default snapshot URLs mirror the WSJT-X 3.2.0-rc1 source implementation:

- 64-bit: `https://hamlib.sourceforge.net/snapshots-4.7/dll64/libhamlib-4.dll`
- 32-bit: `https://hamlib.sourceforge.net/snapshots-4.7/dll32/libhamlib-4.dll`
