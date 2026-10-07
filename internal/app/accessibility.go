package app

import (
	"errors"
	"runtime"
)

const AccessibilitySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

const AccessibilityPermissionGuide = "请在「系统设置 → 隐私与安全性 → 辅助功能」中开启「安卓设备矩阵.app」。\n\n如果列表中没有此 App，点击「＋」添加 /Applications/安卓设备矩阵.app。\n\n开启后返回 App 重试；若仍提示未授权，请退出并重新打开 App。若开关已开启但仍未授权，请移除旧条目，重新添加上述路径的应用并开启。"

type AccessibilityPermissionRequiredError struct {
	Operation string
}

func (e *AccessibilityPermissionRequiredError) Error() string {
	return e.Operation + "需要 macOS 辅助功能权限。" + AccessibilityPermissionGuide
}

func IsAccessibilityPermissionRequired(err error) bool {
	var required *AccessibilityPermissionRequiredError
	return errors.As(err, &required)
}

type AccessibilityPermissionPromptedError struct {
	Operation string
}

func (e *AccessibilityPermissionPromptedError) Error() string {
	if e == nil || e.Operation == "" {
		return "已请求 macOS 辅助功能授权，授权后请重试"
	}
	return e.Operation + "需要 macOS 辅助功能授权。已打开系统授权提示，授权后请重试"
}

func IsAccessibilityPermissionPrompted(err error) bool {
	var prompted *AccessibilityPermissionPromptedError
	return errors.As(err, &prompted)
}

func nativeWindowAccessUnavailable() bool {
	return runtime.GOOS == "darwin" && !accessibilityTrustedNative()
}
