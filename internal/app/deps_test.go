package app

import (
	"strings"
	"testing"
)

func TestDependencyFailureDetailsKeepsDownloadCause(t *testing.T) {
	output := "[adm-deps] go already installed\n" + strings.Repeat("progress\n", 40) + "Warning: Failed to download package!\n[adm-deps] ERROR: Android SDK component failed: platform-tools (exit 1)"
	got := dependencyFailureDetails(output)
	if strings.Contains(got, "go already installed") || !strings.Contains(got, "Failed to download package!") || !strings.Contains(got, "platform-tools (exit 1)") {
		t.Fatalf("misleading failure details: %s", got)
	}
}
