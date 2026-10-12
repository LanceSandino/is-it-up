package checks

import (
	"strings"
	"testing"
)

func TestRedactAddress_RemovesEmbeddedCredentials(t *testing.T) {
	tests := []string{
		"https://alice:topsecret@example.com/health",
		"mongodb+srv://alice:topsecret@cluster.example.com/admin",
		"alice:topsecret@tcp(db.example.com:3306)/app",
		"host=db.example.com user=alice password=topsecret dbname=app",
	}
	for _, input := range tests {
		got := redactAddress(input)
		if strings.Contains(got, "topsecret") || strings.Contains(got, "alice:") || strings.Contains(got, "password=") {
			t.Fatalf("redaction failed for %q: %q", input, got)
		}
	}
}
