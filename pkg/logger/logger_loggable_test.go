package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestLog_WithLoggable_JSONAndConsole tests logging with objects implementing the Loggable interface.
// Verifies that objects with GetLogFields() method are properly logged in both JSON and console modes.
func TestLog_WithLoggable_JSONAndConsole(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf, true, true, "test") // JSON + debug
	// Log using Loggable interface
	lg.Log(&helpersFakeLoggable{})
	out := buf.String()
	if !strings.Contains(out, "test-check") || !strings.Contains(out, "all good") {
		t.Fatalf("unexpected JSON output: %s", out)
	}

	// Console mode using reflection path
	buf.Reset()
	lg2 := New(&buf, false, false, "test")
	s := struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
		Details   string
	}{
		Name:      "ref-check",
		OK:        false,
		Message:   "bad",
		Severity:  "critical",
		Timestamp: time.Unix(0, 0),
		Duration:  2500 * time.Millisecond,
		Details:   "x",
	}
	lg2.Log(&s)
	out2 := buf.String()
	if !strings.Contains(out2, "ref-check") || !strings.Contains(out2, "FAIL") {
		t.Fatalf("unexpected console output: %s", out2)
	}
}
