package app

import (
	"fmt"
	"strconv"
	"strings"
)

func launcherPackageNames(output string) map[string]bool {
	packages := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		pkg, activity, ok := strings.Cut(strings.TrimSpace(line), "/")
		if ok && pkg != "" && activity != "" && !strings.ContainsAny(pkg, " \t:") {
			packages[pkg] = true
		}
	}
	return packages
}

// Use positive installation metadata instead of an OEM package/installer denylist.
// INSTALL_REASON_USER is 4; PACKAGE_SOURCE_LOCAL_FILE/DOWNLOADED_FILE are 3/4.
// Store installs must also expose a launcher activity, excluding background
// components silently delivered by stores with INSTALL_REASON_USER.
// ADB installs commonly have reason UNKNOWN but identify com.android.shell as
// the initiator. Missing metadata is not evidence of a user installation.
func userInstalledPackageNames(dump string, userID int, launchable map[string]bool) (map[string]bool, error) {
	result := map[string]bool{}
	var pkg, initiator, source, reason string
	var installed, inUser, inPackages, seenPackages bool
	flush := func() {
		if pkg != "" && installed && ((reason == "4" && launchable[pkg]) || initiator == "com.android.shell" || source == "3" || source == "4") {
			result[pkg] = true
		}
	}
	for _, line := range strings.Split(dump, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Packages:" {
			inPackages, seenPackages = true, true
			continue
		}
		if !inPackages {
			continue
		}
		if trimmed != "" && !strings.HasPrefix(line, " ") {
			flush()
			pkg = ""
			inPackages = false
			continue
		}
		if strings.HasPrefix(line, "  Package [") {
			flush()
			pkg, _, _ = strings.Cut(strings.TrimPrefix(line, "  Package ["), "]")
			initiator, source, reason = "", "", ""
			installed, inUser = false, false
			continue
		}
		if strings.HasPrefix(trimmed, "initiatingPackageName=") {
			initiator = strings.TrimPrefix(trimmed, "initiatingPackageName=")
		}
		if strings.HasPrefix(trimmed, "packageSource=") {
			source = strings.TrimPrefix(trimmed, "packageSource=")
		}
		if strings.HasPrefix(trimmed, "User ") {
			inUser = strings.HasPrefix(trimmed, "User "+strconv.Itoa(userID)+":")
			if inUser {
				for _, field := range strings.Fields(trimmed) {
					if field == "installed=true" {
						installed = true
					}
				}
			}
		}
		if inUser && strings.HasPrefix(trimmed, "installReason=") {
			reason = strings.TrimPrefix(trimmed, "installReason=")
		}
	}
	flush()
	if !seenPackages {
		return nil, fmt.Errorf("无法读取用户安装记录；未展示未经确认的应用，请重新刷新或取消“仅用户安装的应用”筛选")
	}
	return result, nil
}
