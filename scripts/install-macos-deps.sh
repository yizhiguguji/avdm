#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[adm-deps] %s\n' "$*"
}

require_macos() {
  if [[ "$(uname -s)" != "Darwin" ]]; then
    log "This installer is for macOS only."
    exit 1
  fi
}

require_brew() {
  if command -v brew >/dev/null 2>&1; then
    return
  fi
  log "Homebrew is required. Install it first: https://brew.sh/"
  exit 1
}

brew_install_if_missing() {
  local formula="$1"
  if brew list --formula --versions "$formula" >/dev/null 2>&1 || brew list --cask --versions "$formula" >/dev/null 2>&1; then
    log "$formula already installed"
    return
  fi
  log "Installing $formula"
  brew install "$formula"
}

first_cmd() {
  local name="$1"
  if command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
    return
  fi
  local brew_prefix
  brew_prefix="$(brew --prefix)"
  for candidate in \
    "$brew_prefix/bin/$name" \
    "$brew_prefix/share/android-commandlinetools/cmdline-tools/latest/bin/$name" \
    "$brew_prefix/share/android-commandlinetools/cmdline-tools/bin/$name" \
    "$brew_prefix/share/android-commandlinetools/platform-tools/$name" \
    "$brew_prefix/share/android-commandlinetools/emulator/$name" \
    "$HOME/Library/Android/sdk/cmdline-tools/latest/bin/$name" \
    "$HOME/Library/Android/sdk/platform-tools/$name" \
    "$HOME/Library/Android/sdk/emulator/$name"; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done
  return 1
}

install_android_components() {
  local sdkmanager
  if ! sdkmanager="$(first_cmd sdkmanager)"; then
    log "sdkmanager was not found after installing android-commandlinetools"
    exit 1
  fi

  log "Accepting Android SDK licenses"
  # yes may receive SIGPIPE when sdkmanager finishes; check sdkmanager's exit code.
  set +e
  yes | "$sdkmanager" --licenses
  local license_status=${PIPESTATUS[1]}
  set -e
  if [[ "$license_status" -ne 0 ]]; then
    log "ERROR: Android SDK license check failed (exit $license_status)"
    return "$license_status"
  fi

  local component
  for component in "platform-tools" "emulator" "platforms;android-36" "system-images;android-36;google_apis_playstore;arm64-v8a"; do
    log "Installing Android SDK component: $component"
    local attempt install_status=0
    for attempt in 1 2 3; do
      if "$sdkmanager" --verbose "$component"; then
        install_status=0
        log "Installed Android SDK component: $component"
        break
      else
        install_status=$?
        log "Download/install attempt $attempt/3 failed: $component (exit $install_status)"
      fi
    done
    if [[ "$install_status" -ne 0 ]]; then
      log "ERROR: Android SDK component failed: $component (exit $install_status)"
      return "$install_status"
    fi
  done
}

print_summary() {
  log "Tool summary"
  for tool in go adb emulator avdmanager sdkmanager scrcpy; do
    if path="$(first_cmd "$tool")"; then
      printf '  %-10s %s\n' "$tool" "$path"
    else
      printf '  %-10s %s\n' "$tool" "missing"
    fi
  done
}

main() {
  require_macos
  require_brew

  brew_install_if_missing openjdk
  export JAVA_HOME="$(brew --prefix openjdk)/libexec/openjdk.jdk/Contents/Home"
  export PATH="$JAVA_HOME/bin:$PATH"
  "$JAVA_HOME/bin/java" -version

  brew_install_if_missing go
  brew_install_if_missing android-commandlinetools
  brew_install_if_missing scrcpy

  install_android_components
  print_summary

  log "Done. Open 安卓设备矩阵 and click Tools / 工具 to re-check paths."
  log "If macOS blocks external emulator tiling, allow 安卓设备矩阵 in System Settings > Privacy & Security > Accessibility."
}

main "$@"
