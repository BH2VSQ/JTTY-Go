# JTTY-Go v5.21 功能对接说明

本阶段遵循“**不改现有设计结构，只填充功能对接**”原则。现有 Wails 页面、菜单层级、设置页组织和数据对象保持不变，仅把已经存在的模块边界连接到真实运行时。

参考对象为 WSJT-X 工程的模块化职责划分：Audio、Detector/Decoder、Transceiver、Logbook 等职责分离；JTTY 自身的同步、峰值搜索、相关、TBCC/WAVA、source codec、波形生成在 Go 侧保持独立。

## 1. RX 音频 → 实时解码

`internal/audio` 提供 Windows WASAPI 输入。捕获线程只负责获取 PCM，不等待 GUI 或 FEC。

运行链路：

```text
WASAPI Capture
  -> PCMFrame
  -> 48 kHz 统一采样率
  -> JTTYStream / Pipeline
  -> FFT + Noise Floor
  -> Candidate + Tracker
  -> JTTY DecodeManyDetailed
  -> MessageAssembler
  -> DecodeStore
  -> Wails Event
```

`JTTYStream` 保留 3 个 retro quarter-frame 的历史窗口，并允许“解码”菜单/主界面的手动立即解码请求进入同一个解码队列。

## 2. 频谱搜索

频谱搜索属于内部固定算法参数，不通过用户设置暴露。

编译期参数位于 `internal/detector/search_config.go`：

- 搜索范围：300–2700 Hz
- FFT：4096
- Hop：1024
- Pipeline 工作率：48 kHz

改变这些值需要重新编译，但不改变任何 UI 结构。

## 3. JTTY PHY

发送侧：

```text
文本
 -> PackMessage
 -> 34-bit payload
 -> EncodePayload / EncodeFrame
 -> 59 个四音调 symbol
 -> GenerateJTTYWaveform
 -> 12 kHz PCM
 -> WASAPI Renderer
```

接收侧使用对应的同步搜索、peak-up、payload correlation、TBCC/WAVA + CRC、source-codec 验证及解码后信号消除。

多信号接收由 `DecodeManyDetailed` 按主频道、侧频道和 subtraction 顺序处理，避免仅按全局功率排序造成解码顺序漂移。

## 4. TX 与 PTT

`tx:send`、宏和全局快捷键最终统一进入 `transmitJTTY`。

实际事务顺序：

```text
PackMessage
 -> 打开音频输出
 -> PTT ON
 -> NotifyTXStarted
 -> 播放全部 JTTY frames
 -> 等待输出缓冲耗尽
 -> PTT OFF
 -> NotifyTXFinished
```

支持：

- CAT PTT
- VOX（不额外发送 CAT PTT）
- DTR PTT
- RTS PTT
- Force DTR / Force RTS
- 独立停止 TX
- Tune 音调

`StopTransmit` 与 TX generation ID 联合使用，避免旧 TX 协程在新 TX 已开始后错误释放新 TX 的 PTT。

## 5. 电台对接

当前运行时后端为 `rigctld`：

- 自动连接/重连
- 获取频率
- 获取模式
- 设置频率
- 设置模式/带宽
- CAT PTT
- Rig split / VFOB split frequency
- 周期轮询
- 本机 localhost/127.0.0.1 下自动启动 `rigctld`
- Hamlib 更新/回滚保留现有界面入口

主频率编辑、RX/TX 音频偏移与电台 RF 频率分开处理，避免把 1500 Hz 之类的音频偏移误写入 RF 频率。

## 6. 音频录音

监听启动且 `recordEnabled=true` 时，录音器自动启用。

录音文件按 UTC 日期命名：

```text
<日志目录>/record/YYYYMMDD.wav
```

UTC 00:00 自动切换到新文件。录音在音频接收线程中直接写入，不经过 GUI。

## 7. ALL 解码日志

`DecodeLogger` 在解码被 `DecodeStore` 接受后追加记录，支持：

- `ALL.txt`
- `ALL-YYYY.txt`
- `ALL-YYYY-MM.txt`

重复/更新消息不会重复追加。

## 8. 手动 QSO

QSO 不再由 TX 完成事件自动建立/写入。

当前逻辑：

```text
DX 呼号 = JA1ABC
        |
第一次实际发送内容中包含 JA1ABC
        |
记录 StartUTC
        |
点击“记录通联”
        |
立即捕获 EndUTC
        |
确认窗口允许修改 RST/网格/姓名/功率/交换/备注
        |
JTTY.adi
```

默认报告：`599 / 599`。

`NotifyTXFinished` 只发送运行事件，不写 ADIF。

## 9. 设置实时生效

解码菜单选择一个参数后：

```text
修改
 -> 写入 settings
 -> SaveSettings
 -> 应用到运行中的 Pipeline
 -> 菜单自动关闭
```

当前阈值、频率容差、tracker 容差和 TTL 可以直接更新运行实例；只有线程数/解码模式改变时才需要重新建立 receiver worker。

## 10. 并发与生命周期

已补齐以下运行时生命周期：

- receiver start/stop 防并发启动
- audio recorder start/stop
- radio polling start/stop
- rigctld 子进程失败时清理
- TX generation 防旧任务干扰新任务
- receiver pipeline / stream 访问使用 receiver mutex
- 应用关闭时按 receiver → TX → recorder → hotkey/PTT → radio 顺序释放

## 11. 当前设计边界

本阶段没有新增页面、没有改变现有菜单层级、没有改变用户已经确认的布局结构。新增代码只用于把现有界面的操作连接到具体模块。

后续实现可以继续在同一边界内逐项补齐 WSJT-X 风格的运行细节，而无需重新设计 UI。
