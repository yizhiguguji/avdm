package app

import (
	"context"
	"net"
	"strings"
	"time"
)

type wirelessService struct{ Name, Address, DisplayName string }

func parseWirelessServices(output string) []wirelessService {
	var result []wirelessService
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[1] != "_adb-tls-connect._tcp" {
			continue
		}
		if address, err := WirelessAddress(fields[2]); err == nil {
			result = append(result, wirelessService{Name: fields[0], Address: address})
		}
	}
	return result
}

func wirelessServiceForDevice(services []wirelessService, identity string) (wirelessService, bool) {
	if identity == "" {
		return wirelessService{}, false
	}
	var matches []wirelessService
	seen := map[string]bool{}
	for _, service := range services {
		// Match the device's hardware serial, never just an IP that may be reused.
		if strings.HasPrefix(service.Name, "adb-"+identity+"-") && !seen[service.Address] {
			matches = append(matches, service)
			seen[service.Address] = true
		}
	}
	if len(matches) != 1 {
		return wirelessService{}, false
	}
	return matches[0], true
}

func (a *App) rememberWirelessDevice(address string) {
	if !IsWirelessSerial(address) {
		return
	}
	a.mu.Lock()
	known := a.cfg != nil && a.cfg.WirelessDevices[address] != ""
	a.mu.Unlock()
	if known {
		return
	}
	identity, err := a.runToolOutput(toolADB, 3*time.Second, "-s", address, "shell", "getprop", "ro.serialno")
	identity = strings.TrimSpace(identity)
	if err != nil || identity == "" || strings.ContainsAny(identity, " \r\n\t") {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.WirelessDevices == nil {
		a.cfg.WirelessDevices = map[string]string{}
	}
	a.cfg.WirelessDevices[address] = identity
	_ = saveConfig(a.cfg)
}

func (a *App) forgetWirelessDevice(address string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return
	}
	delete(a.cfg.WirelessDevices, address)
	_ = saveConfig(a.cfg)
}

// A scan can repair a remembered Wi-Fi transport after Android changes its port.
// It never pairs a new device, restarts shared ADB, or guesses by IP address.
func (a *App) recoverWirelessEntries(entries []DeviceEntry) bool {
	var offline []string
	ready := map[string]bool{}
	for _, entry := range entries {
		if entry.Active == nil || entry.Active.IsEmulator || !IsWirelessSerial(entry.Active.Serial) {
			continue
		}
		if entry.Active.State == "device" {
			ready[entry.Active.Serial] = true
			a.rememberWirelessDevice(entry.Active.Serial)
		} else if entry.Active.State == "offline" {
			offline = append(offline, entry.Active.Serial)
		}
	}
	if len(offline) == 0 {
		return false
	}
	a.mu.Lock()
	if time.Since(a.wirelessRecoveryLast) < 10*time.Second {
		a.mu.Unlock()
		return false
	}
	a.wirelessRecoveryLast = time.Now()
	known := map[string]string{}
	for _, address := range offline {
		if a.cfg != nil {
			known[address] = a.cfg.WirelessDevices[address]
		}
	}
	a.mu.Unlock()
	needed := false
	for _, identity := range known {
		needed = needed || identity != ""
	}
	if !needed {
		return false
	}
	output, _ := a.runToolOutput(toolADB, 3*time.Second, "mdns", "services")
	services := parseWirelessServices(output)
	if len(services) == 0 {
		services = discoverWirelessServicesNative()
	}
	changed := false
	for _, old := range offline {
		service, ok := wirelessServiceForDevice(services, known[old])
		if !ok {
			continue
		}
		if !ready[service.Address] {
			if err := a.GUIWirelessOperation("connect", service.Address, ""); err != nil {
				continue
			}
		}
		identity, err := a.runToolOutput(toolADB, 3*time.Second, "-s", service.Address, "shell", "getprop", "ro.serialno")
		if err != nil || strings.TrimSpace(identity) != known[old] {
			continue
		}
		if old != service.Address {
			_, _ = a.runToolOutput(toolADB, 3*time.Second, "disconnect", old)
			a.forgetWirelessDevice(old)
		}
		changed = true
		a.recordMirrorEvent("DONE", "无线设备已根据 Bonjour 广播恢复连接；请重新打开该设备的外部窗。")
	}
	return changed
}

// Resolve a Bonjour SRV target to an IPv4 address for an ordinary ADB serial.
func wirelessBonjourAddress(ctx context.Context, host, port string) string {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.IP.To4(); v4 != nil {
			address, _ := WirelessAddress(net.JoinHostPort(v4.String(), port))
			return address
		}
	}
	return ""
}
