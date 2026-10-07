# 安卓设备矩阵 User Guide

安卓设备矩阵 is a local helper for Android devices and Android Virtual Devices. It keeps
the Android SDK tools separate from the 安卓设备矩阵 command so it does not conflict
with `adb`, `emulator`, `avdmanager`, or `sdkmanager`.

## Build

```bash
make build
```

This produces two user-facing artifacts:

- `bin/adm` for command-line usage.
- `dist/安卓设备矩阵.app` for the macOS desktop app.

To replace the locally installed desktop app:

```bash
make build
cp -R dist/安卓设备矩阵.app /Applications/安卓设备矩阵.app
```

Use one stable copy in `/Applications` when possible. This makes macOS app
search, Accessibility permission, and user support simpler than launching
different ad-hoc copies from different folders.

## macOS Dependency Bootstrap

On a new macOS machine, run:

```bash
make deps-macos
```

This calls `scripts/install-macos-deps.sh`. The script is also copied into the
macOS app bundle as `安卓设备矩阵.app/Contents/Resources/install-macos-deps.sh`, so the
GUI can run the same bootstrap from `工具` > `工具健康状态` > `安装/修复依赖`.
The script is safe to run more than once:

- It requires Homebrew and does not install Homebrew automatically.
- It installs missing Homebrew packages: `go`, `android-commandlinetools`, and
  `scrcpy`.
- It accepts Android SDK licenses and installs the SDK runtime packages 安卓设备矩阵
  expects: `platform-tools`, `emulator`, `platforms;android-36`, and
  `system-images;android-36;google_apis_playstore;arm64-v8a`.
- It prints the resolved paths for `go`, `adb`, `emulator`, `avdmanager`,
  `sdkmanager`, and `scrcpy`.

安卓设备矩阵 can also use tools installed by Android Studio. It searches explicit tool
paths from configuration first, then `ANDROID_HOME`, `ANDROID_SDK_ROOT`, common
Android SDK folders under the user home directory, Homebrew Android SDK folders,
and finally `PATH`.

For distribution, copy `dist/安卓设备矩阵.app` to `/Applications`. The app includes the
same dependency installer used by `make deps-macos`, so the target user does
not need the source checkout for first-run repair. Homebrew must already be
installed on the target machine; 安卓设备矩阵 does not install Homebrew and does not
request sudo. The CLI artifact `bin/adm` can be distributed separately when
only command-line workflows are needed.

Recommended first-run flow for another Mac:

1. Install Homebrew if it is missing.
2. Copy `安卓设备矩阵.app` into `/Applications`.
3. Open 安卓设备矩阵 and click `工具`.
4. If any tool is missing, run `安装/修复依赖`.
5. Click `重新检查` after the installer finishes.
6. Grant Accessibility permission only if external emulator window tiling is
   needed.

For production distribution, sign 安卓设备矩阵 with a stable signing identity.
Development builds use ad-hoc signing by default. After replacing an
ad-hoc-signed app, macOS may display an enabled `安卓设备矩阵.app` Accessibility entry
that belongs to the previous binary identity. If external window tiling still
reports missing permission, remove the old entry, add `/Applications/安卓设备矩阵.app`
again, and restart 安卓设备矩阵.

## GUI

Start the desktop UI:

```bash
open dist/安卓设备矩阵.app
```

Copy `安卓设备矩阵.app` into `/Applications` if you want it to appear in macOS
application search.

The GUI is designed as a plain tool surface:

- The top area shows the current operation target and Android tool status.
- The `工具` button shows dependency health and can run the bundled macOS
  dependency installer.
- The device wall fills the main area with physical devices, running emulators, and stopped AVDs. Its top toolbar contains all device management actions.
- The right area provides fixed task buttons for installing APKs, sending text,
  uninstalling apps, and device operations.
- The bottom log area shows recent operation status with `INFO`, `DONE`, and
  `ERROR` entries.

Use the `工具` panel as the runtime health center. It shows resolved paths for
Android SDK tools and `scrcpy`, runs the bundled macOS dependency installer,
and re-checks tool availability without requiring a source checkout.

### Control Center

The main window contains the device wall. It shows every known physical device, running emulator, and stopped AVD in one place. There is no separate left control panel.

The toolbar uses a single compact row for scanning, screenshot refresh, density, external windows, AVD creation, selection, main target, startup, shutdown and deletion. The `选择` menu groups selecting available devices, selecting stopped AVDs and clearing selection. `启动选中`, `关闭选中`, and `删除选中` use the card checkboxes for one or multiple AVDs. `设为主目标` requires exactly one available device. Deletion lists the selected AVDs and requires confirmation; physical devices are excluded. Logs can be expanded from the right rail.

The wall is for status and selection:

- Running `state=device` targets show an ADB screenshot preview.
- Stopped AVDs can be started from their card.
- Each running target has an `独立窗` action. For an Android Emulator target it
  focuses the native Emulator window; for a physical device it opens a scrcpy
  control window.
- The `独立窗` action opens a physical-device mirror above other windows. If it
  gets in the way, minimize the window yourself. Native emulator windows do
  not use this scrcpy window option.
- The top `外部窗` action opens external windows for available targets in batch:
  Android Emulator targets focus their native Emulator windows, and physical
  devices open scrcpy control windows. If nothing is selected, 安卓设备矩阵 opens all
  available targets.
- Click the card title, such as `demo_5050`, to copy the AVD or device display
  name. Click the compact status line below the preview to copy the active
  device serial, such as `emulator-5554`; stopped AVDs copy the AVD name.
- Screenshot preview clicks are a fallback helper; use the scrcpy mirror window
  for real-time control, notification shade gestures, and detailed operation.
- Right-click a device preview to send Android Back.

Real-time mirroring requires a callable local `scrcpy` binary. 安卓设备矩阵 launches
scrcpy with a per-device serial, a stable window title, no audio, UHID keyboard
input, and a per-device log file. It uses a compact 624-point outer window height with a natural device aspect
ratio for physical devices and resizes an existing scrcpy process for the same
serial instead of opening a duplicate mirror. It does not use `--time-limit`.

The `外部窗` action can focus native Android Emulator windows and open scrcpy
windows for physical devices. Focusing or arranging native Emulator windows on
macOS needs Accessibility permission because 安卓设备矩阵 controls another app's
windows. When permission is missing, the app shows an authorization guide. Click `打开系统设置` to open System Settings > Privacy & Security > Accessibility and enable `安卓设备矩阵.app`. The `工具` panel also has an `辅助功能授权` entry. After enabling permission, return to the app and retry; restart the app if the permission is not yet recognized.

### Install APK

Use `Install APK` for a local `.apk` file or an `http://` / `https://` URL.
Use `Install to multiple devices` when the same APK should be installed on more
than one connected `state=device` target.
When the input is empty, install actions reuse the last APK shown in the input
placeholder.

`Allow downgrade install` maps to `adb install -r -d`. Leave it unchecked for
normal updates. Check it only when installing an APK whose `versionCode` is
lower than the currently installed app. Without this option Android rejects the
install with `INSTALL_FAILED_VERSION_DOWNGRADE`.

### Uninstall App

The uninstall page is list-first:

1. Refresh the app list.
2. Optionally filter the list by keyword.
3. Choose the exact package from the list.
4. Confirm uninstall.

The filter text is never treated as the package name. This prevents accidental
uninstall from partial text.

### Create Emulator

The create dialog shows phone and tablet templates only. TV, Wear OS,
automotive, desktop, XR, glasses, and headset templates are filtered out.
Device templates older than roughly three years are hidden. Newer Pixel
templates are sorted before generic phone and tablet templates.
安卓设备矩阵 only shows hardware profiles returned by `avdmanager list device`; it does
not synthesize newer device names from older templates.

Use `Batch Start` to start multiple stopped AVDs. Use `Batch Delete` to delete
multiple AVDs. If a selected AVD is running or ADB offline, the GUI requires an
extra force-close confirmation before it closes that emulator and deletes the
AVD.

Created and launched AVDs are prepared with hardware keyboard forwarding:

- `hw.keyboard=yes`
- `hw.keyboard.charmap=qwerty2`
- `hw.keyboard.lid=yes`
- emulator launch with `-use-keycode-forwarding`
- emulator launch with `-change-locale zh-CN`
- launch without loading old snapshots
- wait for Android framework boot and then set `show_ime_with_hard_keyboard=1`
- set the default emulator system locale to `zh-CN` after boot
- if a Play Store image cannot apply the locale through emulator flags, use the
  Settings language page to add Simplified Chinese (China) and move it to the
  first language automatically

### Text Input

Simple ASCII text uses `adb shell input text`.

Chinese or complex text uses ADB Keyboard. The GUI detects complex characters
and can install ADB Keyboard on the current device.

## CLI

Interactive mode:

```bash
./bin/adm
```

List active devices:

```bash
./bin/adm -l
```

List active devices and stopped AVDs:

```bash
./bin/adm -al
```

The CLI remembers the last selected current device when it is still available.

## Safety

Dangerous operations require stronger confirmation:

- Deleting an AVD requires explicit confirmation.
- Closing or disconnecting a device in the GUI requires typing the current
  device serial.
- Powering off a physical device requires typing the serial.

The tool does not install duplicate Android SDK command tools. It detects
callable tools first and lets the user set explicit paths when SDK tools are in
non-standard locations.

The control center depends on ADB and, for real-time mirrors, `scrcpy`. macOS
Accessibility permission is not required for scrcpy mirroring; it only affects
legacy external emulator window focus or tiling helpers.

## 桌面工作台（2026-10）

顶部操作保持单行，窗口变窄时次要操作收入「更多」。顶部搜索框搜索设备名称或 serial；顶部「画面」下拉即时切换小、标准、高清三档并记住选择；「视图」提供状态筛选。设备墙显示在线预览，右侧显示未启动与异常设备，可直接启动；「设备库」切换到全部设备的表格。

勾选表示批量范围，主目标表示顶部「设备操作」菜单中安装、卸载、输入、重启和关闭的单台目标。批量启动、关闭及删除会列出可执行目标和跳过原因；失败结果可仅重试失败设备。主目标切换后会清空上一设备的包列表，重新加载后再卸载。

「外部窗」统一窗口高度（默认624点），宽度按设备画面比例计算。保留 scrcpy 默认比例锁，避免强制等宽形成左右黑框，不拉伸或裁切画面；旧的本应用镜像会自动重开以应用新参数。未勾选时打开全部在线设备；明确勾选但没有合格设备时只显示原因，不会扩大范围。排列失败时使用「打开系统设置」进入辅助功能配置，手动授权后点击「已开启，重新检查」继续排列原窗口。若当前进程仍未获得权限，按引导重新添加应用并重启。

创建模拟器时如无系统镜像，可选择下载推荐镜像。下载前需接受 Android SDK 许可，也可在 Android Studio SDK Manager 安装后返回创建。

### 设备墙空间与日志复制

在线预览按所选密度保持尺寸并从左上排列；最大化增加列数，不自动放大画面。「设备库」切换到所有设备的表格，名称、类型、状态和操作按列对齐。顶部「设备操作」菜单提供主目标工具，按需在右侧展开，可点击「收起」关闭；状态集中在底部。展开日志后画面保持大小，网格可纵向滚动。

展开「日志」后可拖动选择文字，使用 ⌘A 全选、⌘C 复制，也可右键复制或点击「复制选中」。点击「复制全部」复制最近 200 行。文本为只读；选择时暂停显示更新，后台继续收集日志，点击「跟随最新」恢复。


### 原有功能与重构回归

画面设置保留原有小 `300×584`、标准 `360×724`、高清 `440×920` 的预览尺寸，设置控制工作台显示大小，不修改 Android 设备本身的分辨率。最大化只改变列数，画面超过工作区时可滚动。点击在线卡片或设备库中的名称复制完整名称；设备卡片「更多」和管理窗口提供复制设备编号。设备库的编号也可点击复制，截断显示不影响复制内容。

启动中、离线或尚未连接 ADB 的运行模拟器仍有聚焦窗口与关闭入口，允许通过正常关闭恢复，不需要删除设备。批量关机只接受模拟器，真机继续要求精确编号确认。批量外部窗遇到个别打开失败时，仍排列成功打开的窗口并报告失败项；调整后的实际窗口尺寸会校验，不能达到统一尺寸时报告错误。
