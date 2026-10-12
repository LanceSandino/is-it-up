package secrets

import (
	"context"
	"sync"
	"testing"
	"time"

	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// fakeClient implements secretsManagerAPI and counts calls.
type fakeClient struct {
	mu    sync.Mutex
	calls int
	resp  *sm.GetSecretValueOutput
	err   error
	delay time.Duration
}

func (f *fakeClient) GetSecretValue(ctx context.Context, params *sm.GetSecretValueInput, optFns ...func(*sm.Options)) (*sm.GetSecretValueOutput, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.resp, f.err
}

func (f *fakeClient) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestGetSecret_SingleflightDedupes(t *testing.T) {
	s := "value"
	fake := &fakeClient{resp: &sm.GetSecretValueOutput{SecretString: &s}, delay: 100 * time.Millisecond}
	fetcher := &AWSSecretsFetcher{client: fake}

	ref := &SecretRef{Provider: "aws-secrets-manager", SecretID: "concurrent"}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	results := make([]string, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			v, err := fetcher.GetSecret(context.Background(), ref)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = v
		}()
	}
	wg.Wait()

	// All goroutines should have received the expected value
	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d got error: %v", i, errs[i])
		}
		if results[i] != "value" {
			t.Fatalf("unexpected result[%d]=%q", i, results[i])
		}
	}

	// Ensure underlying client was called only once due to singleflight.
	if fake.Calls() != 1 {
		t.Fatalf("expected 1 underlying call, got %d", fake.Calls())
	}
}

func TestGetSecret_ExpiredCacheTriggersFetch(t *testing.T) {
	// shorten TTL for this test
	old := DefaultSecretCacheTTL
	DefaultSecretCacheTTL = 10 * time.Millisecond
	defer func() { DefaultSecretCacheTTL = old }()

	// seed an expired cache entry
	fetcher := &AWSSecretsFetcher{}
	fetcher.cache.Store("mysecret", cachedSecret{val: "old", ts: time.Now().Add(-time.Hour)})

	// fake client to return a fresh value
	s := "fresh"
	fake := &fakeClient{resp: &sm.GetSecretValueOutput{SecretString: &s}}
	fetcher.client = fake

	ref := &SecretRef{Provider: "aws-secrets-manager", SecretID: "mysecret"}
	v, err := fetcher.GetSecret(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "fresh" {
		t.Fatalf("expected fresh value, got %q", v)
	}
	if fake.Calls() != 1 {
		t.Fatalf("expected underlying client called once, got %d", fake.Calls())
	}
}

func TestResolve_MultipleKeysSingleFetch(t *testing.T) {
	// payload contains multiple keys; multiple resolvers should share one fetch
	payload := `{"a":"va","b":"vb","c":"vc"}`
	fake := &fakeClient{resp: &sm.GetSecretValueOutput{SecretString: &payload}, delay: 50 * time.Millisecond}
	fetcher := &AWSSecretsFetcher{client: fake}

	keys := []string{"a", "b", "c", "a", "b"}
	var wg sync.WaitGroup
	wg.Add(len(keys))
	results := make([]string, len(keys))
	errs := make([]error, len(keys))

	for i, k := range keys {
		i := i
		k := k
		go func() {
			defer wg.Done()
			sv := &SecretValue{Ref: &SecretRef{Provider: "aws-secrets-manager", SecretID: "multi", Key: k}}
			v, err := sv.Resolve(context.Background(), fetcher)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = v
		}()
	}
	wg.Wait()

	for i := range keys {
		if errs[i] != nil {
			t.Fatalf("unexpected error at %d: %v", i, errs[i])
		}
	}
	// Check that values match expected mapping
	expect := map[string]string{"a": "va", "b": "vb", "c": "vc"}
	for i, k := range keys {
		if results[i] != expect[k] {
			t.Fatalf("unexpected resolved value for key %s: got %q want %q", k, results[i], expect[k])
		}
	}

	if fake.Calls() != 1 {
		t.Fatalf("expected single underlying call, got %d", fake.Calls())
	}
}
