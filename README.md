# JTTY-Go

JTTY-Go 是一款面向 Windows 的业余无线电 JTTY 数字通信桌面软件，使用 **Go + Wails + TypeScript/Vite** 构建。项目提供实时音频接收、JTTY 解码、瀑布图、发送、快捷消息与 QSO 操作，并参考 WSJT-X 的工作方式与界面设计。

## 开发环境

- 操作系统：Windows 10/11 64-bit
- Go：1.23 或更高版本
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
- `github.com/go-ole/go-ole` v1.2.6

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
- JTTY-Go 使用的 Hamlib 支持目前处于 TODO 阶段，相关设备配置页已暂时禁用，但代码保留用于后续开发

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

需要将完整的 Hamlib Windows runtime 放入：

```text
bin/
```

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

- [ ] 完善并重新启用 Hamlib 电台支持与设备配置页
- [ ] 完善 CAT / PTT / Split / Mode 等电台控制能力与设备能力判定
- [ ] 持续优化长时间运行时的实时解码性能与资源占用
