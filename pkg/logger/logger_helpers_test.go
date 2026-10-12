package logger

import "time"

// helpersFakeLoggable implements the Loggable interface for tests.
type helpersFakeLoggable struct{}

func (f *helpersFakeLoggable) GetName() string            { return "test-check" }
func (f *helpersFakeLoggable) IsOK() bool                 { return true }
func (f *helpersFakeLoggable) GetMessage() string         { return "all good" }
func (f *helpersFakeLoggable) GetSeverity() string        { return "info" }
func (f *helpersFakeLoggable) GetTimestamp() time.Time    { return time.Unix(0, 0) }
func (f *helpersFakeLoggable) GetDuration() time.Duration { return 1500 * time.Millisecond }
func (f *helpersFakeLoggable) GetDetails() string         { return "details here" }

// helpersTestResult implements the Loggable interface for typed logging tests.
type helpersTestResult struct {
	name string
	ok   bool
	msg  string
	sev  string
	ts   time.Time
	dur  time.Duration
	det  string
}

func (t helpersTestResult) GetName() string            { return t.name }
func (t helpersTestResult) IsOK() bool                 { return t.ok }
func (t helpersTestResult) GetMessage() string         { return t.msg }
func (t helpersTestResult) GetSeverity() string        { return t.sev }
func (t helpersTestResult) GetTimestamp() time.Time    { return t.ts }
func (t helpersTestResult) GetDuration() time.Duration { return t.dur }
func (t helpersTestResult) GetDetails() string         { return t.det }
