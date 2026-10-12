// This file wires process startup, option parsing, and the testable application runner.
// It depends on the CLI helpers in this package and owns process exit behavior.
package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	// Parse enhanced command-line arguments
	opts, err := ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle help and version
	if opts.ShowHelp {
		opts.ShowUsage()
		os.Exit(0)
	}

	if opts.ShowVersion {
		fmt.Printf("is-it-up %s\n", Version)
		os.Exit(0)
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Map enhanced options to RunOptions
	runOpts := RunOptions{
		Out:             os.Stdout,
		Pretty:          opts.LogFormat == "console",
		Debug:           opts.LogLevel == "debug",
		DumpConfig:      opts.DumpConfig,
		FailOnUnhealthy: opts.FailOnUnhealthy,
	}

	code := Run(context.Background(), opts.ConfigPath, runOpts)
	if code != 0 {
		os.Exit(code)
	}
}
