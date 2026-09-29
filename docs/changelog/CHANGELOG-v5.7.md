# JTTY-Go v5.7

## UI / waterfall

- Reworked the main window to follow the dense WSJT-X-style workstation layout.
- Waterfall is integrated directly above the RX/TX work areas.
- No visible scrollbars; decode lists still support mouse-wheel scrolling.
- Left click on the waterfall changes RX audio frequency.
- Right click on the waterfall changes TX audio frequency.
- RX/TX frequency markers are drawn directly on the waterfall.
- Added a QSO-frequency decode pane on the right side.
- Callsigns remain individually clickable to populate DX Call.
- Latest decodes continue to append at the bottom in the left All Decodes pane.

## Core

- Added RX/TX frequency state and Wails runtime event handlers.
- Changing RX frequency while running updates the JTTY stream center frequency.
- Separated frontend frequency-set events from backend frequency-state events to avoid event feedback loops.
