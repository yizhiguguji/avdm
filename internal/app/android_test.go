package app

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestParseADBDevices(t *testing.T) {
	input := `List of devices attached
emulator-5554 device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1
PHONE-TEST-002 unauthorized usb:1-2 product:o1q model:SM_G9910 device:o1q
`
	got := parseADBDevices(input)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if !got[0].IsEmulator || got[0].State != "device" || got[0].Details["model"] != "sdk_gphone64_arm64" {
		t.Fatalf("unexpected first device: %+v", got[0])
	}
	if got[1].State != "unauthorized" || got[1].Serial != "PHONE-TEST-002" {
		t.Fatalf("unexpected second device: %+v", got[1])
	}
}

func TestParseEmulatorProcessArgs(t *testing.T) {
	input := "/opt/android/emulator/qemu/darwin-aarch64/qemu-system-aarch64 -avd demo_pixel9pro_2 -port 5556 -grpc 34567 -read-only"
	avdName, ports, grpcPort := parseEmulatorProcessArgs(input)
	if avdName != "demo_pixel9pro_2" {
		t.Fatalf("avdName=%q", avdName)
	}
	if !reflect.DeepEqual(ports, []string{"5556"}) {
		t.Fatalf("ports=%v", ports)
	}
	if grpcPort != 34567 {
		t.Fatalf("grpcPort=%d", grpcPort)
	}
}

func TestParseEmulatorProcessArgsEqualsForm(t *testing.T) {
	input := "/opt/android/emulator/emulator @demo_5053 -ports=5558,5559 -grpc=34568"
	avdName, ports, grpcPort := parseEmulatorProcessArgs(input)
	if avdName != "demo_5053" {
		t.Fatalf("avdName=%q", avdName)
	}
	if !reflect.DeepEqual(ports, []string{"5558"}) {
		t.Fatalf("ports=%v", ports)
	}
	if grpcPort != 34568 {
		t.Fatalf("grpcPort=%d", grpcPort)
	}
}

func TestEmulatorGRPCPortForAVDNameIsStable(t *testing.T) {
	got := emulatorGRPCPortForAVDName("demo_5053")
	if got < 30000 || got > 39999 {
		t.Fatalf("port out of range: %d", got)
	}
	if again := emulatorGRPCPortForAVDName("demo_5053"); again != got {
		t.Fatalf("port not stable: %d != %d", again, got)
	}
}

func TestDefaultEmulatorGRPCPortForSerial(t *testing.T) {
	tests := []struct {
		serial string
		want   int
	}{
		{serial: "emulator-5554", want: 8554},
		{serial: "emulator-5556", want: 8556},
		{serial: "device-1234", want: 0},
		{serial: "emulator-bad", want: 0},
	}
	for _, tt := range tests {
		if got := defaultEmulatorGRPCPortForSerial(tt.serial); got != tt.want {
			t.Fatalf("defaultEmulatorGRPCPortForSerial(%q) = %d, want %d", tt.serial, got, tt.want)
		}
	}
}

func TestParseAVDList(t *testing.T) {
	input := `Available Android Virtual Devices:
    Name: demo_pixel9pro
  Device: pixel_9_pro (Google)
    Path: /Users/me/.android/avd/demo_pixel9pro.avd
  Target: Google Play (Google Inc.)
          Based on: Android 15.0 ("VanillaIceCream") Tag/ABI: google_apis_playstore/arm64-v8a
---------
    Name: sample_zh
  Device: pixel_9 (Google)
    Path: /Users/me/.android/avd/sample_zh.avd
  Target: Google APIs
`
	got := parseAVDList(input)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Name != "demo_pixel9pro" || got[0].Device != "pixel_9_pro (Google)" {
		t.Fatalf("unexpected first avd: %+v", got[0])
	}
	if got[1].Name != "sample_zh" || got[1].Target != "Google APIs" {
		t.Fatalf("unexpected second avd: %+v", got[1])
	}
}

func TestParseSystemImages(t *testing.T) {
	input := `Installed packages:
  Path                                                    | Version | Description
  system-images;android-35;google_apis_playstore;arm64-v8a | 9       | Google Play ARM 64 v8a System Image
  platforms;android-35                                     | 2       | Android SDK Platform 35
`
	got := parseSystemImages(input)
	want := []string{"system-images;android-35;google_apis_playstore;arm64-v8a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseDeviceTemplates(t *testing.T) {
	input := `Available devices definitions:
id: 0 or "tv_1080p"
    Name: Television (1080p)
id: 1 or "wearos_large_round"
    Name: Wear OS Large Round
id: 2 or "pixel_7"
    Name: Pixel 7
id: 3 or "pixel_8"
    Name: Pixel 8
id: 4 or "pixel_9"
    Name: Pixel 9
id: 5 or "pixel_fold"
    Name: Pixel Fold
id: 6 or "medium_phone"
    Name: Medium Phone
id: 7 or "medium_tablet"
    Name: Medium Tablet
id: 8 or "Nexus 6P"
    Name: Nexus 6P
id: 9 or "desktop_large"
    Name: Large Desktop
`
	got := parseDeviceTemplates(input)
	if len(got) != 5 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID != "pixel_9" || got[1].ID != "pixel_8" || got[2].ID != "pixel_fold" || got[3].ID != "medium_phone" || got[4].ID != "medium_tablet" {
		t.Fatalf("unexpected templates: %+v", got)
	}
}

func TestEncodeInputText(t *testing.T) {
	got := encodeInputText("hello world")
	if got != "hello%sworld" {
		t.Fatalf("got %q", got)
	}
}

func TestIsHTTPURL(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/app.apk": true,
		"http://example.com/app.apk":  true,
		"ftp://example.com/app.apk":   false,
		"/tmp/app.apk":                false,
	}
	for input, want := range cases {
		if got := isHTTPURL(input); got != want {
			t.Fatalf("isHTTPURL(%q)=%v want %v", input, got, want)
		}
	}
}

func TestAPKNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/app.apk?token=abc": "app.apk",
		"https://example.com/download":          "download",
		"https://example.com/":                  "download.apk",
	}
	for input, want := range cases {
		if got := apkNameFromURL(input); got != want {
			t.Fatalf("apkNameFromURL(%q)=%q want %q", input, got, want)
		}
	}
}

func TestAPKNameFromDownloadUsesContentDisposition(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Disposition", `attachment; filename="app-release.apk"`)
	got := apkNameFromDownload("https://example.com/download?token=abc", header)
	if got != "app-release.apk" {
		t.Fatalf("got %q", got)
	}
}

func TestAnalyzeInstallFailure(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want installFailureHint
	}{
		{
			name: "downgrade",
			out:  "Failure [INSTALL_FAILED_VERSION_DOWNGRADE]",
			want: installFailureHint{NeedsDowngrade: true},
		},
		{
			name: "test only",
			out:  "Failure [INSTALL_FAILED_TEST_ONLY: installPackageLI]",
			want: installFailureHint{NeedsTestOnly: true},
		},
		{
			name: "both",
			out:  "Failure [INSTALL_FAILED_VERSION_DOWNGRADE] testOnly=true",
			want: installFailureHint{NeedsDowngrade: true, NeedsTestOnly: true},
		},
		{
			name: "signature",
			out:  "Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE]",
			want: installFailureHint{NeedsSignatureReinstall: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := analyzeInstallFailure(tc.out); got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestExplainInstallFailure(t *testing.T) {
	got := explainInstallFailure("Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE]")
	if !strings.Contains(got, "签名不一致") {
		t.Fatalf("unexpected explanation: %q", got)
	}
	got = explainInstallFailure("error: device offline")
	if !strings.Contains(got, "offline") {
		t.Fatalf("unexpected explanation: %q", got)
	}
}

func TestInstallArgs(t *testing.T) {
	got := installArgs("emulator-5554", "/tmp/app.apk", installMode{AllowDowngrade: true, AllowTestOnly: true})
	want := []string{"-s", "emulator-5554", "install", "-r", "-d", "-t", "/tmp/app.apk"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestInstallTargetsFromActiveDevicesUsesPhysicalSerials(t *testing.T) {
	selected := map[string]bool{
		"PHONE-TEST-001": true,
		"emulator-5554":  true,
		"missing":        true,
	}
	devices := []ActiveDevice{
		{Serial: "PHONE-TEST-001", State: "device", Details: map[string]string{"model": "SM_S931Q"}},
		{Serial: "emulator-5554", State: "offline", IsEmulator: true, AVDName: "demo_5050"},
	}

	targets, unavailable := installTargetsFromActiveDevices(selected, devices)
	if len(targets) != 1 || targets[0].device.Serial != "PHONE-TEST-001" || targets[0].label != "SM_S931Q | PHONE-TEST-001" {
		t.Fatalf("unexpected targets: %+v", targets)
	}
	wantUnavailable := []string{"demo_5050 | emulator-5554", "missing"}
	if !reflect.DeepEqual(unavailable, wantUnavailable) {
		t.Fatalf("unavailable=%#v want %#v", unavailable, wantUnavailable)
	}
}

func TestUpdateAVDConfigValueReplacesExisting(t *testing.T) {
	input := "avd.ini.displayname=Pixel\nhw.keyboard=no\nskin.name=pixel\n"
	got := updateAVDConfigValue(input, "hw.keyboard", "yes")
	want := "avd.ini.displayname=Pixel\nhw.keyboard=yes\nskin.name=pixel\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUpdateAVDConfigValueReplacesSpacedExisting(t *testing.T) {
	input := "hw.keyboard = false\nhw.keyboard.lid = true\n"
	got := updateAVDConfigValue(input, "hw.keyboard", "yes")
	want := "hw.keyboard=yes\nhw.keyboard.lid = true\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUpdateAVDConfigValueAppendsMissing(t *testing.T) {
	input := "avd.ini.displayname=Pixel\nskin.name=pixel\n"
	got := updateAVDConfigValue(input, "hw.keyboard", "yes")
	want := "avd.ini.displayname=Pixel\nskin.name=pixel\nhw.keyboard=yes\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUpdateAVDHardwareKeyboardConfig(t *testing.T) {
	input := strings.Join([]string{
		"avd.ini.displayname=Pixel",
		"hw.keyboard=no",
		"hw.keyboard.charmap=old",
		"skin.name=pixel",
		"",
	}, "\n")
	got := updateAVDHardwareKeyboardConfig(input)
	want := strings.Join([]string{
		"avd.ini.displayname=Pixel",
		"hw.keyboard=yes",
		"hw.keyboard.charmap=qwerty2",
		"skin.name=pixel",
		"hw.keyboard.lid=yes",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUpdateAVDConfigValueSetsDataPartition(t *testing.T) {
	input := "avd.ini.displayname=Pixel\nskin.name=pixel_9_pro\n"
	got := updateAVDConfigValue(input, "disk.dataPartition.size", defaultAVDDataPartitionSize)
	want := "avd.ini.displayname=Pixel\nskin.name=pixel_9_pro\ndisk.dataPartition.size=32G\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseDataFreeBytesFromDF(t *testing.T) {
	output := "Filesystem     1K-blocks    Used Available Use% Mounted on\n/dev/block/dm-44 6082144 5372468 709676 89% /data/user/0"
	got, err := parseDataFreeBytesFromDF(output)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	want := int64(709676 * 1024)
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestRequiredInstallFreeBytesUsesMinimumAndMultiplier(t *testing.T) {
	if got := requiredInstallFreeBytes(200 * 1024 * 1024); got != minInstallFreeSpaceBytes {
		t.Fatalf("small apk required=%d want %d", got, minInstallFreeSpaceBytes)
	}
	want := int64(700 * 1024 * 1024 * installFreeSpaceMultiplier)
	if got := requiredInstallFreeBytes(700 * 1024 * 1024); got != want {
		t.Fatalf("large apk required=%d want %d", got, want)
	}
}

func TestLocaleMatchesConfig(t *testing.T) {
	config := "config: mcc310-mnc260-zh-rCN-ldltr-sw427dp"
	if !localeMatchesConfig(config, "zh-CN") {
		t.Fatal("expected zh-CN to match")
	}
	if !localeMatchesConfig(config, "zh_CN") {
		t.Fatal("expected zh_CN to match")
	}
	if localeMatchesConfig(config, "en-US") {
		t.Fatal("expected en-US not to match")
	}

	android15Config := "config: mcc310-mnc260-b+zh+Hans+CN,en-rUS-ldltr-sw427dp"
	if !localeMatchesConfig(android15Config, "zh-CN") {
		t.Fatal("expected Android 15 zh-Hans-CN config to match zh-CN")
	}
	secondaryConfig := "config: mcc310-mnc260-en-rUS,b+zh+Hans+CN-ldltr-sw427dp"
	if localeMatchesConfig(secondaryConfig, "zh-CN") {
		t.Fatal("expected secondary zh-Hans-CN not to count as active system language")
	}
}

func TestLocaleSettingContains(t *testing.T) {
	if !localeSettingContains("zh-CN", "zh-CN") {
		t.Fatal("expected exact locale to match")
	}
	if !localeSettingContains("en-US, zh_CN", "zh-CN") {
		t.Fatal("expected normalized locale list to match")
	}
	if !localeSettingContains("zh-Hans-CN,en-US", "zh-CN") {
		t.Fatal("expected zh-Hans-CN to match zh-CN")
	}
	if localeSettingContains("en-US", "zh-CN") {
		t.Fatal("expected different locale not to match")
	}
}

func TestIsSimpleADBInputText(t *testing.T) {
	if !isSimpleADBInputText("hello 123") {
		t.Fatal("expected simple ASCII to pass")
	}
	if isSimpleADBInputText("你好") {
		t.Fatal("expected Chinese text to fail")
	}
	if isSimpleADBInputText("") {
		t.Fatal("expected empty text to fail")
	}
}

func TestSplitLocale(t *testing.T) {
	lang, region := splitLocale("zh-CN")
	if lang != "zh" || region != "CN" {
		t.Fatalf("got %q %q", lang, region)
	}
	lang, region = splitLocale("ja")
	if lang != "ja" || region != "" {
		t.Fatalf("got %q %q", lang, region)
	}
}

func TestSanitizeLaunchLabel(t *testing.T) {
	got := sanitizeLaunchLabel("demo.pixel-9 pro")
	want := "demo_pixel_9_pro"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRemoveString(t *testing.T) {
	got := removeString([]string{"a", "b", "a"}, "a")
	want := []string{"b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
