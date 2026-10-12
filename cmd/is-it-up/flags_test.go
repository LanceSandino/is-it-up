package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParseFlags_DefaultValues(t *testing.T) {
	// Reset flag package for clean test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Simulate no command line arguments
	os.Args = []string{"program"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	// Check default values
	if opts.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", opts.LogLevel)
	}
	if opts.LogFormat != "json" {
		t.Errorf("expected LogFormat 'json', got '%s'", opts.LogFormat)
	}
	if opts.ShowVersion {
		t.Error("expected ShowVersion false by default")
	}
	if opts.ShowHelp {
		t.Error("expected ShowHelp false by default")
	}
}

func TestParseFlags_BackwardCompatibility(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"program", "--debug", "--pretty"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	// Check backward compatibility mapping
	if opts.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug' from --debug flag, got '%s'", opts.LogLevel)
	}
	if opts.LogFormat != "console" {
		t.Errorf("expected LogFormat 'console' from --pretty flag, got '%s'", opts.LogFormat)
	}
}

func TestParseFlags_NewFlags(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"program", "--log-level", "warn", "--log-format", "console", "--verbose", "--fail-on-unhealthy"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	// The verbose flag should be set to true
	if !opts.Verbose {
		t.Error("expected Verbose true with --verbose flag")
	}
	if opts.LogLevel != "debug" {
		t.Errorf("expected --verbose to enable debug logging, got %q", opts.LogLevel)
	}
	if opts.LogFormat != "console" {
		t.Errorf("expected LogFormat 'console', got '%s'", opts.LogFormat)
	}
	if !opts.FailOnUnhealthy {
		t.Error("expected FailOnUnhealthy true")
	}
}

func TestParseFlags_ConfigPath(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"program", "-c", "/path/to/config.yaml"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if opts.ConfigPath != "/path/to/config.yaml" {
		t.Errorf("expected ConfigPath '/path/to/config.yaml', got '%s'", opts.ConfigPath)
	}
}

func TestParseFlags_Version(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"program", "--version"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if !opts.ShowVersion {
		t.Error("expected ShowVersion true with --version flag")
	}
}

func TestParseFlags_Help(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"program", "-h"}

	opts, err := ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if !opts.ShowHelp {
		t.Error("expected ShowHelp true with -h flag")
	}
}

func TestCommandOptions_Validate_Success(t *testing.T) {
	opts := &CommandOptions{
		ConfigPath: "/path/to/config.yaml",
		LogLevel:   "info",
		LogFormat:  "json",
	}

	err := opts.Validate()
	if err != nil {
		t.Errorf("expected validation to pass, got error: %v", err)
	}
}

func TestCommandOptions_Validate_MissingConfig(t *testing.T) {
	opts := &CommandOptions{
		LogLevel:  "info",
		LogFormat: "json",
	}

	err := opts.Validate()
	if err == nil {
		t.Error("expected validation to fail for missing config path")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("expected config-related error, got: %v", err)
	}
}

func TestCommandOptions_Validate_InvalidLogLevel(t *testing.T) {
	opts := &CommandOptions{
		ConfigPath: "/path/to/config.yaml",
		LogLevel:   "invalid",
		LogFormat:  "json",
	}

	err := opts.Validate()
	if err == nil {
		t.Error("expected validation to fail for invalid log level")
	}
	if !strings.Contains(err.Error(), "log level") {
		t.Errorf("expected log level error, got: %v", err)
	}
}

func TestCommandOptions_Validate_InvalidLogFormat(t *testing.T) {
	opts := &CommandOptions{
		ConfigPath: "/path/to/config.yaml",
		LogLevel:   "info",
		LogFormat:  "invalid",
	}

	err := opts.Validate()
	if err == nil {
		t.Error("expected validation to fail for invalid log format")
	}
	if !strings.Contains(err.Error(), "log format") {
		t.Errorf("expected log format error, got: %v", err)
	}
}

func TestShowUsage(t *testing.T) {
	opts := &CommandOptions{}

	// ShowUsage should not panic and should be callable
	// We can't easily test the output without complex mocking
	// but we can ensure it doesn't crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ShowUsage panicked: %v", r)
		}
	}()

	opts.ShowUsage()
}
