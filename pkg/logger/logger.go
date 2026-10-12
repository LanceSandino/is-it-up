// This file provides structured application logging and context propagation.
// It depends on zap and exposes output behavior shared by the CLI and checks.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// getServiceVersion returns the service version from environment or uses provided fallback
func getServiceVersion(fallback string) string {
	// Prefer environment variable for deployments
	if v := os.Getenv("SERVICE_VERSION"); v != "" {
		return v
	}
	// Fallback to git commit if available (common in CI/CD)
	if v := os.Getenv("GIT_SHA"); v != "" {
		if len(v) >= 8 {
			return v[:8] // Short commit hash
		}
		return v
	}
	// Use provided fallback version (from application)
	return fallback
}

// Logger wraps a zap SugaredLogger and provides small helpers used across
// the project. It supports JSON or console output and a debug mode which
// enables more verbose logging and stacktraces where appropriate.
type Logger struct {
	sugar         *zap.SugaredLogger
	json          bool
	mu            sync.Mutex
	debug         bool
	serviceFields map[string]interface{} // Service metadata for observability
}

// Loggable is a small interface representing a check result that can be
// logged by the logger package without creating import cycles. Packages
// that produce structured results (e.g., checks.Result) should implement
// this interface.
type Loggable interface {
	GetName() string
	IsOK() bool
	GetMessage() string
	GetSeverity() string
	GetTimestamp() time.Time
	GetDuration() time.Duration
	GetDetails() string
}

// New creates a new logger with the specified service version.
func New(w io.Writer, jsonOut bool, debug bool, version string) *Logger {
	var cfg zap.Config
	if jsonOut {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.Encoding = "console"
	}
	// Avoid emitting stack traces for non-fatal errors in production use.
	// NewDevelopmentConfig sets Development=true (which causes stacktraces on Error level).
	// We explicitly turn Development off so Error logs won't automatically include stack traces.
	cfg.Development = false
	if debug {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	// Hide caller metadata when not running in debug mode to keep JSON concise.
	// When debug is enabled, include caller info to aid troubleshooting.
	if !debug {
		cfg.DisableCaller = true
	}
	// Disable automatic stacktraces unless in debug mode; stacktraces
	// at error level are noisy for normal operation.
	if !debug {
		cfg.DisableStacktrace = true
	}
	// Configure encoder settings based on output format
	if jsonOut {
		// JSON format: Observability best practices with standard field names
		cfg.EncoderConfig.TimeKey = "timestamp"                   // Standard field name for NewRelic/Splunk
		cfg.EncoderConfig.LevelKey = "level"                      // Standard log level field
		cfg.EncoderConfig.MessageKey = "message"                  // Standard message field
		cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder // ISO8601 with timezone
	} else {
		// Console format: Clean, human-readable output
		cfg.EncoderConfig.TimeKey = "T"
		cfg.EncoderConfig.LevelKey = "L"
		cfg.EncoderConfig.MessageKey = "M"
		cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder   // Simple time format
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // INFO, DEBUG, etc.
		cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // Short caller format
	}
	// If a custom writer was supplied (for tests), build a core that
	// writes directly to that writer so the caller can capture output.
	if w != nil && w != os.Stdout {
		var enc zapcore.Encoder
		if jsonOut {
			enc = zapcore.NewJSONEncoder(cfg.EncoderConfig)
		} else {
			enc = zapcore.NewConsoleEncoder(cfg.EncoderConfig)
		}
		ws := zapcore.AddSync(w)
		core := zapcore.NewCore(enc, ws, cfg.Level)
		logger := zap.New(core)
		sugar := logger.Sugar()

		// Initialize service metadata for observability platforms
		serviceFields := map[string]interface{}{
			"service.name":    "is-it-up",
			"service.version": getServiceVersion(version),
		}

		return &Logger{sugar: sugar, json: jsonOut, debug: debug, serviceFields: serviceFields}
	}

	logger, _ := cfg.Build()
	sugar := logger.Sugar()

	// Initialize service metadata for observability platforms
	serviceFields := map[string]interface{}{
		"service.name":    "is-it-up",
		"service.version": getServiceVersion(version),
	}

	return &Logger{sugar: sugar, json: jsonOut, debug: debug, serviceFields: serviceFields}
}

// CtxKey is the context key type used to store a *Logger in contexts.
type CtxKey struct{}

// WithLogger returns a new context that carries the provided logger.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, CtxKey{}, l)
}

// FromContext retrieves a *Logger stored in the context, or nil.
func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(CtxKey{}); v != nil {
		if lg, ok := v.(*Logger); ok {
			return lg
		}
	}
	return nil
}

// IsDebug returns whether this logger was created in debug mode.
func (l *Logger) IsDebug() bool { return l != nil && l.debug }

// Infow logs a message with alternating key/value pairs.
func (l *Logger) Infow(msg string, keysAndValues ...interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// Add service metadata for observability platforms (JSON format only)
	if l.json && l.serviceFields != nil {
		enhanced := make([]interface{}, 0, len(keysAndValues)+len(l.serviceFields)*2)
		for k, v := range l.serviceFields {
			enhanced = append(enhanced, k, v)
		}
		enhanced = append(enhanced, keysAndValues...)
		l.sugar.Infow(msg, enhanced...)
	} else {
		l.sugar.Infow(msg, keysAndValues...)
	}
}

// Debugw logs a debug-level message with alternating key/value pairs.
func (l *Logger) Debugw(msg string, keysAndValues ...interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// Add service metadata for observability platforms (JSON format only)
	if l.json && l.serviceFields != nil {
		enhanced := make([]interface{}, 0, len(keysAndValues)+len(l.serviceFields)*2)
		for k, v := range l.serviceFields {
			enhanced = append(enhanced, k, v)
		}
		enhanced = append(enhanced, keysAndValues...)
		l.sugar.Debugw(msg, enhanced...)
	} else {
		l.sugar.Debugw(msg, keysAndValues...)
	}
}

// SetSplunkHEC intentionally removed

// To avoid an import cycle between logger and checks, logger accepts a
// generic value in Log and uses a small Loggable interface or reflection to
// read result fields. Use Infow/Debugw for structured logging where possible.

// Log logs a checks-like result object. We accept interface{} here and
// use reflection to read expected exported fields so the logger package
// does not need to import the checks package (avoiding an import cycle).
func (l *Logger) Log(r interface{}) {
	if l == nil || r == nil {
		return
	}

	// Prefer typed Loggable for performance and clarity.
	if lg, ok := r.(Loggable); ok {
		name := lg.GetName()
		okv := lg.IsOK()
		resultMsg := lg.GetMessage()
		severity := lg.GetSeverity()
		ts := lg.GetTimestamp()
		dur := lg.GetDuration()
		details := lg.GetDetails()

		// Construct a meaningful human-readable message for the log entry
		status := "pass"
		if !okv {
			status = "fail"
		}
		// Message format: "Health check <status> for <name> (<duration>ms)"
		// Full error details are in check_result field
		logMessage := fmt.Sprintf("Health check %s for %s (%dms)", status, name, dur.Milliseconds())

		l.mu.Lock()
		defer l.mu.Unlock()

		if l.json {
			// Enhanced structured logging for observability platforms (New Relic best practices)
			fields := []interface{}{
				// Event categorization
				"event_type", "health_check",
				"event_outcome", func() string {
					if okv {
						return "success"
					} else {
						return "failure"
					}
				}(),

				// Health check specific fields (flat structure for easy querying)
				"check_name", name,
				"check_status", status,
				"check_result", resultMsg,
				"check_severity", severity,
				"check_timestamp", ts.Format(time.RFC3339),
				"check_duration_ms", dur.Milliseconds(),
			}

			// Add service metadata
			if l.serviceFields != nil {
				for k, v := range l.serviceFields {
					fields = append(fields, k, v)
				}
			}

			if l.debug && details != "" {
				fields = append(fields, "check_details", details)
			}

			if okv {
				l.sugar.Infow(logMessage, fields...)
			} else {
				l.sugar.Warnw(logMessage, fields...)
			}
			return
		}

		// Pretty/console output: human-readable format
		statusStr := "OK"
		if !okv {
			statusStr = "FAIL"
		}
		if okv {
			l.sugar.Infof("[%s] %s - %s (%s) %dms", statusStr, name, resultMsg, severity, dur.Milliseconds())
		} else {
			l.sugar.Warnf("[%s] %s - %s (%s) %dms", statusStr, name, resultMsg, severity, dur.Milliseconds())
		}
		if l.debug && details != "" {
			l.sugar.Debugf("details: %s", details)
		}
		return
	}

	// Fallback: accept arbitrary struct via reflection for backwards compatibility
	rv := reflect.ValueOf(r)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	getString := func(name string) string {
		if f := rv.FieldByName(name); f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
		return ""
	}
	getBool := func(name string) bool {
		if f := rv.FieldByName(name); f.IsValid() && f.Kind() == reflect.Bool {
			return f.Bool()
		}
		return false
	}
	getTime := func(name string) time.Time {
		if f := rv.FieldByName(name); f.IsValid() {
			if t, ok := f.Interface().(time.Time); ok {
				return t
			}
		}
		return time.Time{}
	}
	getDuration := func(name string) time.Duration {
		if f := rv.FieldByName(name); f.IsValid() {
			if d, ok := f.Interface().(time.Duration); ok {
				return d
			}
		}
		return 0
	}

	name := getString("Name")
	okv := getBool("OK")
	resultMsg := getString("Message")
	severity := getString("Severity")
	ts := getTime("Timestamp")
	dur := getDuration("Duration")
	details := getString("Details")

	// Construct a meaningful human-readable message for the log entry
	status := "pass"
	if !okv {
		status = "fail"
	}
	// Message format: "Health check <status> for <name> (<duration>ms)"
	// Full error details are in check_result field
	logMessage := fmt.Sprintf("Health check %s for %s (%dms)", status, name, dur.Milliseconds())

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.json {
		// Enhanced structured logging for observability platforms (New Relic best practices)
		fields := []interface{}{
			// Event categorization
			"event_type", "health_check",
			"event_outcome", func() string {
				if okv {
					return "success"
				} else {
					return "failure"
				}
			}(),

			// Health check specific fields (flat structure for easy querying)
			"check_name", name,
			"check_status", status,
			"check_result", resultMsg,
			"check_severity", severity,
			"check_timestamp", ts.Format(time.RFC3339),
			"check_duration_ms", dur.Milliseconds(),
		}

		// Add service metadata
		if l.serviceFields != nil {
			for k, v := range l.serviceFields {
				fields = append(fields, k, v)
			}
		}

		if l.debug && details != "" {
			fields = append(fields, "check_details", details)
		}

		if okv {
			l.sugar.Infow(logMessage, fields...)
		} else {
			l.sugar.Warnw(logMessage, fields...)
		}
		return
	}

	// Pretty/console output: human-readable format
	statusStr := "OK"
	if !okv {
		statusStr = "FAIL"
	}
	if okv {
		l.sugar.Infof("%s [%s] %s - %s (%s) %dms", ts.Format(time.RFC3339), statusStr, name, resultMsg, severity, dur.Milliseconds())
	} else {
		l.sugar.Warnf("%s [%s] %s - %s (%s) %dms", ts.Format(time.RFC3339), statusStr, name, resultMsg, severity, dur.Milliseconds())
	}
	if l.debug && details != "" {
		l.sugar.Debugf("details: %s", details)
	}
}

// Infof logs a formatted info-level message.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// For JSON format, add service metadata using structured logging
	if l.json && l.serviceFields != nil {
		fields := make([]interface{}, 0, len(l.serviceFields)*2)
		for k, v := range l.serviceFields {
			fields = append(fields, k, v)
		}
		l.sugar.Infow(fmt.Sprintf(format, args...), fields...)
	} else {
		l.sugar.Infof(format, args...)
	}
}

// Debugf logs a formatted debug-level message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// For JSON format, add service metadata using structured logging
	if l.json && l.serviceFields != nil {
		fields := make([]interface{}, 0, len(l.serviceFields)*2)
		for k, v := range l.serviceFields {
			fields = append(fields, k, v)
		}
		l.sugar.Debugw(fmt.Sprintf(format, args...), fields...)
	} else {
		l.sugar.Debugf(format, args...)
	}
}

// Warnf logs a formatted warn-level message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sugar.Warnf(format, args...)
}

// Errorf logs a formatted error-level message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sugar.Errorf(format, args...)
}
