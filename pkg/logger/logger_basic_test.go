package logger

import (
	"bytes"
	"context"
	"io"
	"testing"
)

// testNew creates a logger for testing with a default version
func testNew(w io.Writer, jsonOut bool, debug bool) *Logger {
	return New(w, jsonOut, debug, "test")
}

// TestInfow_Debugw_Infof_Wraps_Basic tests the basic wrapper methods of the logger.
// Verifies that Infow, Debugw, Infof, Warnf, and Errorf methods properly generate output.
func TestInfow_Debugw_Infof_Wraps_Basic(t *testing.T) {
	var buf bytes.Buffer
	lg := testNew(&buf, true, false)
	lg.Infow("info msg", "k", "v")
	lg.Debugw("debug msg", "k", "v")
	lg.Infof("infof %s", "ok")
	lg.Warnf("warn %s", "ok")
	lg.Errorf("err %s", "ok")

	if buf.Len() == 0 {
		t.Fatalf("expected logger to write output for wrapper methods")
	}
}

// TestNew_FromContext_IsDebug_Basic tests logger creation, context storage, and debug flag.
// Verifies that loggers can be stored in and retrieved from context and IsDebug works correctly.
func TestNew_FromContext_IsDebug_Basic(t *testing.T) {
	var buf bytes.Buffer
	lg := testNew(&buf, true, true)
	ctx := WithLogger(context.Background(), lg)
	got := FromContext(ctx)
	if got == nil {
		t.Fatalf("expected logger in context")
	}
	if !got.IsDebug() {
		t.Fatalf("expected IsDebug true")
	}
}

// TestInfow_Debugw_InfoWarnError_NoPanic_Basic tests that logger methods don't panic.
// Verifies that all logging methods execute successfully without causing runtime errors.
func TestInfow_Debugw_InfoWarnError_NoPanic_Basic(t *testing.T) {
	var buf bytes.Buffer
	lg := testNew(&buf, false, false)
	lg.Infow("msg", "k", "v")
	lg.Debugw("dbg", "k", "v")
	lg.Infof("fmt %s", "ok")
	lg.Warnf("warn %s", "w")
	lg.Errorf("err %s", "e")
	if buf.Len() == 0 {
		t.Fatalf("expected output from logger methods")
	}
}
