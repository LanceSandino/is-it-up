// Package dns registers a DNS resolution health check with pkg/checks.
// Importing this package activates the "dns" check type in the importing binary.
package dns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/LanceSandino/is-it-up/v2/pkg/checks"
)

var lookupHost = net.DefaultResolver.LookupHost

func init() {
	checks.RegisterCheckType("dns", run)
}

// hostname returns the name to resolve from host, or from address with any
// port removed. URLs are rejected because this check resolves names only.
func hostname(cfg checks.CheckConfig) (string, bool) {
	name := strings.TrimSpace(cfg.Host.String())
	if name == "" {
		name = strings.TrimSpace(cfg.Address.String())
	}
	if name == "" || strings.Contains(name, "://") || strings.Contains(name, "/") {
		return "", false
	}
	if host, _, err := net.SplitHostPort(name); err == nil {
		name = host
	}
	return name, name != ""
}

// run relies on RunAll for the check deadline, panic containment, and redaction.
func run(ctx context.Context, cfg checks.CheckConfig) checks.CheckOutcome {
	host, ok := hostname(cfg)
	if !ok {
		return checks.CheckOutcome{OK: false, Message: "dns check requires a hostname in host or address", Details: "dns lookup"}
	}
	addresses, err := lookupHost(ctx, host)
	if err != nil {
		return checks.CheckOutcome{OK: false, Message: "dns lookup failed: " + err.Error(), Details: fmt.Sprintf("dns lookup host=%s", host)}
	}
	return checks.CheckOutcome{OK: true, Message: "dns lookup ok", Details: fmt.Sprintf("dns lookup host=%s addresses=%s", host, strings.Join(addresses, ","))}
}
