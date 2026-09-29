# JTTY-Go v5.20

## Settings and configuration UI

- Removed the editable main-window layout controls from the General settings page; layout remains an internal persisted state controlled by the main-window splitter.
- Removed the old JTTY settings tab.
- Added a top-level **解码** menu with separate submenus for decoder configuration:
  - threads
  - detection threshold
  - frequency tolerance
  - tracker tolerance
  - tracker timeout
  - decode-window limit
  - decoder mode
- Decoder menu changes are persisted to the normal configuration file. Parameters that require pipeline reconstruction take effect after the next receiver start; frequency tolerance is also applied immediately through the existing live control path.
- Spectrum-search range, FFT size and Hop are now compile-time constants in `internal/detector/search_config.go` and are not user-editable.

## Radio settings

- Reworked the Radio settings page to a WSJT-X-style layout with:
  - radio device / connection host / polling interval
  - CAT serial control and serial parameters
  - PTT method and PTT serial port
  - radio audio source
  - mode and split-operation selections
  - CAT/PTT test controls
  - Hamlib update / restore controls and status
  - DTR / RTS forced-control-line settings
### Decoder menu selection
- Decoder submenu selections are saved immediately and the decode menu closes automatically after a selection.
