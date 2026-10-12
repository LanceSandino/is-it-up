package secrets

import (
	"context"
	"fmt"
	"testing"
)

type mockFetcher struct {
	store map[string]string
}

func (m *mockFetcher) GetSecret(ctx context.Context, ref *SecretRef) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("nil ref")
	}
	v, ok := m.store[ref.SecretID]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", ref.SecretID)
	}
	return v, nil
}

func TestResolveSecretValue(t *testing.T) {
	mf := &mockFetcher{store: map[string]string{
		"mysecret": `{"password":"hunter2","nested":{"k":"v"}}`,
	}}
	sv := &SecretValue{Ref: &SecretRef{SecretID: "mysecret", Key: "password"}}
	v, err := sv.Resolve(context.Background(), mf)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if v != "hunter2" {
		t.Fatalf("unexpected value: %s", v)
	}

	// test nested object extraction
	sv2 := &SecretValue{Ref: &SecretRef{SecretID: "mysecret", Key: "nested.k"}}
	v2, err := sv2.Resolve(context.Background(), mf)
	if err != nil {
		t.Fatalf("resolve nested failed: %v", err)
	}
	if v2 != "v" {
		t.Fatalf("unexpected nested value: %s", v2)
	}
}
