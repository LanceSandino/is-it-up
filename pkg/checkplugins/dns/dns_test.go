package dns

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/checks"
	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

func stubLookup(t *testing.T, fn func(context.Context, string) ([]string, error)) {
	t.Helper()
	original := lookupHost
	t.Cleanup(func() { lookupHost = original })
	lookupHost = fn
}

func TestPluginRegistersDNSCheckType(t *testing.T) {
	if !slices.Contains(checks.SupportedCheckTypes(), "dns") {
		t.Fatal("dns plugin did not register its check type")
	}
}

func TestRunResolvesHostname(t *testing.T) {
	stubLookup(t, func(_ context.Context, host string) ([]string, error) {
		if host != "example.test" {
			t.Fatalf("unexpected host: %q", host)
		}
		return []string{"192.0.2.10", "2001:db8::10"}, nil
	})
	outcome := run(context.Background(), checks.CheckConfig{Address: secrets.SecretValue{Raw: "example.test"}})
	if !outcome.OK || outcome.Message != "dns lookup ok" || !strings.Contains(outcome.Details, "192.0.2.10") {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunStripsPortFromHostPortAddress(t *testing.T) {
	stubLookup(t, func(_ context.Context, host string) ([]string, error) {
		if host != "db.example.test" {
			t.Fatalf("port should be stripped, got %q", host)
		}
		return []string{"192.0.2.11"}, nil
	})
	if outcome := run(context.Background(), checks.CheckConfig{Address: secrets.SecretValue{Raw: "db.example.test:5432"}}); !outcome.OK {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunReportsLookupFailure(t *testing.T) {
	stubLookup(t, func(context.Context, string) ([]string, error) { return nil, errors.New("host not found") })
	outcome := run(context.Background(), checks.CheckConfig{Host: secrets.SecretValue{Raw: "missing.test"}})
	if outcome.OK || !strings.Contains(outcome.Message, "host not found") {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestRunRejectsURLsAndEmptyHosts(t *testing.T) {
	stubLookup(t, func(context.Context, string) ([]string, error) {
		t.Fatal("lookup must not run for an invalid hostname")
		return nil, nil
	})
	for _, address := range []string{"https://example.test", "", " "} {
		outcome := run(context.Background(), checks.CheckConfig{Address: secrets.SecretValue{Raw: address}})
		if outcome.OK || !strings.Contains(outcome.Message, "hostname") {
			t.Fatalf("address %q: unexpected outcome %+v", address, outcome)
		}
	}
}

func TestRunAllRedactsSecretBackedDNSHost(t *testing.T) {
	const sensitiveHost = "private-dns-name.internal"
	stubLookup(t, func(_ context.Context, host string) ([]string, error) {
		return nil, errors.New("lookup " + host + ": no such host")
	})
	results := checks.RunAll(context.Background(), &checks.Config{Checks: []checks.CheckConfig{{
		Name: "dns-secret", Type: "dns", Host: secrets.SecretValue{Raw: sensitiveHost, Sensitive: true},
	}}})
	if results[0].OK || strings.Contains(results[0].Message+results[0].Details, sensitiveHost) {
		t.Fatalf("secret-backed DNS host leaked: %+v", results[0])
	}
}
