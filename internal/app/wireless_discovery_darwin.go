package app

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Use the system Bonjour browser when an ADB daemon's own mDNS cache is empty.
// dns-sd runs continuously; the deadline bounds a single discovery pass.
func discoverWirelessServicesNative() []wirelessService {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "/usr/bin/dns-sd", "-Z", "_adb-tls-connect._tcp", "local.").Output()
	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer resolveCancel()
	return parseWirelessBonjourZone(string(out), func(host, port string) string {
		return wirelessBonjourAddress(resolveCtx, host, port)
	})
}

func parseWirelessBonjourZone(output string, resolve func(string, string) string) []wirelessService {
	var services []wirelessService
	names := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "TXT" {
			continue
		}
		for _, match := range wirelessBonjourNamePattern.FindAllStringSubmatch(line, -1) {
			if name, err := strconv.Unquote(`"` + match[2] + `"`); err == nil && name != "" && (names[fields[0]] == "" || match[1] == "given_name") {
				names[fields[0]] = name
			}
		}
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[1] != "SRV" || fields[2] != "0" || fields[3] != "0" || !strings.HasSuffix(fields[0], "._adb-tls-connect._tcp") {
			continue
		}
		if len(services) >= 16 {
			break
		}
		name := strings.TrimSuffix(fields[0], "._adb-tls-connect._tcp")
		if address := resolve(fields[5], fields[4]); address != "" {
			services = append(services, wirelessService{Name: name, Address: address, DisplayName: names[fields[0]]})
		}
	}
	return services
}

var wirelessBonjourNamePattern = regexp.MustCompile(`"(given_name|name)=([^"\\]*(?:\\.[^"\\]*)*)"`)
