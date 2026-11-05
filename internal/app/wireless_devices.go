package app

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type GUIWirelessDevice struct {
	Name      string
	Address   string
	Status    string
	Connected bool
}

func (a *App) GUIWirelessDevices() ([]GUIWirelessDevice, error) {
	// The native browser includes TXT names and also works when ADB's cache is empty.
	services := discoverWirelessServicesNative()
	var discoveryErr error
	if len(services) == 0 {
		output, err := a.runToolOutput(toolADB, 4*time.Second, "mdns", "services")
		services = parseWirelessServices(output)
		discoveryErr = err
	}
	output, err := a.runToolOutput(toolADB, 4*time.Second, "devices", "-l")
	if err != nil {
		return nil, fmt.Errorf("读取无线连接状态失败：%w", err)
	}
	a.mu.Lock()
	known := map[string]string{}
	if a.cfg != nil {
		for address, identity := range a.cfg.WirelessDevices {
			known[address] = identity
		}
	}
	a.mu.Unlock()
	devices := wirelessDeviceList(services, parseADBDevices(output), known)
	if len(devices) == 0 && discoveryErr != nil {
		return nil, fmt.Errorf("无线设备发现失败：%w", discoveryErr)
	}
	return devices, nil
}

func wirelessDeviceList(services []wirelessService, active []ActiveDevice, known map[string]string) []GUIWirelessDevice {
	byAddress := map[string]GUIWirelessDevice{}
	serviceAddresses := map[string]string{}
	serviceAliases := map[string]string{}
	for _, service := range services {
		address, err := WirelessAddress(service.Address)
		if err != nil {
			continue
		}
		discoveredAddress := address
		serviceName := wirelessConnectServiceName(service.Name)
		if existing := serviceAddresses[serviceName]; serviceName != "" && existing != "" {
			address = existing
		} else if serviceName != "" {
			serviceAddresses[serviceName] = address
		}
		serviceAliases[discoveredAddress] = address
		name := strings.TrimSpace(service.DisplayName)
		if name == "" {
			name = service.Name
		}
		status := "配对状态待确认"
		for _, identity := range known {
			if identity != "" && strings.HasPrefix(service.Name, "adb-"+identity+"-") {
				status = "已记录 · 待连接"
				break
			}
		}
		byAddress[address] = GUIWirelessDevice{Name: name, Address: address, Status: status}
	}
	for _, device := range active {
		if device.IsEmulator || device.State != "device" || !IsWirelessSerial(device.Serial) {
			continue
		}
		address := device.Serial
		if resolved := serviceAddresses[wirelessConnectServiceName(device.Serial)]; resolved != "" {
			address = resolved
		}
		if canonical := serviceAliases[address]; canonical != "" {
			address = canonical
		}
		entry := byAddress[address]
		if model := device.Details["model"]; model != "" {
			entry.Name = model
		}
		if entry.Name == "" {
			entry.Name = "Android 手机"
		}
		entry.Address = address
		entry.Status = "已连接"
		entry.Connected = true
		byAddress[address] = entry
	}
	result := make([]GUIWirelessDevice, 0, len(byAddress))
	for _, device := range byAddress {
		result = append(result, device)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Connected != result[j].Connected {
			return result[i].Connected
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Address < result[j].Address
	})
	return result
}

// Match the full service instance, never the model name or just an IP host.
func wirelessConnectServiceName(value string) string {
	return strings.TrimSuffix(strings.TrimSuffix(value, "."), "._adb-tls-connect._tcp")
}
