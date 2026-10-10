package email

import (
	"log/slog"
	"os"
	"strings"
)

// YuriID safety net for test/staging instances that use a real SMTP server.
// When EMAIL_RECIPIENT_ALLOWLIST is set (comma-separated addresses, or "@domain" entries), every email
// to a recipient that is not on the list is dropped before any SMTP connection is opened.
// Unset = no restriction (normal production behaviour).
const recipientAllowlistEnv = "EMAIL_RECIPIENT_ALLOWLIST"

func recipientAllowed(addr string) bool {
	raw := strings.TrimSpace(os.Getenv(recipientAllowlistEnv))
	if raw == "" {
		return true
	}
	addr = strings.ToLower(strings.TrimSpace(addr))
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "@") && strings.HasSuffix(addr, entry) {
			return true
		}
		if addr == entry {
			return true
		}
	}
	return false
}

func logBlockedRecipient(template string) {
	// The address itself is not logged
	slog.Warn("YuriID: email suppressed by EMAIL_RECIPIENT_ALLOWLIST", slog.String("template", template))
}
