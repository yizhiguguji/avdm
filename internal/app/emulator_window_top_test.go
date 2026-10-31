package app

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type settingsPaneConnection struct{ t *testing.T }

func (c settingsPaneConnection) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	if method != "/android.emulation.control.UiController/showExtendedControls" {
		c.t.Fatalf("wrong UI method: %s", method)
	}
	request := args.(*dynamicpb.Message)
	if request.Get(request.Descriptor().Fields().ByName("index")).Enum() != protoreflect.EnumNumber(17) {
		c.t.Fatal("did not select vendor settings pane")
	}
	reply.(*wrapperspb.BoolValue).Value = true
	return nil
}
func (c settingsPaneConnection) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unexpected stream")
}
func TestNativeTopUsesVendorSettingsPane(t *testing.T) {
	opened, err := showEmulatorSettings(context.Background(), settingsPaneConnection{t})
	if err != nil || !opened {
		t.Fatalf("settings status lost: %v %v", opened, err)
	}
}
func TestNativeTopAppliesBeforeFocusAndPreservesFailure(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		applied := false
		err := openNativeEmulatorWindowTop("Demo_AVD", "emulator-demo", enabled, func(value bool) error {
			if value != enabled {
				t.Fatal("wrong top state")
			}
			applied = true
			return nil
		}, func(name, serial string) error {
			if !applied || name != "Demo_AVD" || serial != "emulator-demo" {
				t.Fatal("focus preceded top or lost target")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("native switch unavailable")
	err := openNativeEmulatorWindowTop("Demo_AVD", "emulator-demo", true, func(bool) error { return failure }, func(string, string) error { t.Fatal("reported success without top"); return nil })
	if !errors.Is(err, failure) {
		t.Fatalf("lost top failure: %v", err)
	}
}
