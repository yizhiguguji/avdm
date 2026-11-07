package app

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestUnlockKeychainRoundTrip(t *testing.T) {
	if os.Getenv("ADM_TEST_KEYCHAIN") != "1" {
		t.Skip("requires an unlocked local macOS keychain")
	}
	account := "uuid-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = unlockSecretDelete(account) })
	if err := unlockSecretSave(account, `{"Kind":"PIN","Secret":"123456"}`); err != nil {
		t.Fatal(err)
	}
	if value, err := unlockSecretRead(account); err != nil || value != `{"Kind":"PIN","Secret":"123456"}` {
		t.Fatal("keychain round trip failed", err)
	}
	if err := unlockSecretSave(account, `{"Kind":"PIN","Secret":"654321"}`); err != nil {
		t.Fatal(err)
	}
	if value, err := unlockSecretRead(account); err != nil || value != `{"Kind":"PIN","Secret":"654321"}` {
		t.Fatal("keychain update failed", err)
	}
	if err := unlockSecretDelete(account); err != nil {
		t.Fatal(err)
	}
	if value, err := unlockSecretRead(account); err != nil || value != "" {
		t.Fatal("keychain deletion failed", err)
	}
}
