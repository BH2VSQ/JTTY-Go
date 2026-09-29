# JTTY-Go v5.36-fixed11

## 电台设置与测试

- 修复 Hamlib 电台选择后右侧“模式 / 异频操作 / 发射音频源 / 无线电设备数据”仍保持灰色禁用的问题。
- PTT 方法不再因为选择实际电台而把已保存的 VOX 强制显示成 CAT。
- Test CAT 按 WSJT-X 逻辑：使用当前设置草稿进行连接测试，成功后保持测试连接在线。
- Test PTT 改为可勾选的持续状态：点击激活，再次点击关闭，不再使用固定 300 ms 自动释放。
- Test PTT 仅在 CAT / DTR / RTS 且 Test CAT 已成功后启用；VOX 不提供独立 PTT 测试。
- Cancel / Close / Apply / OK 前关闭临时 CAT/PTT 测试会话，避免与正式电台连接争用串口。
- DTR/RTS 测试支持独立 PTT 串口及强制 DTR/RTS 电平。
- 保持既有设置窗口布局不变，仅增加状态控制和测试反馈。

## WSJT-X 对齐依据

当前实现对应 WSJT-X `Configuration.cpp` 中 `set_rig_invariants()`、`on_test_CAT_push_button_clicked()`、`on_test_PTT_push_button_clicked(bool)` 与 `handle_transceiver_update()` 的关键行为：Test CAT 在当前草稿配置下建立 rig；Test PTT 为 checkable 控件，点击维持状态；在线后才允许 PTT 测试。

### Fixed in v5.36-fixed10
- Fixed a Go compile error in `internal/app/app.go` caused by using the short-lived `e` variable outside the scope of the `if` initializer in the Test CAT/ridctld startup retry loop.
- The retry loop now stores the latest `r.Open()` error in `openErr`, preserving the intended behavior while making the variable scope valid.
