# JTTY-Go 验证记录 — 2026-10-14

基线：`ac82047`。本地分支：`feat/local-hamlib-jtty-sync`。没有推送、打标签或发布 Release。当前修复版编号为 5.36.15，安装包文件名已更新以区别上一版。

## 5.36.15：选择 Hamlib 卡死及底部遮挡修复

- 根因：型号、能力、版本及串口列表的请求事件与返回事件重名。Wails 2.15.0 的 `Events.Emit` 会同时通知 Go 与前端，缓存回复再次触发请求，形成无限回送；与是否监听无关。上一轮只向 JavaScript 派发回复的模拟测试漏掉了这条路径。
- 请求改为 `hamlib:models:request`、`hamlib:capabilities:request`、`hamlib:status:request`、`radio:serial-ports:request`；返回名称保持原有协议。DX 呼号/网格的设置请求也与状态通知分离。
- PTT 选项改为两列；控件高 26px，卡片间距及字体缩小；设置内容区允许纵向滚动，底栏保持独立。
- 新增 Go 回送回归测试，覆盖首次真实 Hamlib 型号加载、缓存查询和四类请求/响应，确保一条请求只返回一次。
- 使用真实 314 个 Hamlib 型号，在 1360×820、1094×680、1280×640 三种视口下反复切换后端、保存独立配置、检查 PTT 与本地连接按钮不会被底栏覆盖。截图在 `.codex/verification/freeze-ui/`。
- 通过 Wails 开发服务器连接真实 Go 后端，完成 21 次后端切换、11 次型号回复、空闲等待无事件风暴、Dummy CAT/PTT 开关、保存和断开。这次没有模拟 Go 业务处理。
- 完整 Go 测试、相关模块 race/vet、TypeScript 检查、浏览器回归与 Windows 打包均通过。本次未改变 DSP/FEC 算法，也不将测试环境结果换算成用户电脑的 CPU 百分比。
- Wails 开发模式 `/wails/ipc.js` 自带调试面板出现 Svelte `nodes` 异常和 `runtime:ready` 提示，单独记录为 SDK 开发工具限制；实际 Go 绑定调用及上述业务流程通过。原生发布版完整 GUI 自动化、实际电台/RF 仍未验证。

## 已通过

- Go 全模块测试：`go test ./internal/... ./tests/... ./cmd/...`。包含独立生成的 WSJT-X 编码/声调向量、PCM 录音样本及配置迁移测试。
- Race：`go test -race ./internal/radio ./internal/app ./internal/model ./internal/jtty ./internal/receiver`。测试时设置 `JTTY_ORACLE`，完整执行独立 Fortran 对照。
- 修改模块静态检查：`go vet ./internal/radio ./internal/app ./internal/model ./internal/jtty ./internal/receiver`。
- 前端：`npx tsc --noEmit`、Vite 构建；使用现有 Edge 自动检查两种后端切换、分别保存配置、PTT 不改变后端、NET 配合 VOX、取消恢复和无默认串口选择。该界面检查使用模拟 Wails 事件通道。
- 真实 Hamlib 4.7.3~rc Dummy：应用自动启动本地服务，频率/模式读写、PTT 开关、Split 开关及频率设置、关闭子进程、型号错误诊断、避开占用的 NET 端口、取消测试后恢复正式连接。关闭 NET 客户端不停止其外部服务器。
- 协议故障：拒绝非法 PTT 回复；查询超时不会阻塞 PTT-OFF；失步连接不复用迟到响应；故障后拒绝自动恢复发射。
- 独立 WSJT-X 提交 `9d059511f77e2d1be746ebfeaa7f88e77c1babb7`：10 组消息载荷位/声调一致，Fortran→Go 与 Go→Fortran 双向解码通过；约 -8 dB 的固定噪声样本与重叠信号解码通过，弱信号 SNR 差异不超过 2 dB。
- 长消息超过 80 字符的分段/组装、最终 EOM、填充文本过滤、消息完成/超时/接收结束及首帧 SNR 保持通过。发送使用一次连续波形提交，避免逐帧排空音频缓冲造成间隙。
- Windows amd64 Wails 构建、便携版与 Inno Setup 安装包编译通过。两种包均在干净目录解包，验证完整 Hamlib 文件集的 SHA-256 和 PE amd64 架构；包内应用 EXE 哈希相同，并用解包后的运行文件重跑真实 Dummy 测试。

## 验证边界

- 打包后的原生应用已启动并正常写出隔离目录中的设置。通过 CDP 自动操作原生 WebView2 的尝试未完成：Wails 使用的 go-webview2 明确清除调试环境变量，因此无法开启测试调试端口。没有将浏览器模拟事件检查称为原生 GUI→Go→电台的完整端到端验证；替代依据是浏览器交互、生成的 Wails 类型/绑定、应用层真实 Dummy 集成测试及打包程序启动检查。
- 未进行实际电台 CAT、串口 RTS/DTR、电台数据音频输入、声卡连续播放或 RF/PTT 实机测试。Dummy 结果不代表具体电台硬件已经适配通过。
- 全仓库 `go vet` 仍报告 `internal/audio/output_windows.go`、`internal/audio/wasapi_windows.go` 中原有的两处 Windows unsafe.Pointer 警告；本次修改模块的 vet 通过。
- 使用 Go 1.27.2、Wails 2.15.0、现有 MSYS2 gfortran/FFTW。新增工具、缓存和临时运行包均放在 H 盘。Inno 编译器静默安装被自动审批拒绝，改用已验证签名的官方安装文件进行便携解包，未运行该安装程序。
- 临时工具清理命令被自动审批拒绝（仅返回 blocked by policy），因此 `H:\tools\jtty-go` 仍保留工具、缓存、独立 oracle 及解包测试目录；测试服务已停止。日志和截图位于项目 `.codex/verification/`。

## 交付与回滚

- `dist/windows/JTTY-Go-portable-windows.zip`：包含应用、完整 `hamlib/` 和许可文件。
- `dist/windows/installer/JTTY-Go-5.36.15-Setup.exe`：本地安装测试包，尚未执行安装/卸载回归。
- 摘要见 `dist/windows/SHA256SUMS.txt`。校验清单同时覆盖包内应用及 Hamlib 可执行文件。
- 便携 ZIP SHA-256：`21207e2a6daf4239c37e14cddea7ff365a3c0163d57ba3f6be9f2b7e7fc846e6`。
- 安装包 SHA-256：`915d89e800a4cbc623cf32185d50f0244a224dbc51f064d1d801fe4fcadb5a31`。
- 升级/回滚须替换完整应用包，禁止单独替换 DLL。回滚前备份用户配置；旧版本可能重新将本地后端迁移为 NET，因此应同时恢复对应旧配置。
- `.codex/` 与 `docs/` 留在本地，不纳入本次代码提交。
