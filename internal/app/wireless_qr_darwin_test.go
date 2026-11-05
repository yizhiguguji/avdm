package app

import "testing"

func TestQRBonjourMatchesOnlyPairingSession(t *testing.T) {
	output := "other._adb-tls-pairing._tcp SRV 0 0 40000 phone.local.\nadm-test._adb-tls-pairing._tcp SRV 0 0 41000 phone.local.\n"
	calls := 0
	address := qrPairingBonjourAddress(output, "adm-test", func(host, port string) string {
		calls++
		if host != "phone.local." || port != "41000" {
			t.Fatal(host, port)
		}
		return "192.168.1.20:41000"
	})
	if address != "192.168.1.20:41000" || calls != 1 {
		t.Fatal(address, calls)
	}
}
