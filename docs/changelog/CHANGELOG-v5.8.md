# JTTY-Go Alpha v5.8

- Redesigned the main UI to follow the compact WSJT-X-style workstation shown in the supplied references.
- Integrated the waterfall into the main window rather than using a separate graph window.
- Left mouse button on the waterfall sets RX frequency; right mouse button sets TX frequency.
- Removed visible scrollbars from the main decode/QSO areas while keeping mouse-wheel scrolling.
- Added a WSJT-X-style Configuration dialog with tabs for General, Audio, Radio, JTTY, Macros, Logging, Automation, and Display.
- Added settings persistence through the Wails event bridge (`settings:save` / `settings:state`).
- Kept UTC/SNR/DT/DF/message decode columns and newest-at-bottom behavior.
