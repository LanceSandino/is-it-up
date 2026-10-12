package secrets

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSecretValue_Resolve_NilReceiverAndFetcherNil tests edge cases for SecretValue.Resolve()
// when the receiver is nil or the fetcher is nil with a reference present.
func TestSecretValue_Resolve_NilReceiverAndFetcherNil(t *testing.T) {
	var s *SecretValue
	// nil receiver should return empty without error
	got, err := s.Resolve(context.Background(), &mockFetcher{store: map[string]string{"x": "x"}})
	if err != nil {
		t.Fatalf("expected nil receiver to not error, got %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string for nil receiver, got %q", got)
	}

	// fetcher nil should produce an error when Ref present
	s2 := &SecretValue{Ref: &SecretRef{SecretID: "s"}}
	_, err2 := s2.Resolve(context.Background(), nil)
	if err2 == nil {
		t.Fatalf("expected error when fetcher is nil")
	}
}

// TestSecretValue_Resolve_NonStringMarshalsToYAML tests that non-string secret values
// are properly marshaled to YAML format when resolved.
func TestSecretValue_Resolve_NonStringMarshalsToYAML(t *testing.T) {
	payload := `{"obj":{"inner":{"v":123}}}`
	f := &mockFetcher{store: map[string]string{"x": payload}}
	sv := &SecretValue{Ref: &SecretRef{SecretID: "x", Key: "obj.inner"}}
	got, err := sv.Resolve(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// result should be YAML-ish and contain the numeric field
	if got == "" || (!contains(got, "v") && !contains(got, "123")) {
		t.Fatalf("unexpected marshaled value: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestSecretValue_Unmarshal_InvalidMappingReturnsError tests that UnmarshalYAML properly
// returns an error when given an invalid mapping structure.
func TestSecretValue_Unmarshal_InvalidMappingReturnsError(t *testing.T) {
	y := `bad: { a: b }`
	var doc map[string]SecretValue
	err := yaml.Unmarshal([]byte(y), &doc)
	if err == nil {
		t.Fatalf("expected unmarshal to fail for invalid secret mapping")
	}
}

// TestParseSecretURI_EmptyIDError tests that parseSecretURI returns an error
// when the secret ID portion of the URI is empty.
func TestParseSecretURI_EmptyIDError(t *testing.T) {
	if _, err := parseSecretURI("sm://"); err == nil {
		t.Fatalf("expected error for empty secret id in URI")
	}
}
