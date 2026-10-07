# Design

## Source of truth
- Status: Active
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
- Color: Dark operational base; blue for primary actions, green for healthy state, red only for destructive actions.
- Typography: Compact hierarchy; bold device names, muted metadata, badge-like states; no oversized headings inside tool surfaces.
- Spacing/layout rhythm: Tight but readable, with consistent row heights, compact toolbars, and no empty placeholder columns.
- Shape/radius/elevation: 6-8px radius, low contrast borders, no nested card stacks.
- Motion: No decorative motion.
- Imagery/iconography: Use icons only when a Fyne/theme icon clearly improves scanability; otherwise concise text is acceptable.

## Components
- Existing components to reuse: Fyne `widget.List`, `widget.Button`, `widget.Select`, `widget.Card`, `container.Border`, split containers.
- New/changed components: Operational dark theme, device row, device wall card, black preview pane, status badge, compact action toolbar, collapsible log/task panel.
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
- Design-token constraints: Fyne theming is limited; use the 安卓设备矩阵 dark theme and repo-local surface helpers before adding custom drawing.
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
