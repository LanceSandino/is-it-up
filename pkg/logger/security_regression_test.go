// This file anchors terminal-safety behavior for peer-controlled diagnostics.
package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type hostileLoggable struct {
	message string
	details string
}

func (h hostileLoggable) GetName() string            { return "hostile\rname" }
func (h hostileLoggable) IsOK() bool                 { return false }
func (h hostileLoggable) GetMessage() string         { return h.message }
func (h hostileLoggable) GetSeverity() string        { return "warning" }
func (h hostileLoggable) GetTimestamp() time.Time    { return time.Unix(0, 0).UTC() }
func (h hostileLoggable) GetDuration() time.Duration { return time.Millisecond }
func (h hostileLoggable) GetDetails() string         { return h.details }

func TestConsoleEscapesUntrustedControls(t *testing.T) {
	var output bytes.Buffer
	log := New(&output, false, true, "test")
	log.Log(hostileLoggable{
		message: "bad\x1b[2J\r\nnext\u0085line\u2028split\xff",
		details: "detail\x00value",
	})

	got := output.String()
	for _, raw := range []string{"\x1b", "\r", "\x00", "\u0085", "\u2028", string([]byte{0xff})} {
		if strings.Contains(got, raw) {
			t.Fatalf("console output retained control %q: %q", raw, got)
		}
	}
	for _, escaped := range []string{`\x1b`, `\r`, `\n`, `\u0085`, `\u2028`, `\x00`} {
		if !strings.Contains(got, escaped) {
			t.Fatalf("console output missing escape %q: %q", escaped, got)
		}
	}
}
