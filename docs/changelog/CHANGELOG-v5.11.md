# v5.11

## Waterfall RX/TX marker fix

- Removed separate DOM RX/TX marker overlays.
- RX/TX markers are drawn in the same canvas render pass as the waterfall.
- The whole canvas is cleared/redrawn on every render, eliminating stale TX marker lines after repeated frequency changes.
- RX and TX are separated by one pixel when they are visually coincident, so RX stays visible.
- Left click sets RX frequency.
- Right click sets TX frequency.
