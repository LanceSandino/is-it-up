package logger

import (
	"bytes"
	"strings"
	"testing"
)

// TestInfof_Warnf_Errorf_WriteToCustomWriter tests the formatters.
// Verifies that Infof, Warnf, and Errorf properly format messages and write to the custom writer.
func TestInfof_Warnf_Errorf_WriteToCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf, false, false, "test")
	lg.Infof("hello %s", "info")
	if !strings.Contains(buf.String(), "hello info") {
		t.Fatalf("expected Infof to write formatted message, got: %s", buf.String())
	}

	buf.Reset()
	lg.Warnf("warn %d", 7)
	if !strings.Contains(buf.String(), "warn 7") {
		t.Fatalf("expected Warnf to write formatted message, got: %s", buf.String())
	}

	buf.Reset()
	lg.Errorf("err %s", "boom")
	if !strings.Contains(buf.String(), "err boom") {
		t.Fatalf("expected Errorf to write formatted message, got: %s", buf.String())
	}
}

// TestDebugf tests the Debugf formatted logging method
func TestDebugf_WriteToCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf, false, true, "test") // Enable debug mode

	// Test basic Debugf functionality
	lg.Debugf("debug message: %s %d", "test", 42)
	if !strings.Contains(buf.String(), "debug message: test 42") {
		t.Fatalf("expected Debugf to write formatted message, got: %s", buf.String())
	}

	// Test Debugf with JSON format
	buf.Reset()
	lgJSON := New(&buf, true, true, "test") // JSON format with debug
	lgJSON.Debugf("json debug: %s", "value")

	output := buf.String()
	if !strings.Contains(output, "json debug: value") {
		t.Fatalf("expected JSON Debugf to write formatted message, got: %s", output)
	}

	// Should contain service metadata for JSON
	if !strings.Contains(output, "service.name") {
		t.Fatalf("expected JSON Debugf to include service metadata, got: %s", output)
	}

	// Test Debugf with nil logger (should not panic)
	var nilLogger *Logger
	nilLogger.Debugf("should not panic")
}
