package app

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func findQRPairingServiceNative(parent context.Context, service string) string {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	output, _ := exec.CommandContext(ctx, "/usr/bin/dns-sd", "-Z", "_adb-tls-pairing._tcp", "local.").Output()
	resolve, stop := context.WithTimeout(parent, 2*time.Second)
	defer stop()
	return qrPairingBonjourAddress(string(output), service, func(host, port string) string { return wirelessBonjourAddress(resolve, host, port) })
}

func qrPairingBonjourAddress(output, service string, resolve func(string, string) string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 6 && fields[0] == service+"._adb-tls-pairing._tcp" && fields[1] == "SRV" {
			return resolve(fields[5], fields[4])
		}
	}
	return ""
}
