package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedMirrorIgnoringTerminateIsReaped(t *testing.T) {
	command := exec.Command("python3", "-u", "-c", "import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); print('ready'); time.sleep(30)")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	if !bufio.NewScanner(stdout).Scan() {
		t.Fatal("child not ready")
	}
	done := make(chan struct{})
	go func() { _ = command.Wait(); close(done) }()
	if err := stopManagedScrcpyProcess(command.Process, done, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	default:
		t.Fatal("old mirror not reaped before replacement")
	}
}
func TestOnlyTransientStartupExitsAreRetried(t *testing.T) {
	for _, message := range []string{"Server connection failed", "device offline", "protocol fault", "Could not retrieve device information"} {
		err := fmt.Errorf("%w: %s", &scrcpyStartupExitError{cause: errors.New("exit status 1")}, message)
		if !transientScrcpyStartupFailure(err) {
			t.Fatalf("transient failure not retried: %v", err)
		}
	}
	for _, err := range []error{errors.New("Server connection failed"), fmt.Errorf("%w: unknown option", &scrcpyStartupExitError{}), errors.New("window mismatch")} {
		if transientScrcpyStartupFailure(err) {
			t.Fatalf("persistent failure retried: %v", err)
		}
	}
}

func TestMirrorRetryIsBoundedAndWaitsForADB(t *testing.T) {
	failure := fmt.Errorf("%w: Server connection failed", &scrcpyStartupExitError{})
	for _, recovered := range []bool{true, false} {
		calls, checks := 0, 0
		err := retryScrcpyStartup(func() error {
			calls++
			if calls == 2 && recovered {
				return nil
			}
			return failure
		}, func() error { checks++; return nil })
		if calls != 2 || checks != 1 || (err == nil) != recovered {
			t.Fatalf("retry calls=%d checks=%d err=%v", calls, checks, err)
		}
	}
	calls := 0
	err := retryScrcpyStartup(func() error { calls++; return failure }, func() error { return errors.New("offline") })
	if calls != 1 || err == nil {
		t.Fatal("retried before device connection recovered")
	}
}
func TestMirrorLogExcerptExcludesPreviousAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mirror.log")
	content := "ERROR: device offline\n--- mirror session ---\nERROR: Server connection failed\n" + strings.Repeat("INFO: starting\n", 12)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	excerpt := scrcpyLogExcerpt(path)
	if strings.Contains(excerpt, "device offline") || !strings.Contains(excerpt, "Server connection failed") {
		t.Fatalf("wrong attempt diagnostics: %s", excerpt)
	}
}
