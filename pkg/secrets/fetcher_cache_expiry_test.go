package secrets

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// TestAWSFetcher_CacheExpiry verifies that cached values are served until the
// TTL expires and that the underlying client is invoked again after expiry.
func TestAWSFetcher_CacheExpiry(t *testing.T) {
	// shorten TTL for deterministic test
	orig := DefaultSecretCacheTTL
	DefaultSecretCacheTTL = 50 * time.Millisecond
	defer func() { DefaultSecretCacheTTL = orig }()

	// reuse existing fakeSMClient defined in fetcher_aws_test.go
	fake := &fakeSMClient{store: map[string]string{"mysecret": `{"k":"v"}`}}
	f := &AWSSecretsFetcher{client: fake, baseCfg: aws.Config{Region: "us-west-2"}}

	ref := &SecretRef{Provider: "aws-secrets-manager", SecretID: "mysecret"}

	ctx := context.Background()

	v1, err := f.GetSecret(ctx, ref)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if v1 != `{"k":"v"}` {
		t.Fatalf("unexpected payload first fetch: %q", v1)
	}
	if fake.calls != 1 {
		t.Fatalf("expected underlying client called once, got %d", fake.calls)
	}

	// immediate second fetch should hit cache and not increment underlying calls
	v2, err := f.GetSecret(ctx, ref)
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if v2 != v1 {
		t.Fatalf("cached value mismatch: %q vs %q", v1, v2)
	}
	if fake.calls != 1 {
		t.Fatalf("expected cached hit (1 call), got %d", fake.calls)
	}

	// wait for TTL to expire then fetch again
	time.Sleep(75 * time.Millisecond)
	v3, err := f.GetSecret(ctx, ref)
	if err != nil {
		t.Fatalf("third fetch failed: %v", err)
	}
	if v3 != v1 {
		t.Fatalf("value after expiry mismatch: %q vs %q", v3, v1)
	}
	if fake.calls != 2 {
		t.Fatalf("expected underlying client called again after expiry, got %d", fake.calls)
	}
}
