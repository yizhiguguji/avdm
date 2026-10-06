package app

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	emugrpcpb "adm/internal/emugrpcpb"
)

func TestEmulatorRealtimeStreamIntegration(t *testing.T) {
	portText := os.Getenv("ADM_TEST_EMULATOR_GRPC_PORT")
	if portText == "" {
		t.Skip("set ADM_TEST_EMULATOR_GRPC_PORT to test against a running emulator")
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("invalid ADM_TEST_EMULATOR_GRPC_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = emulatorRealtimeOutgoingContext(ctx, emulatorRealtimeTokenForPort(port))
	conn, client, err := emulatorRealtimeClient(ctx, port)
	if err != nil {
		t.Fatalf("connect emulator gRPC: %v", err)
	}
	defer conn.Close()
	frame, err := client.GetScreenshot(ctx, &emugrpcpb.ImageFormat{
		Format: emugrpcpb.ImageFormat_PNG,
		Width:  0,
		Height: 0,
	})
	if err != nil {
		t.Fatalf("get screenshot: %v", err)
	}
	if len(frame.GetImage()) == 0 {
		t.Fatalf("empty screenshot frame")
	}
	if width := frame.GetFormat().GetWidth(); width > 0 && width < 700 {
		t.Fatalf("screenshot width too small for high-quality stream: %d", width)
	}
}

func TestNormalizeRealtimeScreenshotSizeKeepsNativeZero(t *testing.T) {
	width, height := normalizeRealtimeScreenshotSize(0, 0)
	if width != 0 || height != 0 {
		t.Fatalf("zero size must request native resolution, got %dx%d", width, height)
	}
	width, height = normalizeRealtimeScreenshotSize(1, -1)
	if width != 160 || height != 0 {
		t.Fatalf("unexpected normalized size: %dx%d", width, height)
	}
}

func TestRealtimeFrameFromImageUsesNativeMetadata(t *testing.T) {
	frame := realtimeFrameFromImage(&emugrpcpb.Image{
		Image: []byte{1, 2, 3},
		Seq:   42,
		Format: &emugrpcpb.ImageFormat{
			Width:  1080,
			Height: 2400,
		},
		TimestampUs: 1_700_000_000_000_000,
	})
	if frame.Width != 1080 || frame.Height != 2400 {
		t.Fatalf("unexpected frame size: %dx%d", frame.Width, frame.Height)
	}
	if frame.Seq != 42 {
		t.Fatalf("unexpected seq: %d", frame.Seq)
	}
	if len(frame.PNG) != 3 {
		t.Fatalf("unexpected image bytes: %d", len(frame.PNG))
	}
	if frame.Timestamp.IsZero() {
		t.Fatalf("timestamp should be populated")
	}
}
