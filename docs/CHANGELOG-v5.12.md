# v5.12

## Waterfall marker rendering redesign

- Waterfall bitmap and RX/TX marker bitmap are now two independent canvases.
- The waterfall canvas never draws RX/TX markers.
- The marker canvas is cleared at the device-pixel level before every marker update.
- RX/TX mouse handling is attached to `waterfall-surface`, not to either canvas.
- Waterfall refreshes no longer repaint or accumulate marker strokes.
- Left click sets RX frequency; right click sets TX frequency.
- RX remains green and TX remains red even when the two frequencies are close.
