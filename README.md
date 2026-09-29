## v5.14 UI update
- Integrated WSJT-X-style waterfall ruler.
- 125 Hz RX/TX bandwidth regions.
- Hover frequency preview band.
- Left-click RX / right-click TX.

# JTTY-Go

JTTY-only weak-signal keyboard terminal written in Go for Windows, designed around a WSJT-X-like operating model but with a continuous receive pipeline and stable decode ordering.

## Current Alpha

- Wails v2 GUI shell with integrated waterfall and two-column RX/QSO layout.
- Pure-Go core packages for FFT, noise-floor estimation, local spectral candidates, candidate tracking, decode ordering, macro templates, QSO automation models, TX queue, ADIF model, and radio interface.
- Sliding FFT windows (default 4096 samples / 1024 hop at 48 kHz) so candidate search runs about every 21.3 ms instead of waiting for non-overlapping windows.
- DecodeStore assigns a monotonic sequence when a decode result is accepted; UI order never sorts by SignalUTC.
- Ring buffer discards oldest samples on overrun to preserve low latency.
- Windows audio abstraction with a WASAPI/go-wca capture and render implementation.

## Implementation status

The JTTY physical codec path is implemented in Go against the preserved WSJT-X JTTY reference, including source packing, 4-FSK/TBCC framing, waveform generation, sync/peak search, payload correlation, list decoding, CRC validation, decoded-signal subtraction, and stateful message assembly. See `docs/WSJTX_JTTY_REFERENCE.md` and `docs/FUNCTION_INTEGRATION_v5.21.md / docs/CHANGELOG-v5.23.md`.

## Build on Windows

Install Go 1.23+, Node.js, and the Wails v2 CLI. Then from the repository root:

```powershell
wails doctor
wails build
```

The Windows Core Audio backend uses the maintained `github.com/degubites/go-wca` fork (v0.4.1), which is pure Go/no-CGO and supports event-driven capture. The fork includes several correctness fixes over the original repository.

## Layout

```text
Audio -> Sliding FFT -> Detector -> Candidate Tracker -> JTTY Decoder -> DecodeEvent
                                           |                   |
                                           v                   v
                                       Waterfall          Decode Window
                                                            |
                                     DX Call <-------------+
                                                            |
                                                     QSO Automation
```

## WSJT-X JTTY reference

The JTTY codec and receiver path are being reimplemented in Go against the
user-provided WSJT-X 3.2.0-rc1 source. The selected upstream JTTY sources are
preserved under `reference/wsjtx-jtty/`; see `docs/WSJTX_JTTY_REFERENCE.md`.


## v5.2 Windows audio compatibility patch

The Windows audio backend is written against the actual `github.com/degubites/go-wca@v0.4.1` API.
Important compatibility points:

- `ole.CoInitializeEx` returns `nil` for `S_OK` and may return an `*ole.OleError` carrying `S_FALSE`; `S_FALSE` is accepted.
- `IMMDeviceEnumerator.EnumAudioEndpoints` receives `uint32`, so `wca.EDataFlow` is explicitly converted.
- `go-wca@v0.4.1` exposes `CreateEventExA` as an error-only function, so the returned Win32 HANDLE cannot be obtained through that wrapper. The capture backend therefore uses the native `kernel32!CreateEventW` call for the WASAPI event handle.

After extracting the project on Windows:

```powershell
go mod tidy
go test ./internal/... ./tests/...
```


## v5.7 UI

The waterfall is integrated into the main workstation window. Left click sets RX audio frequency; right click sets TX audio frequency. Decode panes hide scrollbars while retaining mouse-wheel scrolling.

## UI v5.8

The UI now follows the supplied WSJT-X-style reference: compact gray toolbars, integrated waterfall, two-column decode/QSO layout, hidden scrollbars, and a tabbed Configuration dialog. Waterfall LMB selects RX frequency; RMB selects TX frequency.

## UI / configuration v5.9

The main window intentionally exposes only two top-level menus: `文件` and `配置`.
All audio and radio device selection is performed in the Configuration dialog.
The main window does not contain an audio-device selector.

The Configuration dialog is reduced to six practical pages:

1. 常规 - station callsign/grid and startup/display behavior.
2. 电台 - Hamlib/rigctld, CAT serial parameters, PTT, split, TX audio source, polling, Test CAT/PTT, Hamlib update/revert.
3. 音频 - Input/Output devices, channels, refresh, and buffer.
4. JTTY - decoder/search and waterfall parameters.
5. 快捷消息 - F1-F8 templates and global-hotkey settings.
6. 日志 - ADIF path and automatic QSO logging rules.

This intentionally merges the former Display and Automation pages to avoid redundant settings.

## Waterfall interaction

- Left click: set RX frequency.
- Right click: set TX frequency.
- RX/TX lines are redrawn from a clean canvas on every frame.
- RX/TX coordinate mapping uses the configured fixed frequency range instead of the transient FFT frame start, preventing stale TX-line residue and flicker after repeated RX/TX frequency changes.


## UI / radio v5.10

Main window top-level menus are intentionally limited to `文件` and `配置`. Audio devices are configured only in `配置 → 音频`. Radio setup is concentrated in `配置 → 电台`.

The radio backend is currently implemented through Hamlib `rigctld`, including frequency, mode, PTT and split-TX frequency control. A configured local `rigctld.exe` can be started automatically. Hamlib DLL update/revert targets the rigctld directory when that path is configured.

## v5.10.1 UI/radio adjustments
- Main menu reduced to 文件 / 配置.
- Waterfall RX/TX markers are DOM overlays; the canvas is cleared independently, preventing stale TX line residue/flicker after rapid frequency changes.
- Left click sets RX audio offset; right click sets TX audio offset. These offsets no longer write 1500 Hz-style audio offsets into the radio RF dial.
- Main screen has no audio-device selectors; audio devices live only in 配置 → 音频.
- Radio configuration follows the WSJT-X grouping: radio selection, CAT parameters, PTT, split, mode/passband, polling, and Hamlib update/revert.
- Hamlib radio model list can be populated from rigctld/rigctl `-l`.


## v5.11 waterfall fix

RX/TX markers are rendered directly onto the waterfall canvas rather than as separate DOM overlays. This prevents stale TX marker repaint artifacts during repeated RX/TX changes and keeps the RX marker visible.


## v5.12 waterfall marker architecture

The waterfall display uses independent render layers: the spectrum/waterfall bitmap is rendered on one canvas and RX/TX markers on a second transparent canvas. The marker canvas is explicitly cleared before each update, preventing stale marker trails during repeated RX/TX frequency changes.

## v5.15 operating area
- Reworked the main lower operating area to follow the supplied WSJT-X control layout.
- Moved the main RF frequency display into the operating area.
- Added band buttons, compact control toolbar, RX/TX offset fields, DX call/grid, UTC clock, serial number, message input and F1-F8 macro controls.
- Removed the former separate rig bar from the main window.
- Audio device selection remains in Configuration -> Audio only.

## v5.17 UI/layout changes

v5.17 follows the latest WSJT-X-style operating-area reference supplied for JTTY-Go. The waterfall title bar is removed; the frequency ruler is compact; the legacy mode and lookup controls are removed; TX/RX/tolerance use a single row of spinner inputs; `←` sets TX=RX and `→` sets RX=TX; the main RF frequency field is editable and commits changes to the radio backend; and the lower-left level control is explicitly TX output-audio level.

The main window now has two draggable horizontal splitters. Their positions are constrained by minimum region heights and persisted in the user settings.

## v5.18 changes

- TX 输出音频电平改为横向滑条，放置在主频率显示下方；不再显示“JTTY —”模式文字。
- 操作区固定为 270 px，不再提供解码区与操作区之间的拖动分隔线；仅保留瀑布图与解码区之间的分隔线。
- 数据目录默认位于 `%LOCALAPPDATA%\JTTY-Go`。
- 自动保存全部解码到 `ALL.txt`，支持单文件、按年 `ALL-YYYY.txt`、按月 `ALL-YYYY-MM.txt` 三种模式。
- 通联自动日志固定保存为 `JTTY.adi`。
- 文件菜单提供：删除 ALL.txt、删除通联日志 JTTY.adi、打开日志目录、退出软件。
