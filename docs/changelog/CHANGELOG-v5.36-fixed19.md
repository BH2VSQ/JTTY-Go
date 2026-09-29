# v5.36-fixed19

- 修复预置消息 `%Q` 宏未使用“Call next”输入的问题。
- `Call next` 作为独立的会话状态保存，不再与当前 `DX 呼号` 或发送队列混用。
- 输入 `Call next` 后立即同步到后端；点击预置消息或使用全局快捷键时，`%Q` 自动展开为当前 Call next 呼号。
- 后端不再对每次字符输入回发 UI 事件，避免不必要的事件往返。
- 保留 `TriggerMacro` / `RenderMacro` 的显式参数覆盖能力：调用方提供非空值时优先使用传入的 Call next。
- 增加应用层回归测试，验证 F6 `TU NOW %Q %E` 的展开结果。
