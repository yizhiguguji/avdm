APP_NAME ?= 安卓设备矩阵
APP_STEM ?= adm
APP_ID ?= com.zhuiguang.advm
APP_VERSION ?= 0.1.0
# Keychain code-signing certificate name. This is an internal keychain artifact
# (not user-visible); keeping the original name avoids having to recreate the
# certificate. Builds fall back to ad-hoc signing if it is absent.
LOCAL_CODESIGN_IDENTITY ?= AVDM Local Code Signing
DETECTED_CODESIGN_IDENTITY := $(shell security find-identity -v -p codesigning 2>/dev/null | grep -Fq '"$(LOCAL_CODESIGN_IDENTITY)"' && printf '%s' '$(LOCAL_CODESIGN_IDENTITY)' || printf '%s' '-')
CODESIGN_IDENTITY ?= $(DETECTED_CODESIGN_IDENTITY)
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

.PHONY: all build cli app package-macos deps-macos

all: build

build: cli app

deps-macos:
	./scripts/install-macos-deps.sh

cli:
	mkdir -p "$(BIN_DIR)"
	go build -o "$(BIN_DIR)/$(APP_STEM)" ./cmd/adm

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

package-macos: $(APP_ICON_ICNS)
	mkdir -p "$(APP_CONTENTS)/MacOS" "$(APP_CONTENTS)/Resources"
	go build -o "$(APP_EXECUTABLE)" ./cmd/adm-gui
	cp "$(APP_ICON_ICNS)" "$(APP_CONTENTS)/Resources/$(APP_STEM).icns"
	cp "scripts/install-macos-deps.sh" "$(APP_CONTENTS)/Resources/install-macos-deps.sh"
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
	codesign --force --deep $(CODESIGN_OPTIONS) --sign "$(CODESIGN_IDENTITY)" --identifier "$(APP_ID)" "$(APP_BUNDLE)"
