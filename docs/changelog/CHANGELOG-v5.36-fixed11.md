# JTTY-Go v5.36-fixed11

## 电台配置能力判定

本版本将电台设置页的启用/禁用逻辑集中到 WSJT-X 风格的 `set_rig_invariants()` 规则：

- `None` 时禁用 CAT、Test CAT、模式、异频、发射音频源以及电台数据；VOX 可用；DTR/RTS 可用于独立串口 PTT 测试。
- CAT PTT 只有在 Hamlib model capability 明确支持 CAT PTT 时才可选择。
- PTT 串口仅在 DTR/RTS 时启用；特殊 `CAT` PTT 端口只有 `has_CAT_indirect_serial_PTT` 时才可用。
- RTS 按 WSJT-X 的“串行 CAT + 同一端口 + 硬件握手”冲突规则禁用。
- 串口参数仅在 Hamlib CAT port type 为 serial 时启用。
- 异步 CAT 设备禁用轮询间隔。
- 发射音频源仅在 CAT PTT + 设备支持 CAT mic/data 切换时启用。
- 异频 `Rig` 选项要求设备同时支持 split VFO 和 split frequency 的读写；`Fake It` 不依赖设备 split VFO。
- 模式列表在 Hamlib capability 可用时对不支持的模式直接禁用。

Hamlib capability 通过 `rigctl -m <model> -u` 获取；无法解析 capability 时相关依赖项采取保守禁用，而不是误判为支持。

参考 WSJT-X 当前 master 的 `Configuration.cpp::set_rig_invariants()` 与 `TransceiverFactory::Capabilities`。
