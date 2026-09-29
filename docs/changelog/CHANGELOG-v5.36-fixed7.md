# JTTY-Go v5.36-fixed7

## 更新与修复

### 1. QSO 频率窗口仅显示 TX

- “QSO 频率”列表不再混入 RX 解码记录。
- 只有实际开始发射时产生的 TX 信息才会进入该列表。
- 黄色高亮继续用于标识 TX 记录。
- 点击 QSO 记录时改为设置 TX 频率，避免把 TX 记录误操作为 RX 频率。

### 2. QSO 频率严格按 UTC 从旧到新排序

- 列表以发送事件的 UTC 时间作为主排序键。
- 最新 TX 消息始终追加到列表底部。
- 即使事件到达顺序与实际 UTC 顺序不同，也会重新排列到正确位置。
- 相同 UTC 时间使用发送序号作为稳定的次排序键。

### 3. Hamlib 电台列表显示厂家 + 型号并排序

- Hamlib `rigctl -l` / `rigctld -l` 返回的电台现在显示为“厂家 型号”，例如：`Yaesu FT-817`。
- 使用 Hamlib 输出的固定列解析方式，兼容包含空格的厂家名称，例如 `N2ADR James Ahlstrom Quisk`。
- 列表按照最终显示名称的整体首字母进行升序排序。
- “None” 由 JTTY-Go 单独提供，并始终位于列表最顶端。
- 默认电台仍为 `None` / Model ID 0。

### 4. 电台串口配置向 WSJT-X / Hamlib 语义靠拢

- 启动本地 `rigctld` 时，现在将数据位、停止位、串口握手参数通过 Hamlib `-C/--set-conf` 传递给后端。
- PTT 方法遵循 WSJT-X 的独立配置语义：VOX 不调用 Hamlib PTT；CAT 保持电台后端的 CAT PTT 类型；DTR/RTS 显式选择对应串行 PTT 类型，并支持独立 PTT 串口。
- 保留现有 JTTY-Go 电台设置窗口布局，不改变已有 UI 分组。
- 已取消“选择了 Hamlib 电台就强制把 VOX 改成 CAT”的保存/加载逻辑；选择实体电台后仍可以保持 VOX。

### 5. 启动快捷消息配置继续自动同步

- 保留 fixed6 中的启动 `settings:get` 同步逻辑。
- 每次启动都会从本地配置加载快捷键名称/备注并立即刷新主界面快捷消息按钮，无需再次进入设置保存。

## 验证

- `go test ./internal/radio`：通过。
- `go test ./internal/decode ./internal/jtty ./internal/receiver`：通过。
- 前端 `tsc --noEmit`：通过。
- 当前环境缺少 Wails 依赖缓存，无法执行 `go test ./internal/app` 的完整编译验证；本版已对相关 Go 文件执行 `gofmt`。
