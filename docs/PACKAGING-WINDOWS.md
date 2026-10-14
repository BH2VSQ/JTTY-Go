# JTTY-Go Windows packaging

JTTY-Go is built as a single-file Windows application. The executable embeds the frontend and uses Wails' embedded WebView2 bootstrapper. Radio control connects directly to an independently configured `rigctld` TCP service.

## Requirements

- Go
- Node.js and npm
- The project-pinned Wails v2 CLI
- Inno Setup 6 only when generating the installer

No Hamlib DLL, `rigctld.exe`, or `rigctl.exe` files are required in the JTTY-Go build or release package. The remote computer or radio-control host must run `rigctld` and expose its configured TCP port, normally 4532.

## Build

Run from the Windows development environment:

```powershell
cd C:\path\to\JTTY-Go
.\scripts\package-windows.ps1
```

To build only the portable package:

```powershell
.\scripts\package-windows.ps1 -SkipInstaller
```

To skip tests or clean before building, use `-SkipTests` or `-Clean`.

## WebView2

The build uses `-webview2 embed`. If a suitable WebView2 runtime is not present, Wails can use its embedded official bootstrapper.

## Output

```text
dist\windows\
  JTTY-Go-portable\
    JTTY-Go.exe
    LICENSE.txt
    licenses\...
  JTTY-Go-portable-windows.zip
  installer\
    JTTY-Go-*-Setup.exe
  SHA256SUMS.txt
```

User configuration and data remain in `%APPDATA%\JTTY-Go` and are preserved when the application is uninstalled.
