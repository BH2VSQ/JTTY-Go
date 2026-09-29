# v5.36-fixed22

- 修复 Windows 打包脚本检测不到 Inno Setup 6 的问题。
- `package-windows.ps1` 支持通过 `-InnoSetupPath` 显式指定 `ISCC.exe`。
- 扩展 `ISCC.exe` 自动检测范围：PATH、Program Files、LocalAppData、Scoop、Chocolatey 以及 Windows 卸载注册表中的自定义安装目录。
- 未找到 `ISCC.exe` 时输出实际检查过的路径，便于定位环境问题。
