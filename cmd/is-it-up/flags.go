// This file defines the command-line interface and validates its option values.
// It depends on Go's flag package and returns settings consumed by the CLI runner.
package main

import (
	"flag"
	"fmt"
	"os"
)

// Build metadata is replaced with -ldflags for release artifacts.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// CommandOptions holds all command-line configuration.
type CommandOptions struct {
	ConfigPath  string
	LogLevel    string
	LogFormat   string
	ShowVersion bool
	ShowHelp    bool
	Verbose     bool
	// Backward compatibility
	Debug           bool
	Pretty          bool
	DumpConfig      bool
	FailOnUnhealthy bool
}

// ParseFlags parses command-line arguments and returns configuration.
func ParseFlags() (*CommandOptions, error) {
	opts := &CommandOptions{}

	// File paths
	flag.StringVar(&opts.ConfigPath, "config", "", "Path to configuration file (required)")
	flag.StringVar(&opts.ConfigPath, "c", "", "Path to configuration file (short)")

	// Logging (new granular + backward compatibility)
	flag.StringVar(&opts.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&opts.LogLevel, "l", "info", "Log level (short)")
	flag.StringVar(&opts.LogFormat, "log-format", "json", "Log format (json, console)")
	flag.BoolVar(&opts.Debug, "debug", false, "Enable debug output (equivalent to --log-level debug)")
	flag.BoolVar(&opts.Pretty, "pretty", false, "Enable console output (equivalent to --log-format console)")
	flag.BoolVar(&opts.Verbose, "verbose", false, "Enable verbose output")

	// Additional options
	flag.BoolVar(&opts.DumpConfig, "dump-resolved-config", false, "Print the resolved config (redacted) and exit")
	flag.BoolVar(&opts.FailOnUnhealthy, "fail-on-unhealthy", false, "Exit 1 when any health check is unhealthy")
	flag.BoolVar(&opts.ShowVersion, "version", false, "Show version information")
	flag.BoolVar(&opts.ShowVersion, "v", false, "Show version information (short)")
	flag.BoolVar(&opts.ShowHelp, "help", false, "Show help information")
	flag.BoolVar(&opts.ShowHelp, "h", false, "Show help information (short)")

	flag.Parse()

	// Handle backward compatibility mapping
	if opts.Debug {
		opts.LogLevel = "debug"
	}
	if opts.Verbose {
		opts.LogLevel = "debug"
	}
	if opts.Pretty {
		opts.LogFormat = "console"
	}

	return opts, nil
}

// ShowUsage displays comprehensive help information.
func (opts *CommandOptions) ShowUsage() {
	fmt.Fprintf(os.Stderr, `is-it-up - Health Check Tool

USAGE:
    is-it-up [OPTIONS]

OPTIONS:
    Configuration:
        -c, --config PATH       Path to configuration file (required)
        --config PATH           Path to configuration file (backward compatibility)

    Logging:
        -l, --log-level LEVEL   Log level: debug, info, warn, error (default: info)
        --log-format FORMAT     Log format: json, console (default: json)
        --verbose               Enable debug logging (alias for --log-level debug)
        --debug                 Enable debug output (same as --log-level debug)
        --pretty                Enable console output (same as --log-format console)

    Debugging:
        --dump-resolved-config  Print the resolved configuration with secrets redacted

    Automation:
        --fail-on-unhealthy     Exit 1 when any health check is unhealthy

    Information:
        -v, --version          Show version information
        -h, --help             Show this help message

EXAMPLES:
    # Basic usage
    is-it-up -c config.yaml

    # Backward compatibility
    is-it-up -config config.yaml -debug -pretty

    # New enhanced logging
    is-it-up -c config.yaml --log-level debug --log-format console

    # With verbose output
    is-it-up -c config.yaml --verbose

    # Inspect a resolved configuration (secrets remain redacted)
    is-it-up -c config.yaml --dump-resolved-config --debug

CONFIGURATION:
    Configuration files support YAML format only. Secret references
    use the format sm://SECRET_ID#JSON.KEY for AWS Secrets Manager.

    See example/config.example.yaml for configuration examples.

`)
}

// Validate checks if the provided options are valid.
func (opts *CommandOptions) Validate() error {
	if opts.ConfigPath == "" && !opts.ShowVersion && !opts.ShowHelp {
		return fmt.Errorf("configuration file is required (use -c or --config)")
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[opts.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", opts.LogLevel)
	}

	validLogFormats := map[string]bool{
		"json": true, "console": true,
	}
	if !validLogFormats[opts.LogFormat] {
		return fmt.Errorf("invalid log format: %s (must be json or console)", opts.LogFormat)
	}

	return nil
}
