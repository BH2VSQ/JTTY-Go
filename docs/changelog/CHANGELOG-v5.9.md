# JTTY-Go v5.9

## UI
- 顶部主菜单收敛为“文件 / 配置”。
- 主界面彻底移除音频设备下拉框；输入/输出设备仅在设置窗口配置。
- 主界面继续采用 WSJT-X 风格的紧凑双栏布局。
- 设置窗口合并为 6 个页面：常规、电台、音频、JTTY、快捷消息、日志。
- 删除独立“显示”和“自动化”页，显示选项并入常规/JTTY，自动 QSO 规则并入日志。
- 主窗口隐藏可见滚动条，但保留列表滚轮操作。

## Waterfall
- 左键设置 RX 频率。
- 右键设置 TX 频率。
- 使用固定频谱坐标范围渲染 RX/TX 标线，避免切换 RX/TX 后因帧坐标变化导致残留/闪烁。
- 每次绘制显式 clearRect + 重建标线层。
- 频率变更使用 requestAnimationFrame 合并刷新。

## Audio
- 设置页采用 WSJT-X 风格 Soundcard 布局：Input / Output / Channel / Refresh。
- 主界面不再显示设备选择。

## Radio / Hamlib
- 新增 RadioSettings。
- 新增 Hamlib/rigctld TCP 控制后端：频率、模式、PTT、Split TX frequency。
- 设置页增加 CAT 串口参数、PTT 方法、Split、TX Audio Source、轮询间隔。
- 增加 Test CAT / Test PTT / Connect / Disconnect。
- Windows Hamlib DLL 更新采用 WSJT-X 风格的下载 → 旧 DLL 备份 → 新 DLL 替换 → 可恢复流程；默认使用 Hamlib 4.7 snapshot DLL URL。
