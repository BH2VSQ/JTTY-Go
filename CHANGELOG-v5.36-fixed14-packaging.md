# v5.36-fixed14 Packaging

- Added Windows packaging workflow for a portable single-file `JTTY-Go.exe`.
- The existing `go:embed all:frontend/dist all:bin` mechanism is used to carry the Hamlib runtime inside the executable.
- Added a Windows packaging script that validates the Hamlib runtime, builds Wails with an embedded WebView2 bootstrapper, and creates a portable ZIP.
- Added an Inno Setup script that produces a single-file Windows installer EXE.
- Installer uses a per-user install path under `%LOCALAPPDATA%` so first-run Hamlib materialization remains writable without administrator elevation.
- User configuration and log data under `%APPDATA%\JTTY-Go` are intentionally preserved on uninstall.
