package secrets

import (
	"context"
	"testing"
)

// TestSecretValue_Resolve tests the main SecretValue.Resolve method with comprehensive scenarios
// including nested keys, missing keys, non-JSON payloads, and edge cases.
func TestSecretValue_Resolve(t *testing.T) {
	ctx := context.Background()

	t.Run("edge cases", func(t *testing.T) {
		// nil SecretValue
		var sv *SecretValue
		result, err := sv.Resolve(ctx, nil)
		if err != nil {
			t.Errorf("expected no error for nil SecretValue, got %v", err)
		}
		if result != "" {
			t.Errorf("expected empty string for nil SecretValue, got %q", result)
		}

		// Raw value returns directly
		sv = &SecretValue{Raw: "direct-value"}
		result, err = sv.Resolve(ctx, nil)
		if err != nil {
			t.Errorf("expected no error for Raw value, got %v", err)
		}
		if result != "direct-value" {
			t.Errorf("expected 'direct-value', got %q", result)
		}

		// nil Ref returns empty
		sv = &SecretValue{}
		result, err = sv.Resolve(ctx, nil)
		if err != nil {
			t.Errorf("expected no error for nil Ref, got %v", err)
		}
		if result != "" {
			t.Errorf("expected empty string for nil Ref, got %q", result)
		}

		// nil fetcher returns error
		sv = &SecretValue{Ref: &SecretRef{SecretID: "test"}}
		result, err = sv.Resolve(ctx, nil)
		if err == nil {
			t.Errorf("expected error for nil fetcher")
		}
		if result != "" {
			t.Errorf("expected empty result on error, got %q", result)
		}
	})

	t.Run("nested key extraction", func(t *testing.T) {
		// nested mapping payload
		payload := `{"credentials": {"user":"bob","password":"s3cr3t"}}`
		f := &mockFetcher{store: map[string]string{"s1": payload}}

		sv := &SecretValue{Ref: &SecretRef{SecretID: "s1", Key: "credentials.password"}}
		got, err := sv.Resolve(ctx, f)
		if err != nil {
			t.Fatalf("unexpected error resolving nested key: %v", err)
		}
		if got != "s3cr3t" {
			t.Fatalf("unexpected nested value: %q", got)
		}

		// Test successful nested resolution with new keys
		sv2 := &SecretValue{Ref: &SecretRef{SecretID: "s1", Key: "credentials.user"}}
		got2, err := sv2.Resolve(ctx, f)
		if err != nil {
			t.Fatalf("unexpected error resolving nested user: %v", err)
		}
		if got2 != "bob" {
			t.Fatalf("unexpected nested user value: %q", got2)
		}
	})

	t.Run("missing key error", func(t *testing.T) {
		// missing nested key -> MissingKeyError
		f := &mockFetcher{store: map[string]string{"s2": `{"credentials": {"user":"bob"}}`}}
		sv := &SecretValue{Ref: &SecretRef{SecretID: "s2", Key: "credentials.password"}}
		_, err := sv.Resolve(ctx, f)
		if err == nil {
			t.Fatalf("expected MissingKeyError but got nil")
		}
		if _, ok := err.(*MissingKeyError); !ok {
			t.Fatalf("expected MissingKeyError but got %T: %v", err, err)
		}

		// Test with missing key from mock fetcher that returns simple JSON
		mockFetcherMissing := &mockFetcher{store: map[string]string{"s": `{"present":"value"}`}}
		sv = &SecretValue{Ref: &SecretRef{SecretID: "s", Key: "absent.key"}}
		_, err = sv.Resolve(ctx, mockFetcherMissing)
		if err == nil {
			t.Fatalf("expected missing key error")
		}
		if _, ok := err.(*MissingKeyError); !ok {
			t.Fatalf("expected MissingKeyError, got %T: %v", err, err)
		}
	})

	t.Run("non-JSON payload with key", func(t *testing.T) {
		// Use mockFetcher with plain text payload
		fetcher := &mockFetcher{
			store: map[string]string{
				"plain-secret": "this is not json or yaml",
			},
		}

		sv := &SecretValue{
			Ref: &SecretRef{
				SecretID: "plain-secret",
				Key:      "some.key",
			},
		}

		result, err := sv.Resolve(ctx, fetcher)
		if err == nil {
			t.Errorf("expected error when parsing non-JSON payload with key, got result: %q", result)
		}
		if result != "" {
			t.Errorf("expected empty result on parse error, got %q", result)
		}
	})

	t.Run("no key returns full payload", func(t *testing.T) {
		f := &mockFetcher{store: map[string]string{"s4": "the-full-secret"}}
		sv := &SecretValue{Ref: &SecretRef{SecretID: "s4", Key: ""}}
		got, err := sv.Resolve(ctx, f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "the-full-secret" {
			t.Fatalf("expected full payload returned, got %q", got)
		}
	})

	t.Run("literal fallback for full key", func(t *testing.T) {
		// payload has top-level key literally 'mysql.user'
		mf := &mockFetcher{store: map[string]string{
			"s2": `{"mysql.user":"literal"}`,
		}}

		sv := &SecretValue{Ref: &SecretRef{SecretID: "s2", Key: "mysql.user"}}
		v3, err := sv.Resolve(ctx, mf)
		if err != nil {
			t.Fatalf("SecretValue.Resolve literal fallback failed: %v", err)
		}
		if v3 != "literal" {
			t.Fatalf("unexpected literal fallback value: %s", v3)
		}
	})
}

// TestSecretValue_BasicFunctionality tests basic SecretValue operations
// including the helper methods (IsEmpty, HasValue, IsRef).
func TestSecretValue_BasicFunctionality(t *testing.T) {
	t.Run("helper methods", func(t *testing.T) {
		// Test IsEmpty, HasValue, IsRef on various SecretValue states

		// nil SecretValue
		var sv *SecretValue
		if !sv.IsEmpty() {
			t.Errorf("nil SecretValue should be empty")
		}
		if sv.HasValue() {
			t.Errorf("nil SecretValue should not have value")
		}
		if sv.IsRef() {
			t.Errorf("nil SecretValue should not be ref")
		}

		// Empty SecretValue
		sv = &SecretValue{}
		if !sv.IsEmpty() {
			t.Errorf("empty SecretValue should be empty")
		}
		if sv.HasValue() {
			t.Errorf("empty SecretValue should not have value")
		}
		if sv.IsRef() {
			t.Errorf("empty SecretValue should not be ref")
		}

		// Raw value SecretValue
		sv = &SecretValue{Raw: "test-value"}
		if sv.IsEmpty() {
			t.Errorf("raw SecretValue should not be empty")
		}
		if !sv.HasValue() {
			t.Errorf("raw SecretValue should have value")
		}
		if sv.IsRef() {
			t.Errorf("raw SecretValue should not be ref")
		}

		// Ref SecretValue
		sv = &SecretValue{Ref: &SecretRef{SecretID: "test"}}
		if sv.IsEmpty() {
			t.Errorf("ref SecretValue should not be empty")
		}
		if !sv.HasValue() {
			t.Errorf("ref SecretValue should have value")
		}
		if !sv.IsRef() {
			t.Errorf("ref SecretValue should be ref")
		}
	})
}
