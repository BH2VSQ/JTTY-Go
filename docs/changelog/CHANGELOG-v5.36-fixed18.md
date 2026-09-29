# v5.36-fixed18

## Window minimum width

- Increased the native Windows/Wails main-window minimum width from 1080 to 1094 logical pixels.
- Kept the frontend 1080px layout baseline unchanged so the extra native client width provides the required margin for Windows non-client frame/DPI calculations.
- Updated the same minimum-width constant in `main.go` and `internal/app/app.go` so runtime size save/restore uses one consistent limit.
- No changes were made to the existing panel/grid layout.

This targets the observed case where the minimum window measured about 2178 physical pixels and the rightmost UI content became visible only after resizing to about 2205 physical pixels.
