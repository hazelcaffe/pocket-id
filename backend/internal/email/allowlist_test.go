package email

import "testing"

func TestRecipientAllowlist(t *testing.T) {
	t.Setenv(recipientAllowlistEnv, "")
	if !recipientAllowed("anyone@example.com") {
		t.Fatal("unset allowlist must allow everything")
	}
	t.Setenv(recipientAllowlistEnv, "hazel@yuri.llc, noc@yuri.llc")
	for addr, want := range map[string]bool{"hazel@yuri.llc": true, "NOC@yuri.llc": true, "someone@yuri.llc": false, "hazel@yuri.llc.evil.com": false, "x@example.com": false} {
		if got := recipientAllowed(addr); got != want {
			t.Fatalf("%s: got %v want %v", addr, got, want)
		}
	}
}
