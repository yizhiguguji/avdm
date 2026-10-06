package app

import "errors"

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
