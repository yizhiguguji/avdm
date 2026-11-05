package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQRPairingSessionsAreUniqueAndUseAndroidPayload(t *testing.T) {
	a, err := NewQRPairingSession()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewQRPairingSession()
	if err != nil {
		t.Fatal(err)
	}
	if a.service == b.service || a.password == b.password {
		t.Fatal("reused pairing credentials")
	}
	if !strings.HasPrefix(a.Payload(), "WIFI:T:ADB;S:") || !strings.HasSuffix(a.Payload(), ";;") || !strings.Contains(a.Payload(), ";P:"+a.password+";;") {
		t.Fatal("invalid Android pairing payload")
	}
	if time.Until(a.expires) <= 0 || time.Until(a.expires) > 2*time.Minute {
		t.Fatal("invalid session expiry")
	}
}

func TestQRPairingDiscoveryMatchesOnlySession(t *testing.T) {
	output := "other _adb-tls-pairing._tcp 192.168.1.20:40000\nadm-test _adb-tls-connect._tcp 192.168.1.21:40001\nadm-test _adb-tls-pairing._tcp 192.168.1.22:40002\n"
	if got := qrPairingAddress(output, "adm-test"); got != "192.168.1.22:40002" {
		t.Fatal(got)
	}
	if got := qrPairingAddress(output, "missing"); got != "" {
		t.Fatal("matched an unrelated phone")
	}
}

func TestQRPairingUsesStdinAndRedactsFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "adb")
			ending := "echo 'Successfully paired to 192.168.1.20:40000'"
			if fail {
				ending = `echo "Failed to pair $secret"; exit 1`
			}
			script := "#!/bin/sh\ncase \"$1\" in\nmdns) echo 'adm-test _adb-tls-pairing._tcp 192.168.1.20:40000';;\npair) [ \"$#\" = 2 ] || exit 2; read -r secret; [ \"$secret\" = 'temporary-secret' ] || exit 3; " + ending + ";;\nesac\n"
			if err := os.WriteFile(path, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			app := &App{tools: &ToolResolver{statuses: map[string]ToolStatus{toolADB: {Available: true, Path: path}}}}
			s := &QRPairingSession{service: "adm-test", password: "temporary-secret", expires: time.Now().Add(time.Minute)}
			err := app.GUIWaitQRPairing(context.Background(), s)
			if fail {
				if err == nil || strings.Contains(err.Error(), s.password) {
					t.Fatalf("unredacted or absent failure: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := app.GUIWaitQRPairing(ctx, s); err != context.Canceled {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}
