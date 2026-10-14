# JTTY-Go

JTTY-Go 是一款面向 Windows 的业余无线电 JTTY 数字通信桌面软件，使用 **Go + Wails + TypeScript/Vite** 构建。项目提供实时音频接收、JTTY 解码、瀑布图、发送、快捷消息与 QSO 操作，并参考 WSJT-X 的工作方式与界面设计。

## 开发环境

- 操作系统：Windows 10/11 64-bit
- Go：1.25 或更高版本
- Node.js：22.x 或更高版本
- npm：10.x 或更高版本
- Wails CLI：v2.15.0
- 前端构建：Vite 7.x、TypeScript 5.x
- 编译架构：Windows amd64

## 软件依赖

### Go

项目使用 Go Modules，主要依赖：

- `github.com/wailsapp/wails/v2` v2.15.0
- `github.com/degubites/go-wca` v0.4.1
- `github.com/go-ole/go-ole` v1.3.0

首次获取或更新依赖时执行：

```powershell
go mod tidy
go mod download
```

### 前端

前端依赖由 `frontend/package.json` 管理：

- Vite
- TypeScript

安装依赖：

```powershell
cd frontend
npm install
```

### Windows 运行环境

- Wails Windows 构建所需的 WebView2 环境
- Windows 音频设备 / WASAPI
- 本地 Hamlib：选择电台型号、设备路径及通信参数后，应用自动启动随包的 Windows amd64 `rigctld`，使用独立回环端口。
- NET rigctl：连接已有服务器，应用不启动、更新或停止该服务器。
- 两种后端分别保存连接及 PTT 配置，同一时刻使用一种。关闭电台控制仍可使用独立 VOX/串口 PTT。
- 本地串口型号在左侧选择 CAT 串口；刷新与 RTS/DTR 使用同一本机串口列表，也可手动输入设备路径。不自动选择第一个串口，刷新保留已有选择。
- CAT PTT 音频输入与 WSJT-X 一致：Front/Mic 使用普通 PTT，Rear/Data 仅在型号明确支持 Mic/Data PTT 时使用数据 PTT。普通 CAT PTT、RTS/DTR 和 VOX 不代表支持音频接口选择；NET 先测试连接获取服务器能力。旧 Mic/Data 设置分别迁移为 Front/Rear。

### Windows 安装程序打包依赖

- Inno Setup 6：仅用于生成 Windows 安装程序，不影响 Portable 便携版构建
- `ISCC.exe`：Inno Setup 6 自带的命令行编译器，打包脚本会自动查找

## 编译方法

### 直接编译

在项目根目录执行：

```powershell
go mod tidy
go test ./internal/... ./tests/...
cd frontend
npm install
npm run build
cd ..
wails build -platform windows/amd64 -webview2 embed -trimpath
```

生成的程序位于：

```text
build/bin/JTTY-Go.exe
```

也可以使用项目提供的 Windows 构建脚本：

```powershell
.\scripts\build-windows.ps1
```

### Windows 发行版打包

发行包必须携带 `hamlib/` 中的匹配运行文件和 `runtime-manifest.json`。打包时校验 SHA-256 与 PE amd64 架构；程序启动本地 Hamlib 前再次验证。NET rigctl 的默认端口仍为 4532。单 DLL 在线替换已禁用，升级或回滚请替换完整应用包。

生成 Portable 便携版：

```powershell
.\scripts\package-windows.ps1 -Clean
```

安装 Inno Setup 6 后再次执行同一命令，可同时生成 Windows 安装程序。打包脚本会自动调用 Inno Setup 的 `ISCC.exe` 编译 `installer/` 中的安装脚本。

输出目录：

```text
dist/windows/
├─ JTTY-Go-portable-windows.zip
└─ installer/
   └─ JTTY-Go-*-Setup.exe
```

如果未检测到 Inno Setup 6，脚本仍会生成 Portable 便携版，但会跳过安装程序生成。

## 文件结构

```text
JTTY-Go/
├─ bin/                    # Windows Hamlib runtime
├─ cmd/                    # 辅助命令行工具
├─ docs/                   # 架构、构建与参考文档
│  └─ changelog/           # 版本变更记录
├─ frontend/               # Wails 前端界面
│  └─ src/
├─ installer/              # Windows 安装程序脚本
├─ internal/               # Go 核心模块
│  ├─ app/                 # 应用层与 Wails 接口
│  ├─ audio/               # 音频设备与采集
│  ├─ decode/              # 解码调度
│  ├─ detector/            # 信号检测
│  ├─ dsp/                 # DSP / FFT
│  ├─ jtty/                # JTTY 协议与解码
│  ├─ logbook/             # QSO / 日志
│  ├─ macro/               # 快捷消息
│  ├─ model/               # 数据模型与设置
│  ├─ radio/               # 电台 / Hamlib 接口
│  ├─ receiver/            # 实时接收流水线
│  ├─ tx/                  # 发射
│  └─ waterfall/           # 瀑布图
├─ licenses/               # 第三方许可证与声明
├─ reference/              # WSJT-X / JTTY 参考资料
├─ scripts/                # Windows 开发、测试与打包脚本
├─ tests/                  # 集成测试
├─ go.mod                  # Go 模块定义
├─ main.go                 # 程序入口
└─ wails.json              # Wails 配置
```

## TODO

- [x] 本地 Hamlib 与 NET rigctl 共存，独立配置及受控进程生命周期
- [x] 独立 WSJT-X Fortran 双向互通、弱信号及重叠信号软件验证
- [ ] 持续完善不同 Hamlib 型号的 Split / Mode / PTT 能力判定与实机兼容性验证
- [ ] 持续优化长时间运行时的实时解码性能与资源占用

## 电台配置

在“配置 → 电台”选择“本地 Hamlib”或“NET rigctl”。本地模式通过型号搜索选择电台，按其能力填写 COM 端口、设备路径或电台网络地址；没有自动选择第一个串口。Dummy（型号 1）无需设备路径，可用于软件检查。

本地 RTS/DTR 由 Hamlib 执行；PTT 串口留空表示使用 CAT 设备，避免应用重复打开同一端口。NET 模式的 RTS/DTR 使用本机独立串口，必须明确选择。麦克风/数据 PTT 仅在电台明确支持时可用。连接测试读取状态，不启用发射；PTT 测试最多持续 30 秒。

协议故障后不自动恢复发射。先检查电台是否停止发射，再重新连接。Split 使用电台报告的 VFO 和能力；未实现的 Fake It 不可选。

## 独立 JTTY 互通检查

运行时的 Go PHY 以 WSJT-X 提交 `9d059511f77e2d1be746ebfeaa7f88e77c1babb7` 为参考。普通测试包含独立生成的编码、声调和 PCM 样本。完整双向检查需要在 H 盘准备该提交的官方源码、gfortran 和 FFTW：

```powershell
.\scripts\build-jtty-oracle.ps1 -SourceDir H:\tools\jtty-go\oracle-source -BuildDir H:\tools\jtty-go\oracle-build
$env:JTTY_ORACLE = 'H:\tools\jtty-go\oracle-build\jtty-oracle.exe'
go test ./internal/jtty -run TestIndependent -v
```

测试覆盖载荷位与声调一致性、WSJT-X/Go 双向解码、弱信号 SNR、重叠信号及长消息组装。独立测试工具不进入发行包。设备/RF/PTT 实机验证仍需使用实际电台完成。
