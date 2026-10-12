// This file defines the secret-fetching interface and provider selection boundary.
// It depends on context and returns opaque values to configuration resolution.
package secrets

import (
	"context"
	"strings"
)

// SecretFetcher fetches secret payloads for a SecretRef.
type SecretFetcher interface {
	GetSecret(ctx context.Context, ref *SecretRef) (string, error)
}

// ResolveString resolves a literal string or an sm:// reference using the
// provided SecretFetcher. If the string does not start with sm://, it is
// returned unchanged.
func ResolveString(ctx context.Context, fetcher SecretFetcher, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if !strings.HasPrefix(s, "sm://") {
		return s, nil
	}
	ref, err := parseSecretURI(s)
	if err != nil {
		return "", err
	}
	sv := &SecretValue{Ref: ref}
	return sv.Resolve(ctx, fetcher)
}
