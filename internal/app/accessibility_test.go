package app

import (
	"errors"
	"fmt"
	"testing"
)

func TestAccessibilityPermissionRequiredSurvivesMirrorErrorContext(t *testing.T) {
	permission := &AccessibilityPermissionRequiredError{Operation: "平铺外部窗口"}
	wrapped := fmt.Errorf("已打开 2 个实时镜像，但排列外部窗口失败：%w", permission)
	if !IsAccessibilityPermissionRequired(wrapped) {
		t.Fatal("mirror error context must preserve the permission error for GUI guidance")
	}
	if IsAccessibilityPermissionRequired(errors.New("外部窗口未找到")) {
		t.Fatal("unrelated window errors must not trigger permission guidance")
	}
}
