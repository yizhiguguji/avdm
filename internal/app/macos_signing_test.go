package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacOSPackagingRequiresExplicitAdhocOptIn(t *testing.T) {
	for _, allowed := range []string{"0", "1"} {
		cmd := exec.Command("make", "--no-print-directory", "-f", "../../Makefile", "check-macos-signing", "CODESIGN_IDENTITY=-", "ALLOW_ADHOC_SIGNING="+allowed)
		output, err := cmd.CombinedOutput()
		if allowed == "0" {
			if err == nil || !strings.Contains(string(output), "已停止打包") {
				t.Fatalf("implicit temporary signing was accepted: %v\n%s", err, output)
			}
		} else if err != nil || !strings.Contains(string(output), "显式使用临时签名") {
			t.Fatalf("explicit debug signing was rejected: %v\n%s", err, output)
		}
	}
}

func TestMacOSSigningSignsNestedCodeThenVerifiesBundle(t *testing.T) {
	root := t.TempDir()
	resources := filepath.Join(root, "sample.app", "Contents", "Resources")
	if err := os.MkdirAll(resources, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "helper.dylib"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	// Run the real script with a fake signer and an empty signing directory.
	signer := filepath.Join(root, "codesign")
	if err := os.WriteFile(signer, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ADM_TEST_SIGNING_CALLS\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(root, "calls")
	bundle := filepath.Dir(filepath.Dir(resources))
	cmd := exec.Command("/bin/bash", "../../scripts/sign-macos-app.sh", "-", "com.example.app", bundle)
	cmd.Env = append(os.Environ(), "PATH="+root+":/usr/bin:/bin", "ADM_SIGNING_DIR="+root, "ADM_TEST_SIGNING_CALLS="+trace)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("signing failed: %v\n%s", err, output)
	}
	output, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], "com.example.app.helper") || !strings.HasSuffix(lines[0], "helper.dylib") || !strings.HasSuffix(lines[1], bundle) || lines[2] != "--verify --deep --strict "+bundle {
		t.Fatalf("incorrect signing order: %s", output)
	}
	if strings.Contains(lines[0], "--deep") || strings.Contains(lines[1], "--deep") {
		t.Fatalf("nested signing relied on implicit identity resolution: %s", output)
	}
}
