package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestLoggerJSONDebugDetails tests JSON logging with debug information.
// Verifies that structured logging outputs valid JSON with all expected fields.
func TestLoggerJSONDebugDetails(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, true, "test") // json + debug
	type result struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
		Details   string
	}
	res := result{Name: "test", OK: false, Message: "failed", Severity: "critical", Timestamp: time.Now(), Duration: time.Millisecond * 5, Details: "secret=REDACTED"}
	l.Log(res)
	j := buf.String()
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(j), &data); err != nil {
		t.Fatalf("failed to parse json output: %v, raw: %s", err, j)
	}
	if _, ok := data["check_name"]; !ok {
		t.Fatalf("expected check_name in json output: %s", j)
	}
}

// TestLoggerJSONDetailsAndStacktrace tests JSON logging with details and stack trace information.
// Verifies that non-debug JSON logging still captures details properly.
func TestLoggerJSONDetailsAndStacktrace(t *testing.T) {
	buf := &bytes.Buffer{}
	// non-debug logger
	l := New(buf, true, false, "test")
	type result struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
		Details   string
	}
	res := result{
		Name:      "test",
		OK:        false,
		Message:   "failed",
		Severity:  "critical",
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 5,
		Details:   "secret=REDACTED",
	}
	l.Log(res)
	out := buf.String()
	if strings.Contains(out, "secret=REDACTED") {
		t.Fatalf("details should not be present in JSON when not in debug mode: %s", out)
	}
	if strings.Contains(out, "stacktrace") {
		t.Fatalf("stacktrace should not be present in non-debug mode: %s", out)
	}

	// debug logger should include details
	buf.Reset()
	dl := New(buf, true, true, "test")
	dl.Log(res)
	j := buf.String()
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(j), &data); err != nil {
		t.Fatalf("failed to parse json output: %v, raw: %s", err, j)
	}
	if _, ok := data["check_details"]; !ok {
		t.Fatalf("expected check_details to be present in debug JSON: %s", j)
	}
}
