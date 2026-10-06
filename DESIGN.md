# Design

## Source of truth
- Status: Active
- Last refreshed: 2026-06-04
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
- Token/component ownership: Keep in `cmd/adm-gui/main.go` until repeated patterns justify extraction.

## Accessibility
- Target standard: Keyboard usable for primary actions; readable contrast in default macOS appearance.
- Keyboard/focus behavior: Primary controls must remain reachable by normal Fyne focus traversal.
- Contrast/readability: Secondary metadata cannot be too light to read.
- Screen-reader semantics: Prefer meaningful labels over unlabeled symbolic controls.
- Reduced motion and sensory considerations: No flashing or animation-heavy surfaces.

## Responsive behavior
- Supported breakpoints/devices: macOS desktop windows from 1280x820 upward; optimize 1440x900 and larger.
- Layout adaptations: Left rail holds scan/selection/batch controls, center holds the device wall, right rail holds context actions, bottom holds logs.
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
