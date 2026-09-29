# JTTY-Go Windows 初步发行包

本目录是在 `v5.36-fixed14` 源码基础上加入 Windows 发行打包流程后的版本。

## 重要

当前上传的源码包 `bin\` 只有占位文件，没有真实 Hamlib Windows runtime，因此**这里不能直接生成包含 Hamlib 的最终 Setup.exe**。在 Windows 打包机上先把完整 Hamlib `bin` 内容放进去，再执行打包脚本即可。

项目当前 `main.go` 使用：

```go
//go:embed all:frontend/dist all:bin
```

因此 Wails 生产构建会把 `bin` 内容嵌入 `JTTY-Go.exe`。程序第一次运行时会自动把嵌入的 Hamlib runtime 释放到 EXE 所在目录。

## 推荐发行物

### 单文件便携 EXE

`build\bin\JTTY-Go.exe`

程序本身包含前端和 Hamlib runtime。复制该 EXE 到一个可写目录，首次启动后会自动释放 Hamlib 文件到 EXE 所在目录。

### 单 EXE 安装程序

`JTTY-Go-5.36.14-Setup.exe`

使用 Inno Setup 6 生成。安装程序本身是一个 EXE，默认安装到：

```text
%LOCALAPPDATA%\Programs\JTTY-Go
```

这样无需管理员权限，同时保证应用首次运行释放 Hamlib 时具有目录写权限。

配置、日志、ADIF、录音等用户数据仍在：

```text
%APPDATA%\JTTY-Go
```

卸载程序不会删除这些用户数据。

## 构建命令

```powershell
.\scripts\package-windows.ps1
```

只生成便携包：

```powershell
.\scripts\package-windows.ps1 -SkipInstaller
```

跳过测试：

```powershell
.\scripts\package-windows.ps1 -SkipTests
```

清理后重新构建：

```powershell
.\scripts\package-windows.ps1 -Clean
```

## 必需环境

- Go 1.23+
- Node.js / npm
- Wails CLI v2.15.0
- Inno Setup 6（仅生成 Setup.exe 时需要）
- 完整 Hamlib Windows runtime

Hamlib 至少应包含：

```text
bin\rigctld.exe
bin\rigctl.exe
bin\*.dll
```

以及这些 DLL 所依赖的其它运行时文件。打包脚本会检查 `rigctld.exe`、`rigctl.exe` 和 DLL，不完整时直接停止。
