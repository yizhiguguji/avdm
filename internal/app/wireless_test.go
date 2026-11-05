package app

import (
	"errors"
	"strings"
	"testing"
)

func TestWirelessAddressAndSerial(t *testing.T) {
	for _, address := range []string{"192.168.1.20:37000", "[::1]:1234", "phone.local:5555"} {
		if _, err := WirelessAddress(address); err != nil {
			t.Errorf("%s: %v", address, err)
		}
	}
	for _, address := range []string{"", "host", "host:0", "host:65536", "-option:1234", "host:abc", "a b:1234", ":5555"} {
		if _, err := WirelessAddress(address); err == nil {
			t.Errorf("accepted %q", address)
		}
	}
	for _, serial := range []string{"192.168.1.20:37000", "adb-demo._adb-tls-connect._tcp"} {
		if !IsWirelessSerial(serial) {
			t.Errorf("wireless serial unrecognized %s", serial)
		}
	}
	if IsWirelessSerial("emulator-5554") || IsWirelessSerial("demo-usb") {
		t.Fatal("non-wireless serial misclassified")
	}
}

func TestWirelessResponsesAndCodePrivacy(t *testing.T) {
	for _, tc := range []struct {
		op, out string
		failure bool
	}{
		{"connect", "connected to 192.168.1.20:37000", false},
		{"connect", "already connected to 192.168.1.20:37000", false},
		{"connect", "failed to connect: refused", true},
		{"connect", "", true},
		{"pair", "Enter pairing code: Successfully paired to 192.168.1.20:37000", false},
		{"pair", "Failed to pair", true},
		{"disconnect", "disconnected 192.168.1.20:37000", false},
	} {
		err := wirelessOperation(tc.op, "192.168.1.20:37000", "123456", func(args []string, input string) (string, error) {
			if strings.Contains(strings.Join(args, " "), "123456") {
				t.Fatal("code leaked in process args")
			}
			if tc.op == "pair" && input != "123456\n" {
				t.Fatalf("invalid pairing stdin %q", input)
			}
			return tc.out, nil
		})
		if (err != nil) != tc.failure {
			t.Errorf("%s %s: %v", tc.op, tc.out, err)
		}
	}
	err := wirelessOperation("pair", "192.168.1.20:37000", "123456", func([]string, string) (string, error) { return "rejected 123456", errors.New("exit") })
	if err == nil || strings.Contains(err.Error(), "123456") {
		t.Fatalf("code leaked: %v", err)
	}
	called := false
	err = wirelessOperation("pair", "192.168.1.20:37000", "bad", func([]string, string) (string, error) { called = true; return "", nil })
	if err == nil || called {
		t.Fatal("invalid code reached adb")
	}
}

func TestWirelessFailureExplainsActualNetworkCause(t *testing.T) {
	cases := []struct{ output, want string }{
		{"failed to connect: No route to host", "本地网络"},
		{"Network is unreachable", "同一可互访的网络"},
		{"Connection refused", "该端口拒绝连接"},
		{"Connection timed out", "连接等待超时"},
		{"Failed to authenticate", "配对或认证未通过"},
	}
	for _, tc := range cases {
		// Check both nonzero and zero-exit ADB failures use the same diagnosis.
		for _, runErr := range []error{nil, errors.New("exit status 1")} {
			err := wirelessOperation("connect", "192.168.1.20:37000", "", func([]string, string) (string, error) { return tc.output, runErr })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%q: %v", tc.output, err)
			}
		}
	}
}

func TestWirelessRecoveryMatchesHardwareIdentityAndRejectsAmbiguity(t *testing.T) {
	services := parseWirelessServices("List of discovered mdns services\nadb-DEMO001-guid _adb-tls-connect._tcp 192.168.1.20:41000\nadb-OTHER-guid _adb-tls-connect._tcp 192.168.1.21:42000\nadb-DEMO001-pair _adb-tls-pairing._tcp 192.168.1.20:43000\n")
	service, ok := wirelessServiceForDevice(services, "DEMO001")
	if !ok || service.Address != "192.168.1.20:41000" {
		t.Fatalf("wrong device: %+v %v", service, ok)
	}
	if _, ok := wirelessServiceForDevice(services, "DEMO"); ok {
		t.Fatal("partial identity matched another device")
	}
	if _, ok := wirelessServiceForDevice(services, ""); ok {
		t.Fatal("unknown identity auto-connected")
	}
	services = append(services, wirelessService{Name: "adb-DEMO001-second", Address: "192.168.1.22:44000"})
	if _, ok := wirelessServiceForDevice(services, "DEMO001"); ok {
		t.Fatal("ambiguous discovery auto-connected")
	}
}
