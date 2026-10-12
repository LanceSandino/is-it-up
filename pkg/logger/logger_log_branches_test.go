package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestLog_Typed_SuccessAndFailure tests typed logging for both success and failure scenarios.
// Verifies that success cases use Infow path while failures use Warnw/Errorf paths.
func TestLog_Typed_SuccessAndFailure(t *testing.T) {
	// success case -> Infow path
	var buf bytes.Buffer
	lg := New(&buf, true, false, "test")
	ok := &helpersFakeLoggable{}
	lg.Log(ok)
	out := buf.String()
	if !strings.Contains(out, "test-check") || !strings.Contains(out, "pass") {
		// JSON output should contain "pass" for successful checks
		t.Fatalf("unexpected output for typed success: %s", out)
	}

	// failure case -> Warnw path
	buf.Reset()
	lgFail := New(&buf, true, true, "test") // debug true so details may be present
	fail := helpersTestResult{name: "svc", ok: false, msg: "failed", sev: "critical", ts: time.Now(), dur: time.Millisecond * 3, det: "extra"}
	lgFail.Log(fail)
	fout := buf.String()
	if !strings.Contains(fout, "svc") || !strings.Contains(fout, "critical") {
		t.Fatalf("unexpected output for typed failure: %s", fout)
	}
}

// TestLog_Reflection_PointerAndMissingFields tests reflection-based logging with pointers and missing fields.
// Verifies that the logger can handle struct pointers and gracefully handles missing optional fields.
func TestLog_Reflection_PointerAndMissingFields(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf, false, false, "test")

	// pointer to struct
	s := &struct {
		Name      string
		OK        bool
		Message   string
		Severity  string
		Timestamp time.Time
		Duration  time.Duration
	}{
		Name:      "ptr",
		OK:        true,
		Message:   "ok",
		Severity:  "info",
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 1,
	}
	lg.Log(s)
	if !strings.Contains(buf.String(), "ptr") {
		t.Fatalf("expected reflection pointer logging to include name: %s", buf.String())
	}

	// struct missing some fields (e.g., Details) should not panic and should still log name/status
	buf.Reset()
	partial := struct {
		Name string
		OK   bool
	}{Name: "partial", OK: false}
	lg.Log(partial)
	if !strings.Contains(buf.String(), "partial") {
		t.Fatalf("expected partial reflection logging to include name: %s", buf.String())
	}
}
