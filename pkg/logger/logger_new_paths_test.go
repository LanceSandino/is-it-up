package logger

import (
	"bytes"
	"os"
	"testing"
)

// TestNew_CustomWriter_JSONAndConsole tests logger creation with custom writer and JSON output.
// Verifies that loggers can be created with custom writers and produce expected output.
func TestNew_CustomWriter_JSONAndConsole(t *testing.T) {
	var buf bytes.Buffer
	// custom writer + json output
	lg := New(&buf, true, false, "test")
	if lg == nil {
		t.Fatalf("expected logger instance")
	}
	// write a simple info to ensure writer path used
	lg.Infof("hello %s", "json")
	if buf.Len() == 0 {
		t.Fatalf("expected output written to custom writer for json path")
	}

	// console encoder path with custom writer
	buf.Reset()
	lg2 := New(&buf, false, false, "test")
	if lg2 == nil {
		t.Fatalf("expected logger instance for console")
	}
	lg2.Infof("hello %s", "console")
	if buf.Len() == 0 {
		t.Fatalf("expected output written to custom writer for console path")
	}
}

// TestNew_NilWriter_DoesNotPanic tests that logger creation with nil writer doesn't panic.
// Verifies that passing nil or os.Stdout as writer creates valid logger instances.
func TestNew_NilWriter_DoesNotPanic(t *testing.T) {
	// nil writer should build the default logger (writes to stdout/stderr)
	lg := New(nil, true, false, "test")
	if lg == nil {
		t.Fatalf("expected logger instance with nil writer")
	}
	// Also try passing os.Stdout explicitly; the New code treats os.Stdout specially.
	lg2 := New(os.Stdout, false, false, "test")
	if lg2 == nil {
		t.Fatalf("expected logger instance with os.Stdout")
	}
}

func TestGetServiceVersionPrecedence(t *testing.T) {
	t.Setenv("SERVICE_VERSION", "release-version")
	t.Setenv("GIT_SHA", "1234567890abcdef")
	if got := getServiceVersion("fallback"); got != "release-version" {
		t.Fatalf("service version = %q", got)
	}

	t.Setenv("SERVICE_VERSION", "")
	if got := getServiceVersion("fallback"); got != "12345678" {
		t.Fatalf("long git sha = %q", got)
	}
	t.Setenv("GIT_SHA", "short")
	if got := getServiceVersion("fallback"); got != "short" {
		t.Fatalf("short git sha = %q", got)
	}
	t.Setenv("GIT_SHA", "")
	if got := getServiceVersion("fallback"); got != "fallback" {
		t.Fatalf("fallback = %q", got)
	}
}
