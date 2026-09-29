# JTTY-Go v5.36-fixed14

## Waterfall scrolling correction

- Fixed the waterfall renderer so every incoming spectrum frame is inserted at the top and the existing history shifts downward by one pixel.
- Removed the incorrect circular-buffer display order that caused new signal rows to flash at the top while leaving the historical image stationary/blank underneath.
- The display bitmap now stores rows in display order (newest first), matching WSJT-X-style downward waterfall motion.
- Only the visible waterfall history is shifted for each new frame to avoid unnecessary canvas work.
- Frequency markers, hover handling, range changes, and existing waterfall layout remain unchanged.
