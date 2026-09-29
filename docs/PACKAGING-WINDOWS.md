# JTTY-Go Windows 打包说明

## 两种发行方式

JTTY-Go 当前已经使用 Go `embed.FS` 将 `frontend/dist` 和 `bin` 编译进主程序。生产构建后，程序首次运行会把嵌入的 Hamlib runtime 文件释放到 `JTTY-Go.exe` 所在目录；已有文件不会被覆盖。

因此可以同时提供：

1. **单文件便携版 `JTTY-Go.exe`**
   - 主程序包含前端资源、Hamlib runtime 和 WebView2 bootstrapper。
   - 首次启动自动释放 Hamlib 文件到 EXE 同目录。
   - 适合直接复制到 U 盘/其他电脑测试。

2. **单 EXE 安装程序 `JTTY-Go-*-Setup.exe`**
   - 使用 Inno Setup 创建。
   - 安装包本身只有一个 EXE。
   - 安装时直接把 Hamlib `bin` 中全部运行时文件写入安装目录，避免 Program Files 首次运行时的写权限问题。
   - 默认安装到 `%LOCALAPPDATA%\Programs\JTTY-Go`，不要求管理员权限。
   - `%APPDATA%\JTTY-Go` 中的配置、ALL.txt、JTTY.adi、record 等用户数据不随卸载删除。

## Hamlib 文件要求

将完整的 Windows Hamlib runtime 放入项目：

```text
bin\
  rigctld.exe
  rigctl.exe
  libhamlib-4.dll
  ...其它 DLL / 运行时文件...
```

不要只复制 `libhamlib-4.dll`。`rigctld.exe` 所依赖的 DLL 也必须一并放入 `bin`。

打包脚本会拒绝以下不完整情况：

- 缺少 `rigctld.exe`
- 缺少 `rigctl.exe`
- `bin` 中没有 DLL
- runtime 文件数量明显不足

## 构建

在 Windows 开发环境执行：

```powershell
cd C:\path\to\jtty-go
.\scripts\package-windows.ps1
```

只构建便携版：

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

脚本需要：

- Go
- Node.js + npm
- 项目使用的 Wails v2 CLI
- Inno Setup 6（仅在需要生成安装程序时）

## WebView2

Wails Windows 程序依赖 Microsoft WebView2。当前打包脚本使用：

```text
-webview2 embed
```

因此在检测不到合适的 WebView2 runtime 时，程序可以使用内嵌的官方 bootstrapper 进行安装，而不是强制要求用户事先手动安装。

## 输出目录

```text
dist\windows\
  JTTY-Go-portable\
    JTTY-Go.exe
    bin\...
    licenses\...
  JTTY-Go-portable-windows.zip
  installer\
    JTTY-Go-5.36.14-Setup.exe
  SHA256SUMS.txt
```

## 发布注意事项

安装程序是一个 EXE，但 Hamlib 仍然以普通 DLL/EXE 文件的形式安装到 `JTTY-Go` 安装目录。这样便于 Hamlib 更新、故障排查和后续更换版本。

对于单文件便携版，Hamlib 是嵌入在 JTTY-Go.exe 中的；运行时才释放到 EXE 同目录。

## Hamlib helper process behavior

The Windows build launches bundled `rigctl.exe`/`rigctld.exe` as hidden child processes using `HideWindow` and `CREATE_NO_WINDOW`. This is required because the Hamlib utilities are console-subsystem executables while JTTY-Go is a GUI application.

The radio settings page caches the Hamlib model list, per-model capabilities and runtime status. Reopening the page therefore does not repeatedly start Hamlib helper processes. In-flight probes are cancellable when the settings page is closed.
