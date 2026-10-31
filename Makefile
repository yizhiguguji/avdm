APP_NAME ?= 安卓设备矩阵
APP_STEM ?= adm
APP_ID ?= com.zhuiguang.advm
APP_VERSION ?= 0.1.0
# Keychain code-signing certificate name. This is an internal keychain artifact
# (not user-visible); keeping the original name avoids having to recreate the
# certificate. Installed builds must not silently fall back to ad-hoc signing.
LOCAL_CODESIGN_IDENTITY ?= AVDM Local Code Signing
DETECTED_CODESIGN_IDENTITY := $(shell security find-identity -v -p codesigning 2>/dev/null | grep -Fq '"$(LOCAL_CODESIGN_IDENTITY)"' && printf '%s' '$(LOCAL_CODESIGN_IDENTITY)' || printf '%s' '-')
CODESIGN_IDENTITY ?= $(DETECTED_CODESIGN_IDENTITY)
ALLOW_ADHOC_SIGNING ?= 0
CODESIGN_OPTIONS ?=

BIN_DIR := bin
DIST_DIR := dist
APP_BUNDLE := $(DIST_DIR)/$(APP_NAME).app
APP_CONTENTS := $(APP_BUNDLE)/Contents
APP_EXECUTABLE := $(APP_CONTENTS)/MacOS/$(APP_STEM)
APP_PLIST := $(APP_CONTENTS)/Info.plist
APP_ICON_PNG := assets/adm-icon.png
APP_ICON_ICNS := assets/adm-icon.icns
APP_ICONSET := $(DIST_DIR)/$(APP_STEM).iconset

.PHONY: all build cli app package-macos deps-macos check-macos-signing

all: build

build: cli app

deps-macos:
	./scripts/install-macos-deps.sh

cli:
	mkdir -p "$(BIN_DIR)"
	go build -trimpath -ldflags "-s -w" -o "$(BIN_DIR)/$(APP_STEM)" ./cmd/adm

app: package-macos

$(APP_ICON_ICNS): $(APP_ICON_PNG)
	mkdir -p "$(APP_ICONSET)"
	sips -z 16 16 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_16x16.png"
	sips -z 32 32 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_16x16@2x.png"
	sips -z 32 32 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_32x32.png"
	sips -z 64 64 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_32x32@2x.png"
	sips -z 128 128 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_128x128.png"
	sips -z 256 256 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_128x128@2x.png"
	sips -z 256 256 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_256x256.png"
	sips -z 512 512 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_256x256@2x.png"
	sips -z 512 512 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_512x512.png"
	sips -z 1024 1024 "$(APP_ICON_PNG)" --out "$(APP_ICONSET)/icon_512x512@2x.png"
	iconutil -c icns "$(APP_ICONSET)" -o "$(APP_ICON_ICNS)"

check-macos-signing:
	@if [ "$(CODESIGN_IDENTITY)" = "-" ] || [ -z "$(CODESIGN_IDENTITY)" ]; then \
		if [ "$(ALLOW_ADHOC_SIGNING)" != "1" ]; then \
			printf '%s\n' '错误：没有稳定的代码签名身份，已停止打包，避免更新后辅助功能授权失效。' '请配置有效 CODESIGN_IDENTITY；仅调试时可显式设置 ALLOW_ADHOC_SIGNING=1。' >&2; exit 1; \
		fi; \
		printf '%s\n' '警告：本次显式使用临时签名；替换已安装应用后可能需要重新授权辅助功能。' >&2; \
	fi

package-macos: check-macos-signing $(APP_ICON_ICNS)
	mkdir -p "$(APP_CONTENTS)/MacOS" "$(APP_CONTENTS)/Resources"
	go build -trimpath -ldflags "-s -w -X main.applicationID=$(APP_ID)" -o "$(APP_EXECUTABLE)" ./cmd/adm-gui
	cp "$(APP_ICON_ICNS)" "$(APP_CONTENTS)/Resources/$(APP_STEM).icns"
	cp "scripts/install-macos-deps.sh" "$(APP_CONTENTS)/Resources/install-macos-deps.sh"
	clang -dynamiclib -fobjc-arc -fblocks -framework AppKit scripts/mirror-title-icon.m -o "$(APP_CONTENTS)/Resources/mirror-title-icon.dylib"
	mkdir -p "$(APP_CONTENTS)/Resources/device-icons"
	cp assets/device-icons/*.png "$(APP_CONTENTS)/Resources/device-icons/"
	chmod +x "$(APP_CONTENTS)/Resources/install-macos-deps.sh"
	printf '%s\n' \
		'<?xml version="1.0" encoding="UTF-8"?>' \
		'<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
		'<plist version="1.0">' \
		'<dict>' \
		'  <key>CFBundleDevelopmentRegion</key>' \
		'  <string>zh_CN</string>' \
		'  <key>CFBundleDisplayName</key>' \
		'  <string>$(APP_NAME)</string>' \
		'  <key>CFBundleExecutable</key>' \
		'  <string>$(APP_STEM)</string>' \
		'  <key>CFBundleIdentifier</key>' \
		'  <string>$(APP_ID)</string>' \
		'  <key>CFBundleIconFile</key>' \
		'  <string>$(APP_STEM)</string>' \
		'  <key>CFBundleInfoDictionaryVersion</key>' \
		'  <string>6.0</string>' \
		'  <key>CFBundleName</key>' \
		'  <string>$(APP_NAME)</string>' \
		'  <key>CFBundlePackageType</key>' \
		'  <string>APPL</string>' \
		'  <key>CFBundleShortVersionString</key>' \
		'  <string>$(APP_VERSION)</string>' \
		'  <key>CFBundleVersion</key>' \
		'  <string>$(APP_VERSION)</string>' \
		'  <key>LSApplicationCategoryType</key>' \
		'  <string>public.app-category.developer-tools</string>' \
		'  <key>NSHighResolutionCapable</key>' \
		'  <true/>' \
			'</dict>' \
		'</plist>' \
		> "$(APP_PLIST)"
	chmod +x "$(APP_EXECUTABLE)"
	plutil -lint "$(APP_PLIST)"
	./scripts/sign-macos-app.sh "$(CODESIGN_IDENTITY)" "$(APP_ID)" "$(APP_BUNDLE)" $(CODESIGN_OPTIONS)
