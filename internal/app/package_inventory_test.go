package app

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestUserInstalledPackagesUsePositiveMetadata(t *testing.T) {
	var dump strings.Builder
	dump.WriteString("Packages:\n")
	for _, tc := range []struct{ name, initiator, source, reason string }{
		{"com.adb", "com.android.shell", "1", "0"},
		{"com.manual", "some.installer", "4", "0"},
		{"com.local", "some.installer", "3", "0"},
		{"com.user", "some.store", "0", "4"},
		{"com.background", "some.store", "0", "4"},
		{"com.vendor", "any.vendor.provisioner", "0", "0"},
		{"com.setup", "some.store", "0", "3"},
		{"com.policy", "enterprise.installer", "0", "1"},
		{"com.unknown", "null", "0", "0"},
	} {
		fmt.Fprintf(&dump, "  Package [%s] (abc):\n    initiatingPackageName=%s\n    packageSource=%s\n    User 0: installed=true\n      installReason=%s\n    User 10: installed=true\n      installReason=4\n", tc.name, tc.initiator, tc.source, tc.reason)
	}
	dump.WriteString("  Package [com.removed] (abc):\n    initiatingPackageName=com.android.shell\n    User 0: installed=false\n      installReason=4\n")
	dump.WriteString("Hidden system packages:\n  Package [com.hidden] (abc):\n    User 0: installed=true\n      installReason=4\n")
	got, err := userInstalledPackageNames(dump.String(), 0, map[string]bool{"com.user": true, "com.vendor": true})
	want := map[string]bool{"com.adb": true, "com.manual": true, "com.local": true, "com.user": true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if _, err := userInstalledPackageNames("Permission Denial", 0, nil); err == nil {
		t.Fatal("invalid dump accepted")
	}
}

// Read-only verification against the user's device; never installs/uninstalls.
func TestLiveUserInstalledPackageInventory(t *testing.T) {
	serial := os.Getenv("ADM_LIVE_PACKAGE_SERIAL")
	if serial == "" {
		t.Skip("set ADM_LIVE_PACKAGE_SERIAL for read-only inventory")
	}
	dump, err := exec.Command("adb", "-s", serial, "shell", "dumpsys", "package", "packages").Output()
	if err != nil {
		t.Fatal(err)
	}
	launchers, err := exec.Command("adb", "-s", serial, "shell", "cmd", "package", "query-activities", "--brief", "--components", "--user", "0", "-a", "android.intent.action.MAIN", "-c", "android.intent.category.LAUNCHER").Output()
	if err != nil {
		t.Fatal(err)
	}
	users, err := userInstalledPackageNames(string(dump), 0, launcherPackageNames(string(launchers)))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("adb", "-s", serial, "shell", "pm", "list", "packages", "-3", "--user", "0").Output()
	if err != nil {
		t.Fatal(err)
	}
	var result []string
	for _, pkg := range packageLinesToNames(string(out)) {
		if users[pkg] {
			result = append(result, pkg)
		}
	}
	t.Logf("user installation records: %v", result)
	if expected := strings.TrimSpace(os.Getenv("ADM_LIVE_EXPECTED_PACKAGE")); expected != "" && !users[expected] {
		t.Fatalf("expected installed test app missing: %s", expected)
	}
	if users["com.google.android.safetycore"] {
		t.Fatal("background store component classified as user app")
	}
	if users["com.samsung.android.app.notes"] {
		t.Fatal("OEM provisioning classified as user-installed")
	}
}
