# JTTY-Go 验证记录 — 2026-10-14

基线：`ac82047`。本地分支：`feat/local-hamlib-jtty-sync`。没有推送、打标签或发布 Release。发行物为本地测试包，安装包沿用原有版本号 5.36.14。

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

## 交付与回滚

- `dist/windows/JTTY-Go-portable-windows.zip`：包含应用、完整 `hamlib/` 和许可文件。
- `dist/windows/installer/JTTY-Go-5.36.14-Setup.exe`：本地安装测试包，尚未执行安装/卸载回归。
- 摘要见 `dist/windows/SHA256SUMS.txt`。校验清单同时覆盖包内应用及 Hamlib 可执行文件。
- 便携 ZIP SHA-256：`cbafa6cffe57c7a0bfd60d25555f70ab0723a2921faab79f115bcd19cd9a2077`。
- 安装包 SHA-256：`59095f01ce53aae060c73b516309bd6fd539a0b5b972cb097dcbe8e93916de48`。
- 升级/回滚须替换完整应用包，禁止单独替换 DLL。回滚前备份用户配置；旧版本可能重新将本地后端迁移为 NET，因此应同时恢复对应旧配置。
- `.codex/` 与 `docs/` 留在本地，不纳入本次代码提交。
