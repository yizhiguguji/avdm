package app

import (
	"fmt"
	"runtime"
	"time"
)

func (a *App) GUIAccessibilityTrusted() bool { return accessibilityTrustedNative() }

func GUIRecommendedSystemImage() string {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64-v8a"
	}
	return "system-images;android-35;google_apis_playstore;" + arch
}

func (a *App) GUIInstallRecommendedSystemImage() error {
	imageID := GUIRecommendedSystemImage()
	out, err := a.runToolOutput(toolSDKManager, 20*time.Minute, imageID)
	if err != nil {
		return fmt.Errorf("安装镜像失败：%w\n%s", err, out)
	}
	images, err := a.installedSystemImages()
	if err != nil {
		return err
	}
	if !containsString(images, imageID) {
		return fmt.Errorf("镜像尚未安装。请在 Android Studio 的 SDK Manager 接受 SDK 许可后重试。\n%s", out)
	}
	return nil
}
