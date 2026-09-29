# JTTY-Go v5.23

## 启动与实时性能

- 窗口首次绘制后再枚举音频设备、启动监听与连接电台，避免启动阶段争用 WebView/UI 线程。
- 默认实时解码线程数为 1；仍可从“解码”菜单显式提高。
- 实时频谱最多向 JTTY 解码器提供 4 个中心候选，限制强噪声环境下的重型并发解码任务。
- 接收处理队列积压时优先丢弃旧帧，避免实时显示持续落后于音频。
- 频谱检测复用 Hann 窗与 FFT 工作区。
- JTTY 频移混频改为复数递推旋转，减少逐采样 sin/cos 调用。
- 同步波形和标准 192 samples/symbol 参考波形采用惰性缓存。
- Waterfall 前端复用 ImageData，移除每行频谱临时 canvas/context 的创建。

## 频率范围

- 接收起始频率固定为 0 Hz。
- 顶部“频率范围”菜单可选 2500 / 2700 / 3000 / 3500 / 4000 Hz 截止频率。
- 选择后自动保存并关闭菜单。

## 功能对接

继续沿用既有 UI 与模块边界，仅填充 Audio / Detector / Decoder / Receiver / Transceiver / Logbook 等运行链路，不改变既有界面结构。WSJT-X 官方项目当前源码也按 Audio、Decoder、Detector、Modulator、Transceiver、logbook 等职责组织。
