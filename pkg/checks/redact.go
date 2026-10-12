// This file removes credentials and sensitive query values from diagnostics.
// It depends only on standard URL parsing and is shared by outbound check modules.
package checks

import (
	"net/url"
	"regexp"
	"strings"
)

var credentialAssignmentPattern = regexp.MustCompile(`(?i)(^|\s)(?:user|username|password|pass|pwd)=[^\s]+`)
var urlUserinfoPattern = regexp.MustCompile(`://[^/@\s:]+:[^@/\s]+@`)

// redactAddress removes credentials from endpoint strings before they reach logs.
func redactAddress(value string) string {
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.User != nil {
		parsed.User = url.User("REDACTED")
		return parsed.String()
	}
	if marker := strings.Index(value, "@tcp("); marker >= 0 {
		return "REDACTED" + value[marker:]
	}
	value = urlUserinfoPattern.ReplaceAllString(value, "://REDACTED@")
	return credentialAssignmentPattern.ReplaceAllString(value, "${1}[redacted]")
}

func redactText(value string, secrets ...string) string {
	redacted := value
	for _, secret := range secrets {
		if secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "REDACTED")
			redacted = strings.ReplaceAll(redacted, url.QueryEscape(secret), "REDACTED")
		}
	}
	return redactAddress(redacted)
}
