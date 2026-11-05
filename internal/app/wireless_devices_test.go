package app

import "testing"

func TestWirelessDiscoveryMergesConnectionsAndKeepsPairingUnknown(t *testing.T) {
	services := []wirelessService{
		{Name: "adb-DEMO001-guid", Address: "192.168.1.20:41000", DisplayName: "广播名称"},
		{Name: "adb-DEMO001-guid", Address: "192.168.1.20:41000", DisplayName: "广播名称"},
		{Name: "adb-DEMO002-guid", Address: "192.168.1.21:42000", DisplayName: "新手机"},
		{Name: "adb-DEMO003-guid", Address: "192.168.1.22:43000", DisplayName: "已记录手机"},
	}
	active := []ActiveDevice{
		{Serial: "192.168.1.20:41000", State: "device", Details: map[string]string{"model": "Demo_Phone"}},
		{Serial: "192.168.1.23:44000", State: "device"},
		{Serial: "emulator-5554", State: "device", IsEmulator: true},
		{Serial: "USB-DEMO", State: "device"},
		{Serial: "192.168.1.24:45000", State: "offline"},
	}
	got := wirelessDeviceList(services, active, map[string]string{"old-address": "DEMO003"})
	if len(got) != 4 {
		t.Fatalf("wrong discovery count: %+v", got)
	}
	for i, device := range got {
		if i < 2 && !device.Connected {
			t.Fatalf("connected devices must appear first: %+v", got)
		}
		switch device.Address {
		case "192.168.1.20:41000":
			if device.Name != "Demo_Phone" || device.Status != "已连接" {
				t.Fatalf("name or status differs from device list: %+v", device)
			}
		case "192.168.1.21:42000":
			if device.Status != "配对状态待确认" {
				t.Fatalf("discovery claimed pairing: %+v", device)
			}
		case "192.168.1.22:43000":
			if device.Status != "已记录 · 待连接" {
				t.Fatalf("remembered identity missing: %+v", device)
			}
		}
	}
}
