package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// QRPairingSession is short-lived and never persisted or written to logs.
type QRPairingSession struct {
	service  string
	password string
	expires  time.Time
}

func NewQRPairingSession() (*QRPairingSession, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	return &QRPairingSession{service: "adm-" + hex.EncodeToString(random[:8]), password: hex.EncodeToString(random[8:]), expires: time.Now().Add(2 * time.Minute)}, nil
}

func (s *QRPairingSession) Payload() string {
	return "WIFI:T:ADB;S:" + s.service + ";P:" + s.password + ";;"
}

func qrPairingAddress(output, service string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "_adb-tls-pairing._tcp" && strings.TrimSuffix(fields[0], "._adb-tls-pairing._tcp") == service {
			if address, err := WirelessAddress(fields[2]); err == nil {
				return address
			}
		}
	}
	return ""
}

func (a *App) GUIWaitQRPairing(parent context.Context, s *QRPairingSession) error {
	if s == nil || s.service == "" || s.password == "" {
		return fmt.Errorf("二维码会话无效，请重新生成")
	}
	ctx, cancel := context.WithDeadline(parent, s.expires)
	defer cancel()
	path, err := a.tools.Path(toolADB)
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		probe, stop := context.WithTimeout(ctx, 3*time.Second)
		out, _ := exec.CommandContext(probe, path, "mdns", "services").Output()
		stop()
		address := qrPairingAddress(string(out), s.service)
		if address == "" {
			address = findQRPairingServiceNative(ctx, s.service)
		}
		if address != "" && ctx.Err() == nil {
			pairing, stopPairing := context.WithTimeout(ctx, 30*time.Second)
			cmd := exec.CommandContext(pairing, path, "pair", address)
			cmd.Stdin = strings.NewReader(s.password + "\n")
			output, pairErr := cmd.CombinedOutput()
			stopPairing()
			if ctx.Err() != nil {
				break
			}
			safeOutput := strings.ReplaceAll(string(output), s.password, "******")
			if pairErr != nil || !strings.Contains(strings.ToLower(safeOutput), "successfully paired to ") {
				return fmt.Errorf("扫码配对失败：%s。请重新生成二维码后再扫描", strings.TrimSpace(safeOutput))
			}
			// ADB automatically connects to paired devices advertised over mDNS.
			return nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	return fmt.Errorf("二维码已过期或未发现手机。请重新生成，并确认电脑与手机处于同一可互访网络，允许本地网络权限")
}
