# v5.21

## Functional integration

- 按 WSJT-X 的模块职责开始将现有 Audio / Decoder / Transceiver / Logbook 边界接入真实运行时。
- Windows WASAPI 输入、JTTY 12 kHz 发送、RX/TX PTT 已连接到现有 Wails 事件结构。
- `rigctld` 频率、模式、PTT、split、轮询及本机 rigctld 自动启动已接入。
- DTR / RTS PTT 与 Force DTR / Force RTS 进入实际控制逻辑。
- RX 录音接入实时 PCM，按 UTC 日期自动分 WAV 文件。
- ALL 解码日志继续按现有单文件/按年/按月策略记录。
- 手动 QSO 日志完全使用操作员触发的记录事务；TX 完成不再自动写 ADIF。
- TX generation 与 receiver locking 增强生命周期并发安全。
- 解码菜单的参数选择保存后即时关闭菜单；阈值和 tracker 参数可以直接更新运行中的解码管线。
- 清理此前遗留的音频默认目录解释性 UI 文案，保持已确认的界面结构不变。

## Tests

- Core Go tests passed for audio/decode/detector/dsp/jtty/logbook/model/macro/radio/receiver/tx/waterfall/tests。
- App package tests passed using a local Wails runtime stub用于编译隔离验证。
- Frontend `main.ts` standalone TypeScript type/syntax check passed；完整 Vite 构建仍需在具备项目 npm 依赖的 Windows 开发环境执行。
