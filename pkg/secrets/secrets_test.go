package secrets

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestParseSecretURI tests the parseSecretURI function with various URI formats
// including ARN-style secrets, query parameters, and region extraction.
func TestParseSecretURI(t *testing.T) {
	t.Run("basic URI formats", func(t *testing.T) {
		tests := []struct {
			in         string
			wantID     string
			wantKey    string
			wantRegion string
		}{
			{"sm://mysecret", "mysecret", "", ""},
			{"sm://mysecret#k1", "mysecret", "k1", ""},
			{"sm://mysecret?region=us-east-1", "mysecret", "", "us-east-1"},
			{"sm://mysecret#k?region=eu-west-1", "mysecret", "k", "eu-west-1"},
			{"sm://arn:aws:secretsmanager:us-west-2:123456789012:secret:foo", "arn:aws:secretsmanager:us-west-2:123456789012:secret:foo", "", "us-west-2"},
		}
		for _, tc := range tests {
			r, err := parseSecretURI(tc.in)
			if err != nil {
				t.Fatalf("parseSecretURI(%q) error: %v", tc.in, err)
			}
			if r.SecretID != tc.wantID {
				t.Fatalf("unexpected id for %q: got %q want %q", tc.in, r.SecretID, tc.wantID)
			}
			if r.Key != tc.wantKey {
				t.Fatalf("unexpected key for %q: got %q want %q", tc.in, r.Key, tc.wantKey)
			}
			if r.Region != tc.wantRegion {
				t.Fatalf("unexpected region for %q: got %q want %q", tc.in, r.Region, tc.wantRegion)
			}
		}
	})

	t.Run("region extraction from ARN and query params", func(t *testing.T) {
		cases := []struct {
			uri        string
			wantID     string
			wantKey    string
			wantRegion string
		}{
			{"sm://arn:aws:secretsmanager:us-east-2:123456:secret:my-secret#k", "arn:aws:secretsmanager:us-east-2:123456:secret:my-secret", "k", "us-east-2"},
			{"sm://my-secret?region=eu-west-1#k", "my-secret", "k", "eu-west-1"},
		}
		for _, c := range cases {
			r, err := parseSecretURI(c.uri)
			if err != nil {
				t.Fatalf("parseSecretURI(%s) failed: %v", c.uri, err)
			}
			if r.SecretID != c.wantID || r.Key != c.wantKey || r.Region != c.wantRegion {
				t.Fatalf("uri=%s got id=%s key=%s region=%s want id=%s key=%s region=%s", c.uri, r.SecretID, r.Key, r.Region, c.wantID, c.wantKey, c.wantRegion)
			}
		}
	})

	t.Run("parameter order and empty ID validation", func(t *testing.T) {
		cases := []string{
			"sm://id#k?region=eu-1",
			"sm://id?region=eu-1#k",
			"sm://arn:aws:secretsmanager:us-east-1:123:secret:foo#bar?stage=prod",
		}
		for _, c := range cases {
			if _, err := parseSecretURI(c); err != nil {
				t.Fatalf("parseSecretURI(%q) unexpected err: %v", c, err)
			}
		}

		// Test empty secret ID validation
		if _, err := parseSecretURI("sm://"); err == nil {
			t.Fatalf("expected error for empty secret id")
		}
	})
}

// TestSecretValue_String tests the String method for SecretValue.
func TestSecretValue_String(t *testing.T) {
	t.Run("basic string conversion", func(t *testing.T) {
		tests := []struct {
			name     string
			sv       *SecretValue
			expected string
		}{
			{
				name:     "nil SecretValue",
				sv:       nil,
				expected: "",
			},
			{
				name:     "both Raw and Ref nil",
				sv:       &SecretValue{},
				expected: "",
			},
			{
				name:     "Raw takes precedence over Ref",
				sv:       &SecretValue{Raw: "raw-value", Ref: &SecretRef{SecretID: "test"}},
				expected: "raw-value",
			},
			{
				name:     "Ref when Raw is empty",
				sv:       &SecretValue{Ref: &SecretRef{SecretID: "test-secret", Region: "us-east-1", Key: "password"}},
				expected: "sm://test-secret#password?region=us-east-1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.sv.String()
				if result != tt.expected {
					t.Errorf("String() = %q, want %q", result, tt.expected)
				}
			})
		}
	})

	t.Run("ref with various parameters", func(t *testing.T) {
		s := &SecretValue{Ref: &SecretRef{SecretID: "id", Key: "k", Region: "r", Stage: "st"}}
		out := s.String()
		if out == "" {
			t.Fatalf("expected non-empty string for ref with params")
		}

		// empty Raw and nil Ref should return empty
		var s2 *SecretValue
		if s2.String() != "" {
			t.Fatalf("expected empty string for nil SecretValue")
		}
	})
}

// TestResolveString tests the ResolveString wrapper function.
func TestResolveString(t *testing.T) {
	t.Run("literal and empty values", func(t *testing.T) {
		// empty
		got, err := ResolveString(context.Background(), &mockFetcher{store: map[string]string{"x": "x"}}, "")
		if err != nil {
			t.Fatalf("unexpected err for empty: %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}

		// literal
		got2, err := ResolveString(context.Background(), &mockFetcher{store: map[string]string{"x": "x"}}, "plain")
		if err != nil || got2 != "plain" {
			t.Fatalf("expected literal passthrough, got %q err=%v", got2, err)
		}
	})

	t.Run("sm URI resolution", func(t *testing.T) {
		mf := &mockFetcher{store: map[string]string{
			"s1": `{"mysql.user":"appuser"}`,
			"s2": `{"mysql.user":"literal"}`,
		}}

		// sm:// resolved via fetcher
		got3, err := ResolveString(context.Background(), mf, "sm://s1")
		if err != nil || got3 != `{"mysql.user":"appuser"}` {
			t.Fatalf("expected fetched secret, got %q err=%v", got3, err)
		}

		// sm:// resolution with fragment
		v2, err := ResolveString(context.Background(), mf, "sm://s1#mysql.user")
		if err != nil {
			t.Fatalf("ResolveString sm failed: %v", err)
		}
		if v2 != "appuser" {
			t.Fatalf("unexpected resolved value: %s", v2)
		}

		// literal full-key fallback: payload has top-level key literally 'mysql.user'
		sv := &SecretValue{Ref: &SecretRef{SecretID: "s2", Key: "mysql.user"}}
		v3, err := sv.Resolve(context.Background(), mf)
		if err != nil {
			t.Fatalf("SecretValue.Resolve literal fallback failed: %v", err)
		}
		if v3 != "literal" {
			t.Fatalf("unexpected literal fallback value: %s", v3)
		}
	})
}

// TestSecretValue_UnmarshalYAML tests YAML unmarshaling for SecretValue.
func TestSecretValue_UnmarshalYAML(t *testing.T) {
	t.Run("scalar forms", func(t *testing.T) {
		y := `a: plain
b: sm://foo#bar?region=eu-1
`
		var doc map[string]SecretValue
		if err := yaml.Unmarshal([]byte(y), &doc); err != nil {
			t.Fatalf("yaml unmarshal failed: %v", err)
		}
		if doc["a"].Raw != "plain" {
			t.Fatalf("scalar plain not captured")
		}
		if doc["b"].Ref == nil || doc["b"].Ref.SecretID != "foo" {
			t.Fatalf("sm:// scalar not parsed into ref")
		}
	})

	t.Run("mapping form", func(t *testing.T) {
		var sv SecretValue
		in := `secret:
  secret_id: my-mapping-secret
  key: nested.password
  region: us-east-2
`
		if err := yaml.Unmarshal([]byte(in), &sv); err != nil {
			t.Fatalf("yaml unmarshal failed: %v", err)
		}
		if !sv.IsRef() || sv.Ref == nil {
			t.Fatalf("expected SecretValue to be a ref after unmarshal; got %#v", sv)
		}
		if sv.Ref.SecretID != "my-mapping-secret" || sv.Ref.Key != "nested.password" || sv.Ref.Region != "us-east-2" {
			t.Fatalf("unexpected ref: %#v", sv.Ref)
		}
	})

	t.Run("unsupported node kind", func(t *testing.T) {
		var s SecretValue
		// create a sequence node to trigger the unsupported-kind branch
		node := &yaml.Node{Kind: yaml.SequenceNode}
		if err := s.UnmarshalYAML(node); err == nil {
			t.Fatalf("expected error for unsupported yaml node kind")
		}
	})
}

// TestMissingKeyError tests the MissingKeyError type
func TestMissingKeyError(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple key",
			key:      "password",
			expected: `key "password" not found in secret payload`,
		},
		{
			name:     "nested key",
			key:      "database.password",
			expected: `key "database.password" not found in secret payload`,
		},
		{
			name:     "empty key",
			key:      "",
			expected: `key "" not found in secret payload`,
		},
		{
			name:     "key with special chars",
			key:      "app-config.db_pass",
			expected: `key "app-config.db_pass" not found in secret payload`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &MissingKeyError{Key: tt.key}
			result := err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, expected %q", result, tt.expected)
			}
		})
	}

	// Test that it implements error interface
	var err error = &MissingKeyError{Key: "test"}
	if err.Error() == "" {
		t.Error("MissingKeyError should implement error interface")
	}
}
