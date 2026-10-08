# 安卓设备矩阵

Android device and virtual device management helper.

See [docs/USER_GUIDE.md](docs/USER_GUIDE.md) for detailed GUI and CLI usage.

## Build

```bash
make build
```

This creates the command-line tool at `bin/adm` and the macOS desktop app at
`dist/安卓设备矩阵.app`.

For release-style local packaging, build the app and copy it into
`/Applications`:

```bash
make build
cp -R dist/安卓设备矩阵.app /Applications/安卓设备矩阵.app
```

## macOS Dependencies

For a fresh macOS machine, install the runtime dependencies from the source
checkout:

```bash
make deps-macos
```

The dependency installer is idempotent. It uses Homebrew to install missing
tools and then asks `sdkmanager` to install the Android SDK components 安卓设备矩阵
needs:

- `go` for local builds.
- `android-commandlinetools` for `adb`, `emulator`, `avdmanager`, and
  `sdkmanager`.
- `scrcpy` for low-latency real-time device mirrors.
- Android SDK `platform-tools`, `emulator`, `platforms;android-36`, and the
  recommended ARM64 Play Store system image.

Homebrew itself is not installed automatically. Install Homebrew first from
`https://brew.sh/`, then run `make deps-macos`.

The packaged macOS app also embeds the same installer at
`安卓设备矩阵.app/Contents/Resources/install-macos-deps.sh`. In the GUI, open
`工具` > `工具健康状态` and click `安装/修复依赖` to run it, then 安卓设备矩阵 will
re-check the tool paths. This is the intended first-run path when distributing
only `安卓设备矩阵.app`.

## Usage

```bash
./bin/adm        # interactive menu
./bin/adm -l     # active devices
./bin/adm -al    # active devices and stopped AVDs
```

Open `dist/安卓设备矩阵.app` for the desktop UI. Copy `安卓设备矩阵.app` into
`/Applications` if you want it to appear in macOS application search.

When distributing 安卓设备矩阵 to another user, ship `dist/安卓设备矩阵.app`. The app contains
the dependency installer and can run it from the `工具` panel, so users do not
need the source checkout for first-run repair. Homebrew still has to exist on
the target machine; 安卓设备矩阵 deliberately does not install Homebrew or request sudo.
macOS Accessibility permission also has to be granted manually in System
Settings because macOS does not allow apps to grant that permission silently.
安卓设备矩阵 detects tools from `ANDROID_HOME`, `ANDROID_SDK_ROOT`, common Homebrew
Android SDK paths, and `PATH`.

First-run checklist for another Mac:

1. Install Homebrew if it is missing.
2. Copy `安卓设备矩阵.app` into `/Applications`.
3. Open 安卓设备矩阵, click `工具`, then run `安装/修复依赖` if any tool is missing.
4. Click `重新检查` in the same panel after the installer finishes.
5. Grant Accessibility permission only if external emulator window tiling is
   needed.

For reliable Accessibility permission across upgrades, distribute a consistently
signed app. Local ad-hoc builds are fine for development, but replacing an
ad-hoc-signed `.app` changes the code identity hash and macOS may keep showing
the old `安卓设备矩阵.app` entry while the new binary is not trusted. In that case,
remove the old Accessibility entry, add the new `/Applications/安卓设备矩阵.app`, and
restart 安卓设备矩阵.

The GUI is intentionally plain. It keeps the same backend logic as the CLI and
organizes the daily workflow around a device list, the current-device action
panel, tool status, and operation logs.

Interactive mode keeps a current device and remembers the last selected device
when it is still available. With a current device selected, the main menu works
as a current-device console for install, uninstall, text input, IME, language,
reboot, and shutdown actions.

Device and emulator lists include physical devices, running emulators, and
stopped AVDs. Choosing a stopped AVD asks whether to start it, waits until it is
usable, and then opens actions for that selected device.

## Features

- List active Android devices and stopped AVDs.
- Start, create, close, and delete AVDs.
- Install APKs from local paths or HTTP/HTTPS URLs.
- Uninstall packages from a selected device.
- Manage language: open system language settings or try best-effort system locale writes without pretending they always apply.
- List, enable, and select Android input methods.
- Enable soft keyboard display when a hardware keyboard is connected.
- Enable AVD physical keyboard input by setting hardware keyboard config, emulator shortcut forwarding, and keycode forwarding.
- Send text with automatic mode selection: simple ASCII through `adb shell input text`, Chinese or complex text through ADB Keyboard.
- Reboot the current device.
- Close an emulator, disconnect a TCP device, or power off a physical device.
- Manage aliases for device serials and AVD names.
- Remember the last APK source for quick reinstall.
- Open a control-center wall for device status, screenshot previews, and selected-device actions.
- Open low-latency scrcpy mirror windows for real-time device control.
- Show tool diagnostics for non-standard Android SDK installations.
- Use a desktop GUI for common current-device workflows without walking through
  nested numeric menus.

APK installation accepts either a local APK path or an `http://` / `https://`
URL. URL downloads are streamed to a temporary file and removed after install.
Install mode is detected from `adb install` failures: downgrade installs retry
with `-d` after confirmation, and `testOnly` packages retry with `-t`.
Signature conflicts and blocked release downgrades offer an explicit
uninstall-and-reinstall or abandon choice in both GUI install entry points.
Uninstalling clears app data; batch retries target only the affected devices.
Keep-data uninstall uses `adb shell pm uninstall -k` and can be combined with
`--user 0`. Keeping data cannot resolve a signing-key conflict.

Both GUI uninstall lists default to user-installed applications on the current
Android user: non-system packages whose installation metadata identifies a user
install with a launcher entry, local/downloaded APK, or ADB initiator.
Store-delivered background components without a launcher entry are excluded;
ADB and manually installed APKs do not require a launcher entry. Unknown installation origins
are excluded instead of guessing from vendor names; unchecking the filter
shows all packages. Android metadata is installer-reported, so it is not a
complete audit of who physically installed an application. See Android's
[installation reasons](https://developer.android.com/reference/android/content/pm/PackageManager#INSTALL_REASON_USER)
and [package sources](https://developer.android.com/reference/android/content/pm/PackageInstaller#PACKAGE_SOURCE_DOWNLOADED_FILE).

The `独立窗` action opens a physical-device mirror above other windows. If it
gets in the way, minimize the window yourself. Emulator native windows do not
use this scrcpy window option.

The tool reuses existing Android SDK tools when they are callable. It does not
install duplicate `adb` tooling, and it avoids naming the binary `avdmanager`
to prevent conflicts with the Android SDK command.

Real-time mirroring uses the local `scrcpy` command when available. The control
center keeps ADB screenshots as a status preview, but detailed operation should
use the scrcpy mirror window.

External Android Emulator window tiling on macOS uses Accessibility automation.
If the `外部窗` action cannot move or focus emulator windows, allow `安卓设备矩阵.app`
in System Settings > Privacy & Security > Accessibility, then retry.
安卓设备矩阵 checks this permission without triggering Apple's system prompt during
window operations; failed checks are reported in the app log instead.

Physical keyboard and keycode-forwarding changes require restarting the running
emulator process before they apply to that instance. 安卓设备矩阵 starts emulators with
keycode forwarding and without loading old snapshots to avoid stale keyboard
state. AVD creation and startup also write hardware keyboard config so new AVDs
use the same behavior.

Dangerous operations require stronger confirmation. Deleting an AVD requires
typing the AVD name, and powering off a physical device requires typing the
device serial.

## Tests

Run `go test ./...` for the automated suite. Device integration tests are opt-in.
For the read-only installed-package inventory check, set
`ADM_LIVE_PACKAGE_SERIAL` to the target device serial. Optionally set
`ADM_LIVE_EXPECTED_PACKAGE` or its compatible alias `ADM_LIVE_PACKAGE_EXPECTED`
to a package that must be present. If both are set, both packages are checked.
Keep actual device identifiers and business package names in environment
variables rather than committed examples or fixtures.
