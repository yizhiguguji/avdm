package app

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// WirelessAddress requires an explicit port: pairing and connection ports differ.
func WirelessAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	host, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" || strings.ContainsAny(host, " /\\\t\r\n") || strings.HasPrefix(host, "-") {
		return "", fmt.Errorf("请输入手机显示的 IP 地址和端口，例如 192.168.1.20:37000；IPv6 使用 [地址]:端口")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("端口必须为 1–65535")
	}
	return net.JoinHostPort(host, strconv.Itoa(n)), nil
}

type wirelessRunner func(args []string, input string) (string, error)

func wirelessOperation(operation, address, code string, run wirelessRunner) error {
	address, err := WirelessAddress(address)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	input := ""
	switch operation {
	case "pair":
		if len(code) != 6 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fmt.Errorf("请输入手机当前显示的 6 位配对码")
		}
		input = code + "\n"
	case "connect", "disconnect":
	default:
		return fmt.Errorf("不支持的无线操作")
	}
	out, err := run([]string{operation, address}, input)
	if code != "" {
		out = strings.ReplaceAll(out, code, "******")
	}
	if err != nil {
		return wirelessFailure(operation, out)
	}
	lower := strings.ToLower(out)
	// ADB may print daemon startup diagnostics before the result line.
	lines := strings.Split(lower, "\n")
	success := false
	switch operation {
	case "pair":
		success = strings.Contains(lower, "successfully paired to ")
	case "connect":
		for _, line := range lines {
			line = strings.TrimSpace(line)
			success = success || strings.HasPrefix(line, "connected to ") || strings.HasPrefix(line, "already connected to ")
		}
	case "disconnect":
		for _, line := range lines {
			success = success || strings.HasPrefix(strings.TrimSpace(line), "disconnected ")
		}
	}
	if !success {
		return wirelessFailure(operation, out)
	}
	return nil
}

// ADB can report connection failures with exit status zero.
func wirelessFailure(operation, output string) error {
	lower := strings.ToLower(output)
	hint := "请确认电脑与手机在同一网络，手机已开启无线调试，并使用手机当前显示的地址和端口。配对端口与连接端口不同。"
	switch {
	case strings.Contains(lower, "no route to host"), strings.Contains(lower, "network is unreachable"), strings.Contains(lower, "operation not permitted"), strings.Contains(lower, "permission denied"):
		hint = "电脑当前无法访问该网络地址。请确认手机与电脑连接同一可互访的网络，检查 VPN、路由器访客网络或客户端隔离设置。\nmacOS 用户还需检查「系统设置 → 隐私与安全性 → 本地网络」中的「安卓设备矩阵」权限；如系统弹出访问提示，请允许后重试。此错误不能据此判断为端口填写错误。"
	case strings.Contains(lower, "connection refused"):
		hint = "手机地址可访问，但该端口拒绝连接。请保持手机无线调试开启，并重新读取无线调试主页当前的「IP 地址和端口」；端口可能已变化。若正在配对，请重新打开配对码弹窗，使用其中的配对地址。"
	case strings.Contains(lower, "timed out"), strings.Contains(lower, "timeout"), strings.Contains(lower, "超时"):
		hint = "连接等待超时。请确认手机保持联网、无线调试开启，核对当前地址，并检查防火墙或路由器是否阻止设备互访。"
	case strings.Contains(lower, "failed to authenticate"), strings.Contains(lower, "failed to pair"), strings.Contains(lower, "pairing failed"):
		hint = "配对或认证未通过。请在手机重新打开「使用配对码配对设备」，使用新的配对地址和 6 位配对码；配对成功后再用主页连接地址连接。"
	}
	if strings.TrimSpace(output) == "" {
		output = "ADB 未返回成功结果"
	}
	return fmt.Errorf("无线%s未成功：%s\n%s", wirelessActionLabel(operation), output, hint)
}

func wirelessActionLabel(operation string) string {
	switch operation {
	case "pair":
		return "配对"
	case "disconnect":
		return "断开"
	default:
		return "连接"
	}
}

func (a *App) GUIWirelessOperation(operation, address, code string) error {
	err := wirelessOperation(operation, address, code, func(args []string, input string) (string, error) {
		path, err := a.tools.Path(toolADB)
		if err != nil {
			return err.Error(), err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, args...)
		// Keep the temporary pairing code out of process arguments and diagnostic logs.
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			return "ADB 等待超时，请重新打开手机配对弹窗或检查网络", ctx.Err()
		}
		return strings.TrimSpace(string(out)), err
	})
	if err != nil {
		return err
	}
	address, _ = WirelessAddress(address)
	if operation == "connect" {
		state, stateErr := a.runToolOutput(toolADB, 3*time.Second, "-s", address, "get-state")
		if stateErr != nil || strings.TrimSpace(state) != "device" {
			return fmt.Errorf("ADB 已接受连接请求，但设备尚未就绪。请保持手机无线调试开启后重新扫描；当前状态：%s", state)
		}
		a.rememberWirelessDevice(address)
	}
	if operation == "disconnect" {
		a.forgetWirelessDevice(address)
	}
	return nil
}

func IsWirelessSerial(serial string) bool {
	if strings.HasPrefix(serial, "emulator-") {
		return false
	}
	if strings.HasSuffix(serial, "._adb-tls-connect._tcp") || strings.HasSuffix(serial, "._adb-tls-connect._tcp.") {
		return true
	}
	_, err := WirelessAddress(serial)
	return err == nil
}
