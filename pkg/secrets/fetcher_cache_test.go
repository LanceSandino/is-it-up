package secrets

import (
	"context"
	"testing"
	"time"
)

// TestAWSFetcher_ReturnsCachedValue ensures that when the internal cache
// contains a fresh value, GetSecret returns it without attempting any
// network calls. The test constructs a fetcher with a nil AWS client and
// pre-populates the cache.
func TestAWSFetcher_ReturnsCachedValue(t *testing.T) {
	f := &AWSSecretsFetcher{}
	// seed cache with a recent timestamp
	f.cache.Store("mysecret|stage=", cachedSecret{val: "cached-data", ts: time.Now()})

	ref := &SecretRef{Provider: "aws-secrets-manager", SecretID: "mysecret"}
	v, err := f.GetSecret(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "cached-data" {
		t.Fatalf("expected cached-data, got %q", v)
	}
}
