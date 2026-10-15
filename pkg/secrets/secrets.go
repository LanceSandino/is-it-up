// This file parses secret references and coordinates their resolution.
// It depends on the fetcher interface while keeping provider details outside callers.
package secrets

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// MissingKeyError is returned when a referenced key is not present in a
// structured secret payload.
type MissingKeyError struct {
	Key string
}

func (m *MissingKeyError) Error() string {
	return fmt.Sprintf("key %q not found in secret payload", m.Key)
}

// SecretRef is a compact reference to an external secret (e.g. AWS Secrets Manager)
type SecretRef struct {
	Provider string `yaml:"provider" json:"provider"`
	SecretID string `yaml:"secret_id" json:"secret_id"`
	Key      string `yaml:"key,omitempty" json:"key,omitempty"`
	Region   string `yaml:"region,omitempty" json:"region,omitempty"`
	Stage    string `yaml:"stage,omitempty" json:"stage,omitempty"`
}

func (r *SecretRef) String() string {
	if r == nil {
		return ""
	}
	base := "sm://" + r.SecretID
	if r.Key != "" {
		base += "#" + r.Key
	}
	parts := []string{}
	if r.Region != "" {
		parts = append(parts, "region="+r.Region)
	}
	if r.Stage != "" {
		parts = append(parts, "stage="+r.Stage)
	}
	if len(parts) > 0 {
		base += "?" + strings.Join(parts, "&")
	}
	return base
}

// SecretValue holds either a raw literal or a SecretRef reference.
type SecretValue struct {
	Raw       string
	Ref       *SecretRef
	Sensitive bool `json:"-" yaml:"-"`
}

// IsEmpty reports whether the SecretValue has no literal or reference.
func (s *SecretValue) IsEmpty() bool { return s == nil || (s.Raw == "" && s.Ref == nil) }

// HasValue is the inverse of IsEmpty.
func (s *SecretValue) HasValue() bool { return !s.IsEmpty() }

// IsRef reports whether this SecretValue is a reference that requires
// runtime resolution (e.g. sm://... or mapping form).
func (s *SecretValue) IsRef() bool { return s != nil && s.Ref != nil }

func (s *SecretValue) String() string {
	if s == nil {
		return ""
	}
	if s.Raw != "" {
		return s.Raw
	}
	if s.Ref != nil {
		return s.Ref.String()
	}
	return ""
}

// Resolve fetches the secret value (or returns Raw) using the provided fetcher.
// If SecretValue is a reference, it asks fetcher for the secret payload and
// optionally extracts a nested key (dot-separated) from JSON payloads.
// Resolve fetches the secret payload when this SecretValue is a reference.
// If the SecretValue contains a Raw literal the literal is returned.
// Resolution uses the provided SecretFetcher and does not perform any
// network operations during YAML unmarshalling (unmarshal remains pure).
//
// Behavior:
//   - If no key is requested, the full payload (SecretString or SecretBinary)
//     is returned.
//   - If a dotted key is provided (e.g. credentials.password), the payload is
//     parsed as JSON/YAML and the nested value is extracted. If nested lookup
//     fails, a fallback to a top-level literal key is attempted before returning
//     a MissingKeyError.
func (s *SecretValue) Resolve(ctx context.Context, fetcher SecretFetcher) (string, error) {
	if s == nil {
		return "", nil
	}
	if s.Raw != "" {
		return s.Raw, nil
	}
	if s.Ref == nil {
		return "", nil
	}
	if fetcher == nil {
		return "", fmt.Errorf("no secret fetcher provided for resolving secret ref")
	}
	payload, err := fetcher.GetSecret(ctx, s.Ref)
	if err != nil {
		return "", err
	}
	// If no key requested, return full payload
	if s.Ref.Key == "" {
		return payload, nil
	}
	// Try to parse JSON and extract dotted key path
	var doc interface{}
	if err := yaml.Unmarshal([]byte(payload), &doc); err != nil {
		// not JSON/YAML or parse failed; return error
		return "", fmt.Errorf("failed to parse secret payload as JSON/YAML to extract key: %w", err)
	}
	// traverse dotted path
	parts := strings.Split(s.Ref.Key, ".")
	cur := doc
	originalKey := s.Ref.Key
	for idx, p := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			// traversal failed because current node isn't a map; try fallback:
			// check top-level map for the full originalKey as a literal.
			if top, ok := doc.(map[string]interface{}); ok {
				if v, exists := top[originalKey]; exists {
					cur = v
					break
				}
			}
			return "", fmt.Errorf("key %q not found (not a mapping)", p)
		}
		nxt, ok := m[p]
		if !ok {
			// If missing, try literal full-key lookup at top level before failing.
			if top, ok := doc.(map[string]interface{}); ok {
				if v, exists := top[originalKey]; exists {
					cur = v
					break
				}
			}
			return "", &MissingKeyError{Key: originalKey}
		}
		cur = nxt
		// if this was the last part, loop ends normally
		_ = idx
	}
	// final value should be scalar
	switch v := cur.(type) {
	case string:
		return v, nil
	default:
		// marshal back to string
		b, err := yaml.Marshal(cur)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
}

// UnmarshalYAML supports scalar strings (raw or sm:// URI) and mapping forms.
func (s *SecretValue) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var v string
		if err := node.Decode(&v); err != nil {
			return err
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "sm://") {
			ref, err := parseSecretURI(v)
			if err != nil {
				return err
			}
			s.Ref = ref
			s.Sensitive = true
			return nil
		}
		s.Raw = v
		return nil
	case yaml.MappingNode:
		var m map[string]interface{}
		if err := node.Decode(&m); err != nil {
			return err
		}
		if sec, ok := m["secret"]; ok {
			b, _ := yaml.Marshal(sec)
			var r SecretRef
			if err := yaml.Unmarshal(b, &r); err != nil {
				return err
			}
			if r.Provider == "" {
				r.Provider = "aws-secrets-manager"
			}
			s.Ref = &r
			s.Sensitive = true
			return nil
		}
		b, _ := yaml.Marshal(m)
		var r2 SecretRef
		if err := yaml.Unmarshal(b, &r2); err == nil && r2.SecretID != "" {
			if r2.Provider == "" {
				r2.Provider = "aws-secrets-manager"
			}
			s.Ref = &r2
			s.Sensitive = true
			return nil
		}
		return fmt.Errorf("invalid secret mapping")
	default:
		return fmt.Errorf("unsupported yaml node kind for secret value: %v", node.Kind)
	}
}

// parseSecretURI parses compact sm:// URIs into a SecretRef without performing any network I/O.
func parseSecretURI(u string) (*SecretRef, error) {
	// Avoid url.Parse because secret IDs (ARNs) may contain ':' which url.Parse
	// can interpret as an invalid port. Use a simple, local parse:
	if !strings.HasPrefix(u, "sm://") {
		return nil, fmt.Errorf("unsupported secret scheme or malformed URI: %s", u)
	}
	body := strings.TrimPrefix(u, "sm://")
	// Locate positions of '?' and '#' and slice accordingly so we support
	// both sm://id#key?region=... and sm://id?region=...#key forms.
	var key string
	var region, stage string
	qi := strings.Index(body, "?")
	hi := strings.Index(body, "#")
	var qstr string
	switch {
	case qi == -1 && hi == -1:
		// nothing to do
	case qi == -1 && hi != -1:
		// only fragment
		key = body[hi+1:]
		body = body[:hi]
	case qi != -1 && hi == -1:
		// only query
		qstr = body[qi+1:]
		body = body[:qi]
	default:
		// both present; determine order
		if qi < hi {
			// query appears before fragment: id?query#key
			qstr = body[qi+1 : hi]
			key = body[hi+1:]
			body = body[:qi]
		} else {
			// fragment appears before query: id#key?query
			key = body[hi+1 : qi]
			qstr = body[qi+1:]
			body = body[:hi]
		}
	}
	if qstr != "" {
		vals, _ := url.ParseQuery(qstr)
		region = vals.Get("region")
		stage = vals.Get("stage")
	}
	secretID := body
	if secretID == "" {
		return nil, fmt.Errorf("empty secret id in URI: %s", u)
	}
	// If the secretID is an ARN, extract the region component so callers
	// can construct region-scoped clients. ARN format: arn:partition:service:region:account:resource
	if strings.HasPrefix(secretID, "arn:") {
		// split into at most 6 parts to preserve resource section
		parts := strings.SplitN(secretID, ":", 6)
		if len(parts) >= 6 {
			// parts[3] is the region
			region := parts[3]
			if region != "" {
				return &SecretRef{Provider: "aws-secrets-manager", SecretID: secretID, Key: key, Region: region, Stage: stage}, nil
			}
		}
	}
	return &SecretRef{Provider: "aws-secrets-manager", SecretID: secretID, Key: key, Region: region, Stage: stage}, nil
}
