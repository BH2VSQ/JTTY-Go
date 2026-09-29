# JTTY-Go v5.10

- 保留 v5.9 的 WSJT-X 风格主界面：仅“文件 / 配置”两个顶级菜单。
- 保留瀑布图左键 RX、右键 TX，并使用固定频率坐标 + clean redraw，避免 TX 标线残留闪烁。
- 主界面完全移除声卡设备选择；声卡配置集中在“配置 → 音频”。
- 配置窗口精简为常规 / 电台 / 音频 / JTTY / 快捷消息 / 日志六页。
- 电台页继续按 WSJT-X 的 CAT Control、Serial Port Parameters、PTT Method、Split、TX audio source、polling、Update Hamlib 组织。
- Hamlib/rigctld：可使用指定 rigctld.exe 自动启动本地 rigctld，并使用 Model ID、串口和波特率建立连接。
- Hamlib Update / Revert 的目标目录优先使用 rigctld.exe 所在目录，否则使用 JTTY-Go.exe 所在目录。
