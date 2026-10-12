package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestLog_Reflection_MissingDetails ensures reflection fallback logs structs
// that are missing the Details field and does not panic; verifies name is present.
func TestLog_Reflection_MissingDetails(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, false, false, "test") // console, non-debug

	// Struct intentionally missing Details field
	s := struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
	}{
		Name:      "rmissing",
		OK:        false,
		Message:   "missing details",
		Severity:  "warn",
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 2,
	}

	l.Log(s)
	out := buf.String()
	if out == "" {
		t.Fatalf("expected output for reflection logging, got empty")
	}
	if !contains(out, "rmissing") {
		t.Fatalf("expected reflected name in output: %s", out)
	}
}

// TestLog_Reflection_PointerWithDetails_Debug exercises pointer-to-struct
// reflection path when debug is enabled and Details is present; verifies details appear.
func TestLog_Reflection_PointerWithDetails_Debug(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, true, "test") // json + debug to include details

	p := &struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
		Details   string
	}{
		Name:      "rptr",
		OK:        false,
		Message:   "ptr details",
		Severity:  "critical",
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 7,
		Details:   "sensitive=REDACTED",
	}

	l.Log(p)
	out := buf.String()
	if out == "" {
		t.Fatalf("expected output for reflection pointer logging, got empty")
	}
	if !contains(out, "rptr") {
		t.Fatalf("expected name present in output: %s", out)
	}
	if !contains(out, "sensitive=REDACTED") {
		t.Fatalf("expected details present in debug json output: %s", out)
	}
}
