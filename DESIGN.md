# Design

## Source of truth
- Status: Redesign baseline implemented in the application; current delivery recorded below, subject to user visual review.
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

### 双视图工作台实际落地（2026-10-07）

移除顶部状态行、右侧常驻动作轨和 1600px 居中内容上限，工作区使用整窗可用空间。唯一顶部操作行含设备库切换、搜索、扫描、视图、常用动作与设备操作菜单；较窄时次要动作进入更多。状态移到底部，任务工具临时展开并显示主目标及收起入口。设备库包含全部设备，名称、类型、状态和动作按共同列对齐。

预览采用固定三档密度、左上共同基线，默认标准；按手机约 0.45 的宽高比减少画面黑边，卡片仅留单行返回、主页、窗口，管理收入更多。辅助字号由 11px 提高为 12px，正文与辅助文字采用深色，减少泛白、低对比的问题。

布局测试覆盖常规与大窗口尺寸不漂移、基线、无居中上限、双视图及预览复用；完整 race 与 vet 通过。已对本机常规、最大化、完整设备库、工具与日志状态进行实际检查，签名构建并备份替换安装版。实际窗口截图仅存放在忽略的本地 .build/ui-review-20261007，未将设备屏幕内容推送到仓库。

### 控件对齐与界面层次修正（2026-10-07）

搜索框使用工具栏专属的蓝灰填充和圆角，取消突兀的常态输入边框，聚焦仍保留提示。工具栏和底部状态栏采用统一 36px 槽位，单行输入与标签保持自身高度并居中，修正文字靠上及按钮与标签基线错位。工作区底色、卡片边框、复选框和辅助文字加深；白色操作栏与浅蓝灰状态栏增加清晰边界。本机常规与最大化窗口实际检查通过，已签名编译、备份替换；race、vet 通过。验收截图仍只保留在忽略的本地构建目录。

### 恢复设备可见性及圆角（2026-10-07）

根据实际使用反馈，设备墙旁恢复未启动与异常设备列表，名称、选择、状态、管理与直接启动入口始终可见，不再要求用户先切换设备库。列表和在线预览共享搜索条件，完整设备库保留；没有未启动或异常设备时侧列隐藏。顶部操作栏、底部状态栏统一 8px 圆角。搜索框的空提示按真实边界水平及垂直居中，聚焦或输入时隐藏提示，不影响原输入、过滤和键盘能力。本机验证提示居中、聚焦隐藏、在线与未启动设备过滤、清空恢复及圆角，race、vet 和签名构建通过，已备份替换应用。此交付修正退化问题，整体设计仍需持续接受用户视觉验收。

### 原有功能契约与退化修复（2026-10-07）

此次先对照 49aef8a、76a6ae7 与当前实现，逐项确认功能是否可达和行为是否一致，再落实修改。原项目核心是同时管理在线真机、运行模拟器、等待 ADB 的模拟器和未启动 AVD；内嵌画面交互、外部窗口开窗/平铺、批量范围和单台主目标属于不同操作范围，布局简化不能删除其状态恢复路径。

| 原功能或契约 | 发现的问题 | 实际修复与验证 |
| --- | --- | --- |
| 顶部小/标准/高清即时调整 | 移入弹窗，预览尺寸缩水 | 恢复 300×584、360×724、440×920 与顶部即时下拉，持久化选择；实机逐档切换，单测验证会话与勾选不丢失 |
| 名称与编号复制 | copyable helper 无调用 | 卡片标题与库名称/编号可复制，卡片菜单和管理中有明确复制入口；实机名称复制成功 |
| 等待 ADB / offline 模拟器恢复 | 窗口和关机入口缺失，批量不能关闭无ADB的进程 | 补回聚焦/关闭入口，复用AVD关闭与互斥；fakeADB测试覆盖公开单台、批量API及真机前置拒绝 |
| 默认打开模拟器窗口 | 默认置顶参数触发拒绝模拟器错误 | 普通开窗与明确真机置顶请求分开；模拟器在线/离线/无ADB状态分支测试 |
| 批量外部窗 | 任一打开失败，成功窗口也跳过平铺 | 成功keys继续排列，合并失败原因，保留授权恢复类型；部分失败与全失败测试 |
| 外部窗统一大小 | 两套尺寸、AX失败被忽略；scrcpy比例锁使同一请求变成不同外框 | 统一624点高度、宽度随真实画面比例；保留scrcpy比例锁，取消固定宽度造成的黑边；原生setter重试并读回校验 |
| 本应用旧镜像升级 | SIGINT被旧进程忽略 | 仅针对本应用识别出的旧镜像使用SIGTERM正常退出再重开，其他程序镜像保持保护边界；实际升级两镜像成功 |
| 窗口工作区 | 沿用旧左右栏固定留白 | 对齐单工具栏布局，整个网格同比缩放，尺寸与位置末轮读回复验；五种网格布局测试 |

前轮固定288×624外框的方案造成额外黑边，已取消。新方案按内容比例统一高度，复用与重复平铺验证共同高度及比例不漂移。真实测试日志保存在忽略的 .build/external-window-verification-20261007.log，设备内容截图仅本地保留。当前没有运行中的原生模拟器，其状态、开窗路由和关闭用fakeADB/注入分支测试验证，Qt原生窗口约束未做设备实测，遇到拒绝统一外框会报告请求/实际尺寸，不假报成功。

### 主窗口对齐修正

顶部工具行和底部状态行共享8点水平内边距，所有操作控件使用36点高度。搜索提示和密度选项的常规字形按按钮文字做3点光学校正，控件背景保持同一垂直中心。

本轮两台真机实测外框约265×623、273×623，共同画面高度、保留各自画面比例；模拟不同高度再连续两次平铺，比例和尺寸无漂移。原生模拟器当前未运行，未声称完成其真实窗口视觉验收。

### 恢复直接操作动线

右侧恢复常驻64点设备操作栏，安装、卸载、输入、设主目标、重启、关闭、日志均可直接点击；移除顶部设备操作下拉与重复日志入口。主目标和批量选择继续分开，危险操作仍保留原确认流程。右侧工具面板单实例切换，保留已填写内容，再点活动按钮即可收起。顶部继续只承担设备搜索、显示设置与全局/批量操作。

### 操作复杂度退化修正（2026-10-07）

卡片恢复两行各四个高频操作：主目标、管理、窗口、隐藏、返回、主页、通知、关闭。名称和编号支持单击复制。状态筛选改为直接菜单，避免重复搜索和大小设置；密度入口改称「大小」，表示预览显示尺寸。选在线、清选直接可达，勾选后现有底栏显示启动、关闭、删除与清选入口。安装明确显示勾选优先的实际范围以及筛选隐藏数，过滤不改变操作目标。

常驻右侧操作栏保持直接入口，工具展开时按实际列数折叠未启动栏，数量及展开按钮持续可见；无在线设备时直接展示启动列表。设备库窄布局保留名称、状态和操作，避免固定列宽溢出。隐藏预览停止实时流，显示重新连接；批量关闭无 ADB 模拟器按 AVD 名执行，真机管理按实际类型命名。回归覆盖批量执行、隐藏连接生命周期、全部直接卡片动作、筛选后的范围及宽窄布局。

本轮完整 `go test -race ./...` 与 `go vet ./...` 通过，已签名构建并备份替换 `/Applications/安卓设备矩阵.app`，签名及二进制一致性校验通过。实机检查正常/最大化、安装面板两列、卡片八个直接入口、勾选后的底栏、筛选隐藏范围提示及未启动列表弹窗。当前无运行原生模拟器，关闭执行使用注入服务回归，未对真机执行关闭、删除或安装。截图仅留本地忽略目录。

### 底部文字基线修正

底栏工具与任务文字增加2点光学校正，使其字形基线与检查工具、批量按钮及目标文字一致；保留36点槽位和原水平内边距，活动指示居中。GUI测试与签名构建通过，已替换安装版，并检查正常、最大化及勾选后批量入口出现时的底栏。

### 搜索框焦点光标居中

搜索框按当前字体行高居中整个可编辑视口，让光标、输入文字和选区共同对齐；保留单行横向滚动及原背景边界。恢复自定义控件的渲染绑定，使输入和焦点刷新继续应用对齐。回归覆盖空值、中文、长编号及连续刷新；GUI race与vet通过，已编译替换，实机检查空光标与混合文字全选。
