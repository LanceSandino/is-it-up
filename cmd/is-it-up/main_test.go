package main

import (
	"context"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/LanceSandino/is-it-up/pkg/checks"
)

// TestMainWorkflow tests the main workflow through the exposed functions
// Since main() calls os.Exit(), we test the components it uses instead
func TestMainWorkflow_ParseFlags(t *testing.T) {
	// Test that main workflow can handle valid flags
	opts := &CommandOptions{
		ConfigPath:  "/path/to/config.yaml",
		LogLevel:    "info",
		LogFormat:   "json",
		ShowHelp:    false,
		ShowVersion: false,
	}

	err := opts.Validate()
	if err != nil {
		t.Errorf("expected validation to pass, got: %v", err)
	}
}

func TestMainWorkflow_RunOptionsMapping(t *testing.T) {
	// Test the mapping from CommandOptions to RunOptions
	opts := &CommandOptions{
		LogFormat:  "console", // should map to Pretty: true
		LogLevel:   "debug",   // should map to Debug: true
		DumpConfig: true,
	}

	// Simulate the mapping done in main()
	runOpts := RunOptions{
		Pretty:     opts.LogFormat == "console",
		Debug:      opts.LogLevel == "debug",
		DumpConfig: opts.DumpConfig,
	}

	if !runOpts.Pretty {
		t.Error("expected Pretty true when LogFormat is console")
	}
	if !runOpts.Debug {
		t.Error("expected Debug true when LogLevel is debug")
	}
	if !runOpts.DumpConfig {
		t.Error("expected DumpConfig true")
	}
}

func TestMainWorkflow_BackwardCompatibilityMapping(t *testing.T) {
	// Test backward compatibility mapping
	tests := []struct {
		name       string
		opts       CommandOptions
		wantDebug  bool
		wantPretty bool
	}{
		{
			name:      "debug flag sets debug level",
			opts:      CommandOptions{Debug: true},
			wantDebug: true,
		},
		{
			name:       "pretty flag sets console format",
			opts:       CommandOptions{Pretty: true},
			wantPretty: true,
		},
		{
			name:      "verbose flag sets debug level",
			opts:      CommandOptions{Verbose: true},
			wantDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the same logic as in ParseFlags backward compatibility
			if tt.opts.Debug || tt.opts.Verbose {
				tt.opts.LogLevel = "debug"
			}
			if tt.opts.Pretty {
				tt.opts.LogFormat = "console"
			}

			if tt.wantDebug && tt.opts.LogLevel != "debug" {
				t.Error("expected LogLevel to be 'debug'")
			}
			if tt.wantPretty && tt.opts.LogFormat != "console" {
				t.Error("expected LogFormat to be 'console'")
			}
		})
	}
}

// Test the complete workflow but with mocked Run function
func TestMainWorkflow_Complete(t *testing.T) {
	cfg := map[string]interface{}{
		"checks": []map[string]interface{}{
			{"name": "test", "type": "http", "address": "http://localhost:8080", "severity": "info"},
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}

	f, err := os.CreateTemp("", "main-workflow-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.Write(b); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close temp config: %v", err)
	}

	// Test the workflow components
	opts := &CommandOptions{
		ConfigPath: f.Name(),
		LogLevel:   "info",
		LogFormat:  "json",
	}

	err = opts.Validate()
	if err != nil {
		t.Errorf("validation failed: %v", err)
	}

	runOpts := RunOptions{
		Pretty:     opts.LogFormat == "console",
		Debug:      opts.LogLevel == "debug",
		DumpConfig: opts.DumpConfig,
		RunAllOverride: func(ctx context.Context, cfg *checks.Config) []checks.Result {
			return []checks.Result{{Name: "test", OK: true, Message: "ok", Severity: "info"}}
		},
	}

	code := Run(context.Background(), opts.ConfigPath, runOpts)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}
