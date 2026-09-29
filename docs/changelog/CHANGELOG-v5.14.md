# JTTY-Go Alpha v5.14

## Waterfall UI

- Reworked the integrated waterfall to use one persistent SVG overlay for all interactive ruler/marker elements.
- RX and TX now display an explicit 125 Hz JTTY bandwidth region instead of only a center line.
- Added a mouse-hover preview band with live frequency readout.
- Left mouse button sets RX center frequency.
- Right mouse button sets TX center frequency.
- Added a prominent WSJT-X-style frequency ruler with 100 Hz minor ticks and 500 Hz major labels.
- RX/TX state updates modify existing SVG elements only; they do not append marker nodes per click.
- Waterfall rendering and interactive overlay are isolated so frequency changes do not accumulate visual remnants.
