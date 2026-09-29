# JTTY-Go v5.24

本版本继续保持既有界面和模块边界，仅补齐功能对接并修复实际运行问题。

## 频率

- 波段按钮实际切换拨号频率。
- 无 CAT/VOX 模式下，频率切换只更新软件拨号频率，不尝试建立 CAT 连接。
- Hamlib/rigctld 模式下，频率切换通过 `F <Hz>` 写入电台；配置了模式时同步尝试写入模式。
- 20 m 默认频率为 14.090000 MHz。
- 波段切换失败时保持原选中波段并显示错误。

## 电台控制

- 新默认值为 `VOX / 直通音频`。
- 默认不建立 Hamlib/rigctld 连接。
- 选择 Hamlib 并指定有效电台型号后才启用 CAT 区域。
- PTT 选择 VOX 时禁用 PTT 测试和 PTT 串口；DTR/RTS 时启用对应串口配置。
- 本地自动启动 `rigctld` 仅在配置了串口和有效型号时执行。

## 音频

- Windows 音频设备枚举显示 Windows Core Audio 的 Friendly Name，而不是内部 Endpoint ID。
- 音频输入保持 48 kHz 交付到接收处理链；非 48 kHz 的 WASAPI 混音格式先重采样。
- 监听启动/停止事件异步执行，避免阻塞 Wails UI 事件线程。
- 停止监听一定发送 `audio:state`，保证按钮状态可以恢复。
- 首屏不再启动时立即枚举所有音频设备；进入音频设置时再刷新设备列表。

## QSO

- QSO 记录包含 `GRIDSQUARE`。
- 未在确认窗口中重新填写网格时，使用当前 DX 网格。
- 解码中发现有效 Maidenhead 网格时，点击呼号会同时填入 DX 网格。

## 验证

- `frontend/src/main.ts` TypeScript standalone check：通过。
- `go test ./internal/app`：通过（使用本地 Wails runtime stub 验证应用包逻辑）。
- 主要纯 Go 包测试：通过。
- 完整 Wails/Windows 构建需要在开发机执行，因为验证容器无法访问 Go module proxy。
