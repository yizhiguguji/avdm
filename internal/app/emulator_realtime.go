package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	emugrpcpb "adm/internal/emugrpcpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GUIEmulatorRealtimeTarget struct {
	Available bool
	EntryKey  string
	Serial    string
	AVDName   string
	Port      int
	Token     string
	Reason    string
}

type GUIEmulatorRealtimeFrame struct {
	PNG       []byte
	Width     int
	Height    int
	Seq       uint32
	Timestamp time.Time
	Error     string
}

func (a *App) GUIEmulatorRealtimeTarget(entryKey string) (GUIEmulatorRealtimeTarget, error) {
	entry, ok, err := a.findDeviceEntryByKey(entryKey)
	if err != nil {
		return GUIEmulatorRealtimeTarget{}, err
	}
	if !ok {
		return GUIEmulatorRealtimeTarget{}, fmt.Errorf("未找到设备：%s", entryKey)
	}
	return a.GUIEmulatorRealtimeTargetForEntry(entry), nil
}

// GUIEmulatorRealtimeTargetForEntry probes the realtime (gRPC) target using an
// already-resolved device entry, skipping a full device re-enumeration. The
// control center probes every card with this, so it must stay cheap and is
// safe to call concurrently off the UI thread.
func (a *App) GUIEmulatorRealtimeTargetForEntry(entry DeviceEntry) GUIEmulatorRealtimeTarget {
	target := GUIEmulatorRealtimeTarget{EntryKey: entry.Key}
	if entry.Active == nil || entry.Active.State != "device" {
		target.Reason = "设备不是 state=device"
		return target
	}
	if !entry.Active.IsEmulator {
		target.Reason = "真机暂不支持内嵌画面，请使用 scrcpy 窗口"
		return target
	}
	target.Serial = entry.Active.Serial
	target.AVDName = strings.TrimSpace(entry.Active.AVDName)
	if target.AVDName == "" && entry.AVD != nil {
		target.AVDName = entry.AVD.Name
	}
	if target.AVDName == "" {
		target.Reason = "未识别 AVD 名称"
		return target
	}
	for _, proc := range runningEmulatorProcessesForAVD(target.AVDName) {
		discovery := emulatorDiscoveryForPID(proc.PID)
		if port := parsePositiveInt(discovery["grpc.port"]); port > 0 && isTCPPortOpen(port) {
			target.Port = port
			target.Token = discovery["grpc.token"]
			target.Available = true
			return target
		}
		if proc.GRPCPort > 0 {
			target.Port = proc.GRPCPort
			target.Token = discovery["grpc.token"]
			target.Available = true
			return target
		}
	}
	if port := defaultEmulatorGRPCPortForSerial(target.Serial); port > 0 && isTCPPortOpen(port) {
		target.Port = port
		target.Token = emulatorRealtimeTokenForPort(port)
		target.Available = true
		return target
	}
	target.Reason = "该模拟器启动时未启用 gRPC；请用 安卓设备矩阵 重新启动该模拟器后使用内嵌画面"
	return target
}

func (a *App) GUIStreamEmulatorRealtime(entryKey string, width, height int) (<-chan GUIEmulatorRealtimeFrame, func(), error) {
	target, err := a.GUIEmulatorRealtimeTarget(entryKey)
	if err != nil {
		return nil, nil, err
	}
	return a.streamEmulatorRealtimeForTarget(target, width, height)
}

// GUIStreamEmulatorRealtimeForEntry starts a realtime stream using an
// already-resolved entry, avoiding a device re-enumeration on the UI thread.
func (a *App) GUIStreamEmulatorRealtimeForEntry(entry DeviceEntry, width, height int) (<-chan GUIEmulatorRealtimeFrame, func(), error) {
	return a.streamEmulatorRealtimeForTarget(a.GUIEmulatorRealtimeTargetForEntry(entry), width, height)
}

func (a *App) streamEmulatorRealtimeForTarget(target GUIEmulatorRealtimeTarget, width, height int) (<-chan GUIEmulatorRealtimeFrame, func(), error) {
	if !target.Available {
		return nil, nil, errors.New(target.Reason)
	}
	width, height = normalizeRealtimeScreenshotSize(width, height)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = emulatorRealtimeOutgoingContext(ctx, target.Token)
	conn, client, err := emulatorRealtimeClient(ctx, target.Port)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	format := &emugrpcpb.ImageFormat{
		Format: emugrpcpb.ImageFormat_PNG,
		Width:  uint32(width),
		Height: uint32(height),
	}
	frames := make(chan GUIEmulatorRealtimeFrame, 1)
	stop := func() {
		cancel()
	}
	go func() {
		defer close(frames)
		defer conn.Close()
		if err := streamRealtimeFrames(ctx, client, format, frames); err != nil {
			if ctx.Err() != nil {
				return
			}
			sendRealtimeFrame(frames, GUIEmulatorRealtimeFrame{Error: firstLine(err.Error())})
		}
	}()
	return frames, stop, nil
}

func streamRealtimeFrames(ctx context.Context, client emugrpcpb.EmulatorControllerClient, format *emugrpcpb.ImageFormat, frames chan GUIEmulatorRealtimeFrame) error {
	stream, err := client.StreamScreenshot(ctx, format)
	if err != nil {
		return pollRealtimeFrames(ctx, client, format, frames)
	}
	received := false
	for {
		image, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !received {
				return pollRealtimeFrames(ctx, client, format, frames)
			}
			return fmt.Errorf("模拟器实时流中断：%w", err)
		}
		if len(image.GetImage()) == 0 {
			continue
		}
		received = true
		sendRealtimeFrame(frames, realtimeFrameFromImage(image))
	}
}

func pollRealtimeFrames(ctx context.Context, client emugrpcpb.EmulatorControllerClient, format *emugrpcpb.ImageFormat, frames chan GUIEmulatorRealtimeFrame) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		image, err := client.GetScreenshot(ctx, format)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("模拟器截图轮询失败：%w", err)
		}
		if len(image.GetImage()) > 0 {
			sendRealtimeFrame(frames, realtimeFrameFromImage(image))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func realtimeFrameFromImage(image *emugrpcpb.Image) GUIEmulatorRealtimeFrame {
	ts := time.Now()
	if image.GetTimestampUs() > 0 {
		ts = time.UnixMicro(int64(image.GetTimestampUs()))
	}
	return GUIEmulatorRealtimeFrame{
		PNG:       image.GetImage(),
		Width:     int(image.GetFormat().GetWidth()),
		Height:    int(image.GetFormat().GetHeight()),
		Seq:       image.GetSeq(),
		Timestamp: ts,
	}
}

func (a *App) GUITapEmulatorRealtime(entryKey string, x, y int) error {
	if x < 0 || y < 0 {
		return fmt.Errorf("点击坐标无效：%d,%d", x, y)
	}
	return a.sendEmulatorRealtimeTouch(entryKey, []*emugrpcpb.Touch{
		{X: int32(x), Y: int32(y), Identifier: 1, Pressure: 1, TouchMajor: 1, TouchMinor: 1},
		{X: int32(x), Y: int32(y), Identifier: 1, Pressure: 0},
	}, 16*time.Millisecond)
}

func (a *App) GUISwipeEmulatorRealtime(entryKey string, x1, y1, x2, y2, durationMs int) error {
	if x1 < 0 || y1 < 0 || x2 < 0 || y2 < 0 {
		return fmt.Errorf("滑动坐标无效：%d,%d -> %d,%d", x1, y1, x2, y2)
	}
	if durationMs < 50 {
		durationMs = 50
	}
	if durationMs > 2000 {
		durationMs = 2000
	}
	const steps = 8
	touches := make([]*emugrpcpb.Touch, 0, steps+2)
	for i := 0; i <= steps; i++ {
		x := x1 + (x2-x1)*i/steps
		y := y1 + (y2-y1)*i/steps
		touches = append(touches, &emugrpcpb.Touch{
			X: int32(x), Y: int32(y), Identifier: 1, Pressure: 1, TouchMajor: 1, TouchMinor: 1,
		})
	}
	touches = append(touches, &emugrpcpb.Touch{X: int32(x2), Y: int32(y2), Identifier: 1, Pressure: 0})
	return a.sendEmulatorRealtimeTouch(entryKey, touches, time.Duration(durationMs/steps)*time.Millisecond)
}

func (a *App) sendEmulatorRealtimeTouch(entryKey string, touches []*emugrpcpb.Touch, delay time.Duration) error {
	target, err := a.GUIEmulatorRealtimeTarget(entryKey)
	if err != nil {
		return err
	}
	if !target.Available {
		return errors.New(target.Reason)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = emulatorRealtimeOutgoingContext(ctx, target.Token)
	conn, client, err := emulatorRealtimeClient(ctx, target.Port)
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, touch := range touches {
		if _, err := client.SendTouch(ctx, &emugrpcpb.TouchEvent{Touches: []*emugrpcpb.Touch{touch}}); err != nil {
			return fmt.Errorf("发送模拟器触控失败：%w", err)
		}
		if touch.Pressure > 0 && delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}

func emulatorRealtimeClient(ctx context.Context, port int) (*grpc.ClientConn, emugrpcpb.EmulatorControllerClient, error) {
	if port <= 0 {
		return nil, nil, fmt.Errorf("模拟器 gRPC 端口无效：%d", port)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		dialCtx,
		net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("连接模拟器 gRPC 失败（port=%d）：%w", port, err)
	}
	return conn, emugrpcpb.NewEmulatorControllerClient(conn), nil
}

func normalizeRealtimeScreenshotSize(width, height int) (int, int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	if width > 0 && width < 160 {
		width = 160
	}
	if height > 0 && height < 160 {
		height = 160
	}
	return width, height
}

func emulatorRealtimeOutgoingContext(ctx context.Context, token string) context.Context {
	token = strings.TrimSpace(token)
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

func emulatorDiscoveryForPID(pid string) map[string]string {
	if strings.TrimSpace(pid) == "" {
		return nil
	}
	for _, path := range emulatorDiscoveryCandidates("pid_" + pid + ".ini") {
		if values := readEmulatorDiscoveryINI(path); len(values) > 0 {
			return values
		}
	}
	return nil
}

func emulatorRealtimeTokenForPort(port int) string {
	if port <= 0 {
		return ""
	}
	for _, dir := range emulatorDiscoveryDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "pid_*.ini"))
		for _, path := range matches {
			values := readEmulatorDiscoveryINI(path)
			if parsePositiveInt(values["grpc.port"]) == port {
				return values["grpc.token"]
			}
		}
	}
	return ""
}

func emulatorDiscoveryCandidates(fileName string) []string {
	var candidates []string
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Library", "Caches", "TemporaryItems", "avd", "running", fileName))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "avd", "running", fileName))
	return candidates
}

func emulatorDiscoveryDirs() []string {
	var dirs []string
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, "Library", "Caches", "TemporaryItems", "avd", "running"))
	}
	dirs = append(dirs, filepath.Join(os.TempDir(), "avd", "running"))
	return dirs
}

func readEmulatorDiscoveryINI(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values
}

func defaultEmulatorGRPCPortForSerial(serial string) int {
	const prefix = "emulator-"
	if !strings.HasPrefix(serial, prefix) {
		return 0
	}
	adbPort, err := strconv.Atoi(strings.TrimPrefix(serial, prefix))
	if err != nil || adbPort <= 0 {
		return 0
	}
	return adbPort + 3000
}

func parsePositiveInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func isTCPPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func sendRealtimeFrame(ch chan GUIEmulatorRealtimeFrame, frame GUIEmulatorRealtimeFrame) {
	select {
	case ch <- frame:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- frame
	}
}
