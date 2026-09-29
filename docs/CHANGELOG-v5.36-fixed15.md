# v5.36-fixed15

本版本主要修复 Windows 发行版进入电台配置页面时频繁闪出命令提示符窗口，以及 Hamlib 探测请求集中启动多个子进程造成的资源占用问题。

### 处理方式

1. `rigctl.exe`、`rigctld.exe` 与 `taskkill.exe` 均使用 Windows `HideWindow + CREATE_NO_WINDOW` 启动。
2. 长驻 `rigctld.exe` 不再继承 GUI 的标准输出/错误输出。
3. Hamlib 列表、能力、运行时状态探测增加进程串行化。
4. Hamlib 型号、能力和运行时状态增加进程生命周期缓存，重复进入配置页不再重复启动探测程序。
5. 前端抑制同一型号的重复 capability 请求。
6. 关闭配置页时取消正在执行的 Hamlib 探测；探测过程增加超时。
7. 更新/还原 Hamlib 后自动清除缓存。

这些修改不改变现有设置页布局，也不改变 CAT/PTT 的功能判定规则。
