package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Execute the real installer against fake tools; never install host packages.
func TestDependencyInstallerRetriesAndReportsFailures(t *testing.T) {
	for _, scenario := range []struct {
		name, mode, license string
		wantAttempts        int
		wantError           string
	}{
		{"retry succeeds", "retry", "0", 3, ""},
		{"retry exhausted", "fail", "0", 3, "Android SDK component failed: platform-tools (exit 23)"},
		{"license rejected", "retry", "7", 0, "Android SDK license check failed (exit 7)"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			tools := map[string]string{
				"uname": "#!/bin/sh\necho Darwin\n",
				"brew": `#!/bin/sh
printf 'brew %s\n' "$*" >> "$ADM_TEST_ROOT/calls"
case "$1" in
  --prefix) printf '%s\n' "$ADM_TEST_ROOT";;
  list) case "$*" in *--formula*android-commandlinetools*) exit 1;; esac;;
  *) echo 'unexpected package installation' >&2; exit 99;;
esac
`,
				"sdkmanager": `#!/bin/sh
printf 'sdkmanager %s\n' "$*" >> "$ADM_TEST_ROOT/calls"
if [ "$1" = --licenses ]; then
  IFS= read -r answer
  exit "$ADM_TEST_LICENSE"
fi
if [ "$2" = platform-tools ]; then
  attempt=0
  if [ -f "$ADM_TEST_ROOT/attempts" ]; then
    IFS= read -r attempt < "$ADM_TEST_ROOT/attempts"
  fi
  attempt=$((attempt + 1))
  printf '%s\n' "$attempt" > "$ADM_TEST_ROOT/attempts"
  if [ "$ADM_TEST_MODE" = fail ] || [ "$attempt" -lt 3 ]; then exit 23; fi
fi
`,
			}
			for _, name := range []string{"go", "adb", "emulator", "avdmanager", "scrcpy"} {
				tools[name] = "#!/bin/sh\nexit 0\n"
			}
			for name, script := range tools {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			javaBin := filepath.Join(root, "libexec", "openjdk.jdk", "Contents", "Home", "bin")
			if err := os.MkdirAll(javaBin, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(javaBin, "java"), []byte("#!/bin/sh\nprintf 'java %s\\n' \"$*\" >> \"$ADM_TEST_ROOT/calls\"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/bin/bash", "../../scripts/install-macos-deps.sh")
			cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "ADM_TEST_ROOT="+root, "ADM_TEST_MODE="+scenario.mode, "ADM_TEST_LICENSE="+scenario.license)
			output, err := cmd.CombinedOutput()
			if scenario.wantError == "" {
				if err != nil || !strings.Contains(string(output), "Done.") {
					t.Fatalf("installer failed: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), scenario.wantError) {
				t.Fatalf("missing expected failure %q: %v\n%s", scenario.wantError, err, output)
			}
			calls, err := os.ReadFile(filepath.Join(root, "calls"))
			if err != nil {
				t.Fatal(err)
			}
			trace := string(calls)
			if got := strings.Count(trace, "sdkmanager --verbose platform-tools\n"); got != scenario.wantAttempts {
				t.Fatalf("got %d attempts, want %d: %s", got, scenario.wantAttempts, trace)
			}
			if !strings.Contains(trace, "java -version") || !strings.Contains(trace, "brew list --cask --versions android-commandlinetools") || strings.Contains(trace, "brew install ") {
				t.Fatalf("Java or existing cask detection failed: %s", trace)
			}
			if got := strings.Contains(trace, "sdkmanager --verbose emulator\n"); got != (scenario.wantError == "") {
				t.Fatalf("installer continued after failure or stopped after recovery: %s", trace)
			}
		})
	}
}
