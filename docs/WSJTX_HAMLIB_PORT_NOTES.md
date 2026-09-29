# WSJT-X Hamlib mapping notes

The radio configuration in JTTY-Go follows the structure of WSJT-X 3.2.0-rc1's `Configuration.ui`, `Configuration.cpp`, `Transceiver/TransceiverFactory.hpp/.cpp`, and `Transceiver/HamlibTransceiver.cpp`.

## Configuration groups retained

- Radio selection / Hamlib model
- CAT serial port, baud, data bits, stop bits, handshake
- PTT method: VOX / CAT / DTR / RTS
- PTT port
- Split mode: None / Rig / Emulate
- TX audio source: Front / Rear
- Poll interval
- Hamlib update / revert

Redundant display-only settings were removed from the main window. Audio devices are configured only in the Audio tab.

## Hamlib runtime path

JTTY-Go starts `rigctld.exe` locally when a local rigctld path is configured, using the selected Hamlib model and serial connection. The JTTY-Go client then uses the rigctld text protocol for frequency, mode and PTT operations.

The official Hamlib documentation defines `-l` for listing model IDs, `-m` for selecting a model, `-r` for the radio device, `-s` for serial speed and `-t` for the TCP port. The command protocol includes `F/f` for frequency, `M/m` for mode, and `T/t` for PTT.

## WSJT-X Hamlib DLL update flow

The Windows update flow mirrors the structure found in WSJT-X `Configuration.cpp`: download the selected 32/64-bit snapshot to `libhamlib-4_new.dll`, preserve the current DLL as `libhamlib-4_old.dll`, then install the new DLL and require a program restart for the new version to be loaded. Revert swaps the saved old DLL back into the active filename.

## Capability-driven configuration (v5.36-fixed11)

The radio settings dialog now separates device existence from device capabilities, matching the WSJT-X `set_rig_invariants()` model. Hamlib model capabilities are queried with `rigctl -m <model> -u` before the controls are enabled. Capability-dependent controls fail closed until the selected model's capability result arrives.
