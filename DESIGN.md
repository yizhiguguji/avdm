# Design

## Source of truth
- Status: Current implementation rejected in visual review; redesign proposal below is the next design baseline, pending application implementation.
- Last refreshed: 2026-10-07
- Primary product surfaces: 安卓设备矩阵 single-window desktop workbench, CLI.
- Evidence reviewed: `cmd/adm-gui/main.go`, `README.md`, `docs/USER_GUIDE.md`, BrowserStack multi-device testing docs, Android Studio Device Manager docs, Genymotion SaaS UI docs, local UX review notes from this session.

## Brand
- Personality: Modern operations console, technical, dense, controlled.
- Trust signals: Clear device state, predictable actions, visible task results, readable diagnostics, no misleading controls.
- Avoid: Old desktop form layouts, default control dumping, large red danger blocks, excessive white space, unstable window-management promises.

## Product goals
- Goals: Manage Android emulators/devices, install APKs, run common device actions, monitor multiple devices, inspect failures.
- Non-goals: Become Android Studio, hide every technical detail, promise stable cross-process emulator window embedding.
- Success signals: Users can identify usable devices, choose a target, run common actions, and understand failures without reading logs first.

## Personas and jobs
- Primary personas: Android app operators, QA testers, developers managing multiple AVDs.
- User jobs: Start AVDs, choose a target, install APKs, control devices, inspect screenshots, diagnose adb/emulator/tool failures.
- Key contexts of use: Repeated operational work, multiple devices, occasional flaky adb/emulator states.

## Information architecture
- Primary navigation: Single-window device workbench.
- Core routes/screens: Device wall workbench, create AVD dialog, batch start/delete dialogs, package uninstall dialog.
- Content hierarchy: Device wall first, current target/selection second, task actions third, logs/diagnostics last.

## Design principles
- Device-first: A device card/list row must show name, state, serial/product, and available next action.
- Stable actions: Remove or demote actions that repeatedly fail in normal use.
- Progressive disclosure: Show common actions inline; move rare, advanced, or dangerous actions to secondary groups.
- Single workbench: Device management, device wall, and task actions share one window; avoid a second primary Control Center window.
- Tradeoffs: Dense operational UI is preferred over decorative whitespace; clarity is preferred over clever labels.

## Visual language
- Color: Light neutral gray app background and white surfaces; blue for primary actions and focus, green for healthy state, red for destructive actions. Previews retain a dark background.
- Typography: Compact hierarchy; bold device names, muted metadata, badge-like states; no oversized headings inside tool surfaces.
- Spacing/layout rhythm: Tight but readable, with consistent row heights, compact toolbars, and no empty placeholder columns.
- Shape/radius/elevation: 6-8px radius, low contrast borders, no nested card stacks.
- Motion: No decorative motion.
- Imagery/iconography: Use icons only when a Fyne/theme icon clearly improves scanability; otherwise concise text is acceptable.

## Components
- Existing components to reuse: Fyne `widget.List`, `widget.Button`, `widget.Select`, `widget.Card`, `container.Border`, split containers.
- New/changed components: Light neutral theme, device row, device wall card, black preview pane, status badge, compact action toolbar, collapsible log/task panel.
- Variants and states: available, current, offline, unauthorized, running-but-not-ready, stopped, busy, error.
- Token/component ownership: Device wall/session lifecycle in `device_wall.go` and `wall_state.go`; toolbar and selection planning in `workbench_toolbar.go`; application tools in `application_panels.go`; task results in `batch_tasks.go`.

## Accessibility
- Target standard: Keyboard usable for primary actions; readable contrast in default macOS appearance.
- Keyboard/focus behavior: Primary controls must remain reachable by normal Fyne focus traversal.
- Contrast/readability: Secondary metadata cannot be too light to read.
- Screen-reader semantics: Prefer meaningful labels over unlabeled symbolic controls.
- Reduced motion and sensory considerations: No flashing or animation-heavy surfaces.

## Responsive behavior
- Supported breakpoints/devices: macOS desktop windows from 1280x820 upward; optimize 1440x900 and larger.
- Layout adaptations: One top row holds scan/selection/common actions. Secondary actions move into More when width is limited. No left action rail. Right text buttons open application tools or act on the main target; logs stay collapsible below. Stopped and abnormal entries use compact rows, with previews reserved for online devices.
- Touch/hover differences: Desktop mouse/keyboard only.

## Interaction states
- Loading: Show per-action busy text and keep the rest of the UI usable.
- Empty: Show next available action, not only "none".
- Error: Explain the failed tool/action and the next repair step.
- Success: Update affected device/card state and log one concise task result.
- Disabled: Prefer hidden or disabled controls over buttons that open known-bad flows.
- Offline/slow network: Mark devices as offline/unauthorized/running-not-ready with direct remediation.

## Content voice
- Tone: Direct, short, operational Chinese.
- Terminology: Use "主目标" instead of "当前"; use "重新扫描" instead of vague "刷新"; use "截图" only for non-stream previews.
- Microcopy rules: Avoid promising "实时" unless the action truly opens a low-latency stream; mark advanced/unstable actions as advanced.

## Implementation constraints
- Framework/styling system: Go + Fyne; avoid large framework rewrites in this repo.
- Design-token constraints: Fyne theming is limited; use the 安卓设备矩阵 light neutral theme and repo-local surface helpers before adding custom drawing.
- Performance constraints: Avoid global adb screenshot storms; keep refresh scoped and paced; do not discard existing preview frames while a device list refresh is in progress.
- Compatibility constraints: macOS Accessibility-dependent window movement is not a primary flow.
- Test/screenshot expectations: Run `go test ./...`, `make build`, launch app, and screenshot major surfaces after visual changes.

## Open questions
- [ ] Should scrcpy be restored as a primary manual-control flow for both emulator and physical devices? Owner: product. Impact: device-card and context actions.
- [ ] What minimum number of devices should the workbench optimize for on a 1440x900 window? Owner: product. Impact: card dimensions.

## 2026-10-07 团队实施计划与交付范围

采用分阶段重构，保留 Go/Fyne 和现有设备后端。

| 阶段 | 负责模块 | 落地行为 | 验证 |
| --- | --- | --- | --- |
| 目标安全 | 应用与交互 | 操作提交时固定 serial 与选项；包列表绑定目标及请求代次；同设备写操作互斥，跨设备至多 4 并发 | fake ADB 目标切换、确认及并发测试 |
| 布局与导航 | UI | 单行自适应顶部操作；搜索、状态筛选、密度；未启动设备紧凑行；右侧标准文本按钮 | 窄宽溢出恢复、筛选与动作计划测试；本机视觉验收 |
| 预览生命周期 | 架构 | 筛选、选择与密度变化复用连接；后台建连 2 并发、截图 3 并发；过期结果与关闭保护 | 生命周期、失效回调、并发、停帧及密度边界测试 |
| 反馈与恢复 | 集成 | 批量启动/关闭/删除逐台结果与仅失败重试；辅助功能配置入口及继续排列；缺少系统镜像时提供下载入口 | 批量结果测试；权限入口和对话框验收 |
| 交付 | 集成 | 完整测试、签名编译、备份替换本机应用、中文提交推送 | race、vet、签名和产物一致性检查 |

操作范围：明确勾选项不合格时不得扩大为全部在线设备。仅外部窗支持未勾选时默认全部在线；启动、关闭、删除和主目标设置要求明确勾选。主目标操作与勾选批量操作分别提示目标。

权限恢复：用户在 macOS 设置手动授权；应用重新检测当前进程授权后，只继续排列原目标窗口。系统要求重启应用时保留授权指引。应用不会自行修改 TCC。

预览语义：实时画面标示延迟；截图标示更新时间；超过 30 秒标示画面过期；实时流停帧 5 秒后退回截图。隐藏或筛选设备不重复建立连接，消失或离线设备释放连接。

交付限制：自动化不执行真机关机、卸载、删除 AVD 或大容量下载。真实设备控制和跨进程窗口排列最终受设备状态及 macOS 授权影响，不能仅由单元测试保证。

### 验收结果（2026-10-07）

- `go test -race ./...`、`go vet ./...`、`git diff --check` 通过。
- `make build APP_ID=com.local.adm` 成功；Fyne preferences ID 与已有签名 bundle ID 一致。
- 本机 1280px 工作台验收：右侧安装面板展开时顶部仍单行；名称搜索与清空正常；6 个未启动 AVD 在一屏以列表显示；明确勾选未启动 AVD 后打开外部窗只提示该项未在线，没有扩大目标。
- 「打开系统设置」已确认进入「隐私与安全性 → 辅助功能」；当前已存在的授权被新进程成功识别，显示「辅助功能权限已开启」。验收未改动权限开关。
- 签名验证通过；已备份并替换 `/Applications/安卓设备矩阵.app`，安装后二进制及 plist 与构建产物 SHA-256 一致。
- 真实设备危险操作和系统镜像下载未执行；预览后台操作已验收，批量写入由 fake ADB 与行为测试覆盖。

### 布局纠偏与日志选择（2026-10-07）

根据大窗口截图验收重新调整：在线设备墙根据可用宽高和设备数量放大并居中，每行居中排列；消除卡片底部固定占位。未启动、启动中及异常设备进入独立设备库，名称与正常高度的操作按钮相邻，单独滚动；设备库可收起，窄宽度通过弹窗打开，避免与在线预览共用纵向滚动。无在线设备时集中显示设备库。顶部工具状态改为简短可读的就绪或待处理数量。

日志采用可选择的只读文本，支持鼠标拖选、全选、快捷键复制、右键复制，以及「复制选中」「复制全部」。选择时暂停显示更新，后台日志继续收集；点击「跟随最新」恢复显示和滚动，防止新日志覆盖选区。

验证：布局几何、设备库分区、预览复用、日志只读/复制/选区快照测试通过；完整 race 和 vet 通过。已在本机常规窗口、大窗口、安装面板展开及日志展开状态验收；日志全选复制和拒绝键入已实测。新版已签名编译并备份替换。

### 视觉重整与本机自查（2026-10-07）

统一浅灰底、白色面板、深色正文与灰色辅助文字，移除深蓝大块背景和卡片双层粗边框。普通工具按钮使用低强调样式，蓝色用于主要执行操作与焦点；右侧工具配文字和图标。卡片保留单层细边框，截图时间和工具状态减弱到辅助层级。

设备库宽度收至 300px，条目采用分隔线与短状态，管理和启动按钮保持相邻。大窗口工作区最大宽度 1600px，预览高度上限 760px；按窗口、日志和工具面板实际占用空间调整，避免手机无限放大。

本机已检查常规窗口、大窗口、视图对话框、安装面板和日志展开。日志拖选的蓝色选区、暂停更新提示与快捷键复制正常。完整 race、vet、编译及签名验证通过，新版已备份替换本机应用。

## 2026-10-07 设计师调研与新基线（提案，尚未实现到 App）

用户否定当前正常和最大化窗口的整体布局与配色。此前编译、测试和本机截图检查只证明功能运行，不能代表视觉验收通过。本节覆盖此前的视觉方向；上文交付记录保留作为历史。

两位设计师分别进行了竞品调研和独立视觉审查。共同结论：布局层级、动作归属和尺度是主要问题，单纯更换深浅主题没有解决它们。

### 调研依据

| 官方参考 | 观察 | 本项目采用的设计推导 |
| --- | --- | --- |
| [Android Studio Device Manager](https://developer.android.com/studio/run/managing-avds)，官方设备列表截图 | 管理用规则列表；启动与更多围绕对应设备 | 完整设备库改为表格，列对齐，操作归属设备行 |
| [Genymotion Desktop](https://docs.genymotion.com/quickstart/desktop_install/)，官方 launcher 截图 | 搜索、创建与设备表分层 | 搜索可见，全局工具集中，管理信息不拆成大量卡片 |
| [BrowserStack 多设备测试](https://www.browserstack.com/docs/live/multi-device-testing/single-tab)，官方多设备截图 | 多设备画面有规则区域，操作范围区分单设备与全部设备 | 主目标与批量选择分别显示，执行前固定目标范围 |
| [Apple 工具栏指南](https://developer.apple.com/design/human-interface-guidelines/toolbars) | 工具栏优先常用动作，逻辑分组，减少控件背景干扰 | 单行主工具栏，常见动作可见，低频和危险动作进入菜单 |

色板、尺寸和双视图结构是本项目的设计提案，不是上述产品规定。

### 现状失败原因

- 标题状态区、全局工具栏、在线标题、卡片头尾、设备库和右轨同时常驻，层级碎片化。
- 全局刷新、单台导航、主目标安装、批量启动和危险关闭混在不同位置，重复且缺乏明确归属。
- 少数设备随窗口膨胀为海报；最大化时 1600px 居中上限又形成漂浮白色内容岛。
- 300px 常驻库只列未启动项，既占监看空间又不是完整管理界面。
- 所有按钮一律低强调，只去掉边框而没有建立操作主次。11px 辅助文字也过小。

### 推荐骨架

保留原生标题栏，其下只有一条 48px 主工具栏：设备墙 / 设备库切换、扫描、新建、搜索、主目标设备操作、批量操作、日志。没有左右常驻动作栏。

设备墙只承担在线监看：固定密度的左上对齐网格，卡片一层细边框，头部名称和状态，底部最多返回、主页、独立窗口。更多、管理与危险操作进入对应设备菜单。标准卡约 252–280px 宽，画面约 410–448px 高；紧凑和放大由用户显式选择。相同密度不因设备数量或最大化改变尺寸，窗口增加可容纳列数。少量设备的大图控制通过未来显式聚焦模式提供，不自动膨胀。

设备库承担完整管理：展示全部在线、未启动、启动中和异常设备；全宽表格有共同的名称、类型、状态和操作列。未启动行的启动动作可见，其他管理动作在更多。顶部筛选与搜索只影响显示，勾选按唯一 ID 保存，隐藏勾选不得被清空或重复计算。

主目标操作与批量勾选分别呈现。设备操作菜单先显示主目标身份，再列安装、卸载和输入，危险动作独立分组。安装等任务临时展开 320px 工具面板，持续显示目标名称与 serial；提交前固定目标。批量入口显示唯一设备数和目标清单。主目标用文字标识，勾选用浅底与选区边框，避免混同。

底部为 28px 状态条，集中工具健康、任务摘要和主目标。日志展开约 220px，占据固定窗口内部空间；设备网格自然滚动，画面尺寸不改变。保留只读拖选、全选、复制、暂停更新提示和跟随最新。

### 视觉规范

| 用途 | 值 |
| --- | --- |
| 工作区 / 工具栏 / 内容 | `#ECEEF1` / `#F7F8FA` / `#FFFFFF` |
| 正文 / 次文 / 分隔线 | `#252A34` / `#626B7A` / `#D8DDE5` |
| 强调 / 选中浅底 | `#5265D8` / `#E9EDFF` |
| 在线 / 警告 / 危险 | `#278361` / `#A66B12` / `#C54545` |
| 预览空边 | `#171A20` |
| 标题 / 正文 / 元信息 | 15 / 13 / 12px |
| 外边距 / 卡间距 / 卡内边距 | 16 / 16 / 12px |
| 圆角 / 边框 | 5–6px / 单层 1px |

中性色为主，强调色用于选择、焦点和真实提交；状态有文字伴随。没有嵌套白卡、装饰阴影、大块危险色或纯文字按钮堆砌。上述颜色要在实际 Fyne 渲染中检查，不能只凭网页原型宣称视觉通过。

### 落地顺序与验收门槛

1. 完成双视图、固定尺度和任务工具原型，独立审查目标及日志状态。
2. 在 Go/Fyne 迁移工作台骨架，保留既有设备连接、生命周期、权限恢复和目标安全实现；移除旧居中放大布局和右轨。
3. 接入完整设备库及明确的主目标/批量入口，再统一组件色彩、字号与状态。
4. 实测常规 1280×820、1440×900、最大化；分别检查 2 台和多台布局、工具展开、日志展开、空结果和异常状态。
5. 通过功能与视觉检查后，签名编译、备份替换本机 App，中文提交推送。

验收要求：同密度卡宽和画面高在不同窗口下偏差不超过 1px；桌面工具栏一行且不横向滚动；网格共同基线；无双层边框；正文与元信息对比度至少 4.5:1；元信息不小于 12px；工具与日志展开只减少可用空间而不改变卡片尺度；勾选跨搜索与视图保持且按唯一 ID 计数；不以虚构大量在线设备掩盖 2 台场景。

### 本轮研究交付

`docs/design/workbench-prototype.html` 是自包含、可直接打开的交互原型，明确标注示例画面，不接入 ADB。可切换窗口模式、2/6 台演示、设备墙/库、密度、搜索、状态筛选、目标工具菜单和日志。浏览器验证了 1280 与 1920 下标准卡尺寸不变、工具栏 48px、搜索和视图切换保留唯一勾选、日志在固定窗口内占高。此轮没有修改或替换 App，实际 Fyne 迁移属于以上计划的后续实施阶段。
