package app

import "testing"

func TestWirelessBonjourUsesConnectionSRVOnly(t *testing.T) {
	output := `; comment
_adb-tls-connect._tcp PTR adb-DEMO001-guid._adb-tls-connect._tcp
adb-DEMO001-guid._adb-tls-connect._tcp SRV 0 0 41000 phone.local. ; comment
adb-DEMO001-guid._adb-tls-connect._tcp TXT "name=Demo" "given_name=测试手机"
adb-DEMO001-guid._adb-tls-pairing._tcp SRV 0 0 43000 phone.local.
malformed SRV
`
	got := parseWirelessBonjourZone(output, func(host, port string) string {
		if host != "phone.local." || port != "41000" {
			t.Fatalf("unexpected lookup: %s %s", host, port)
		}
		return "192.168.1.20:41000"
	})
	if len(got) != 1 || got[0].Name != "adb-DEMO001-guid" || got[0].Address != "192.168.1.20:41000" || got[0].DisplayName != "测试手机" {
		t.Fatalf("bad Bonjour result: %+v", got)
	}
}
