package app

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// UiController's settings pane is part of the emulator's installed
// ui_controller_service.proto. Keep this small request descriptor local instead
// of generating the unrelated controller APIs. PaneIndex.SETTINGS is 17.
var emulatorSettingsPaneDescriptor = func() protoreflect.MessageDescriptor {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("adm_ui_settings.proto"), Package: proto.String("android.emulation.control"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("PaneEntry"), Field: []*descriptorpb.FieldDescriptorProto{{Name: proto.String("index"), Number: proto.Int32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(), TypeName: proto.String(".android.emulation.control.PaneIndex")}}}},
		EnumType:    []*descriptorpb.EnumDescriptorProto{{Name: proto.String("PaneIndex"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("KEEP_CURRENT"), Number: proto.Int32(0)}, {Name: proto.String("SETTINGS"), Number: proto.Int32(17)}}}},
	}, nil)
	if err != nil {
		panic(err)
	}
	return file.Messages().Get(0)
}()

func showEmulatorSettings(ctx context.Context, conn grpc.ClientConnInterface) (bool, error) {
	request := dynamicpb.NewMessage(emulatorSettingsPaneDescriptor)
	request.Set(emulatorSettingsPaneDescriptor.Fields().ByName("index"), protoreflect.ValueOfEnum(17))
	// ExtendedControlsStatus.visibilityChanged is field 1, a boolean. BoolValue
	// has the identical wire shape and lets us preserve an already-open pane.
	response := new(wrapperspb.BoolValue)
	err := conn.Invoke(ctx, "/android.emulation.control.UiController/showExtendedControls", request, response)
	return response.Value, err
}

func (a *App) setNativeEmulatorWindowTop(entry DeviceEntry, enabled bool) error {
	name, _ := emulatorEntryIdentity(entry)
	pid, ok := qemuPIDForAVD(name)
	if !ok {
		return fmt.Errorf("未找到模拟器原生窗口进程：%s", entry.Label)
	}
	if nativeWindowAccessUnavailable() {
		return &AccessibilityPermissionRequiredError{Operation: "设置模拟器窗口置顶"}
	}
	if top, err := emulatorWindowTopNative(pid, name); err == nil && top == enabled {
		return nil
	}
	target := a.GUIEmulatorRealtimeTargetForEntry(entry)
	if !target.Available {
		// Window controls exist before Android/ADB finishes booting.
		for _, process := range runningEmulatorProcessesForAVD(name) {
			discovery := emulatorDiscoveryForPID(process.PID)
			port := parsePositiveInt(discovery["grpc.port"])
			if port == 0 {
				port = process.GRPCPort
			}
			if port > 0 {
				target.Port, target.Token, target.Available = port, discovery["grpc.token"], true
				break
			}
		}
		if !target.Available {
			return fmt.Errorf("无法应用模拟器原生窗口置顶：%s", target.Reason)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ctx = emulatorRealtimeOutgoingContext(ctx, target.Token)
	conn, _, err := emulatorRealtimeClient(ctx, target.Port)
	if err != nil {
		return err
	}
	defer conn.Close()
	opened, err := showEmulatorSettings(ctx, conn)
	if err != nil {
		return fmt.Errorf("打开模拟器窗口设置失败：%w", err)
	}
	if opened {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			closeCtx = emulatorRealtimeOutgoingContext(closeCtx, target.Token)
			_ = conn.Invoke(closeCtx, "/android.emulation.control.UiController/closeExtendedControls", &emptypb.Empty{}, &wrapperspb.BoolValue{})
		}()
	}
	return applyEmulatorWindowTopNative(pid, name, enabled)
}

func openNativeEmulatorWindowTop(name, serial string, enabled bool, setTop func(bool) error, focus func(string, string) error) error {
	if err := setTop(enabled); err != nil {
		return err
	}
	return focus(name, serial)
}
