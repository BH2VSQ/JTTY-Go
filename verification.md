# JTTY-Go 5.36.16 验证记录

日期：2026-10-14。基线：`ac82047`。验证环境为 Windows amd64、Go 1.27.2、Wails 2.15.0、TypeScript 5.9、Vite 7.3.7。工具、缓存和临时运行目录位于 H 盘。

## 软件验证

- `go test ./internal/... ./tests/... ./cmd/...` 通过。
- `go test -race ./internal/radio ./internal/app ./internal/model ./internal/jtty ./internal/receiver` 通过，设置 `JTTY_ORACLE` 执行独立 Fortran 对照。
- 上述修改模块的 `go vet`、前端 `npx tsc --noEmit` 和 Vite 构建通过。
- 真实 Hamlib 4.7.3~rc Dummy：自动启动独立回环端口、频率/模式读写、PTT 开关、Split、断开、测试后恢复正式连接、启动失败与进程退出诊断通过。外部 NET 服务器不会被停止。
- 协议异常测试覆盖非法 PTT 回复、超时/迟到响应、连接失步、能力不足和 Split 失败；查询阻塞时可独立停止 PTT，故障不会自动恢复发射。
- 型号查询使用 `-L -u`，在打开设备前退出。真实 FT-817 型号报告 RS-232，正确启用 CAT 串口；真实 TS-480 型号保留原始 `RIGMICDATA` 能力。能力判断读取当前 Value，不能从 Default、可选 Combo 或普通 CAT PTT 推断 Mic/Data 支持。
- Front/Mic、Rear/Data 的协议映射与固定 WSJT-X 参考一致：普通 ON 为 `T 1`，明确支持时 Rear/Data 为 `T 3`，停止为 `T 0`；旧 Mic/Data 配置、能力不足回退和独立配置保存通过。
- 浏览器通过真实 Wails Go 绑定验证：21 次后端切换、11 次型号回复、空闲无事件回送风暴、Dummy CAT/PTT、保存及断开。另用显式配置 Mic/Data 的 Dummy NET 服务器验证能力读取、音频选项启用及数据 PTT 开关；这只验证软件协议。
- 串口实际链路为 Windows `QueryDosDeviceW` → Go `AvailableSerialPorts` → `radio:serial-ports` 回复 → CAT/PTT 两侧列表。设备连接时各层均读到 COM18；后续设备断开时各层均返回空列表并提示未发现串口。没有打开物理 CAT 串口或触发硬件 PTT。
- Echo-aware 浏览器回归覆盖 314 个型号、三种窗口尺寸（1360×820、1094×680、1280×640）、反复切换、分别保存、取消、无默认串口、热拔插保留选择和手动路径。设置正文可滚动，PTT 与连接测试按钮不会被底部按钮栏遮挡。
- 固定 WSJT-X 提交 `9d059511f77e2d1be746ebfeaa7f88e77c1babb7`：10 组消息的位/声调一致，Fortran→Go 与 Go→Fortran 双向解码通过；固定约 -8 dB 噪声与重叠信号样本通过，弱信号 SNR 差异不超过 2 dB。超过 80 字符的分段、最终 EOM、填充过滤及接收完成/超时/结束通过。

## 打包与验证边界

- Windows amd64 Wails、Portable ZIP 与 Inno Setup 安装包构建通过。两种包在干净目录解包，验证完整 Hamlib 文件集的 SHA-256 和 PE 架构，并用解包后的运行文件执行 Dummy 集成测试。
- 包版本为 5.36.16；校验值见本地 `dist/windows/SHA256SUMS.txt`。升级/回滚替换完整应用包，禁止单独更新 DLL。回滚旧版本前备份配置，必要时恢复对应旧配置。
- 未进行真实电台 CAT、RTS/DTR、麦克风/数据音频接口、声卡连续播放或 RF 发射验证。Dummy 与型号元数据通过不表示具体硬件已验证。
- Wails 开发模式自带 `/wails/ipc.js` 调试面板存在 Svelte `nodes` 异常及 `runtime:ready` 提示，独立记录；上述业务绑定与交互通过。原生发布版 WebView2 全界面自动化未完成，安装/卸载未执行。
- 全仓库 `go vet` 原有两处 Windows 音频 `unsafe.Pointer` 警告仍存在；修改模块的检查通过。未将测试结果换算为用户电脑的 CPU 百分比。
