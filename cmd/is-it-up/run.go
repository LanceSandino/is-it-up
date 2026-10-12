// This file executes one configured health-check run and formats its result.
// It depends on the checks, logging, and secret-fetching packages through injectable seams.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LanceSandino/is-it-up/pkg/checks"
	"github.com/LanceSandino/is-it-up/pkg/logger"
	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// RunOptions allows injecting dependencies for testing.
type RunOptions struct {
	Out             io.Writer
	Pretty          bool
	Debug           bool
	DumpConfig      bool
	FailOnUnhealthy bool
	SecretsFetcher  secrets.SecretFetcher // optional override
	RunAllOverride  func(ctx context.Context, cfg *checks.Config) []checks.Result
}

// Run executes the main CLI logic but is testable because it accepts options
// and injectable dependencies. It returns an exit code style integer (0 ok,
// non-zero for failures) so tests can assert behavior without os.Exit.
func Run(ctx context.Context, cfgPath string, opts RunOptions) int {
	finalJSON := !opts.Pretty
	outLogger := logger.New(opts.Out, finalJSON, opts.Debug, Version)

	ctx = logger.WithLogger(ctx, outLogger)

	if cfgPath == "" {
		outLogger.Errorf("no config provided; specify -config")
		return 2
	}

	cfg, _, err := checks.LoadConfigAutoNoValidation(cfgPath)
	if err != nil {
		outLogger.Errorf("failed to load config %s: %v", cfgPath, err)
		return 2
	}
	outLogger.Debugf("loaded config: %s", cfgPath)

	var fetcher secrets.SecretFetcher
	if opts.SecretsFetcher != nil {
		fetcher = opts.SecretsFetcher
	} else if f, ferr := secrets.NewAWSSecretsFetcher(ctx); ferr == nil {
		fetcher = f
	} else {
		outLogger.Infof("secrets manager unavailable; will not resolve sm:// refs")
	}

	if fetcher != nil {
		secretsResolved, err := checks.ResolveConfigSecrets(ctx, cfg, fetcher)
		if err != nil {
			outLogger.Errorf("failed to resolve secrets in config %s: %v", cfgPath, err)
			return 2
		}
		if secretsResolved > 0 {
			outLogger.Infof("secrets resolved in config: %s (%d secrets)", cfgPath, secretsResolved)
		} else {
			outLogger.Debugf("no secrets to resolve in config: %s", cfgPath)
		}

		// Validate resolved secret values
		if err := checks.ValidateResolvedSecrets(cfg); err != nil {
			outLogger.Errorf("configuration validation failed after resolving secrets: %v", err)
			return 2
		}
	} else {
		for _, c := range cfg.Checks {
			if c.Address.IsRef() || c.Host.IsRef() || c.Port.IsRef() || c.User.IsRef() || c.Password.IsRef() || c.SMTPEncryption.IsRef() {
				outLogger.Errorf("config contains sm:// secret references but secrets manager unavailable; set AWS creds or remove refs")
				return 2
			}
		}
	}

	// Validate configuration (do this after secret resolution)
	if err := checks.ValidateConfig(cfg); err != nil {
		outLogger.Errorf("configuration validation failed: %v", err)
		return 2
	}

	ctx = logger.WithLogger(ctx, outLogger)

	if opts.DumpConfig {
		rc := checks.RedactedConfig(cfg, false)
		b, err := json.MarshalIndent(rc, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			return 1
		}

		var writeErr error
		if opts.Out != nil {
			if _, err := opts.Out.Write(b); err != nil {
				writeErr = err
			}
			if _, err := opts.Out.Write([]byte("\n")); err != nil && writeErr == nil {
				writeErr = err
			}
		} else {
			if _, err := os.Stdout.Write(b); err != nil {
				writeErr = err
			}
			if _, err := os.Stdout.Write([]byte("\n")); err != nil && writeErr == nil {
				writeErr = err
			}
		}

		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", writeErr)
			return 1
		}
		return 0
	}

	var results []checks.Result
	if opts.RunAllOverride != nil {
		results = opts.RunAllOverride(ctx, cfg)
	} else {
		results = checks.RunAll(ctx, cfg)
	}

	for _, r := range results {
		outLogger.Log(r)
	}

	total := len(results)
	failed := 0
	warnCount := 0
	for _, r := range results {
		if !r.OK {
			failed++
			if strings.ToLower(r.Severity) == "warning" {
				warnCount++
			}
		}
	}
	if failed == 0 {
		outLogger.Debugf("summary: %d checks passed", total)
	} else {
		if failed == warnCount {
			outLogger.Warnf("summary: %d checks run, %d warnings", total, warnCount)
		} else {
			outLogger.Errorf("summary: %d checks run, %d failures (%d warnings)", total, failed, warnCount)
		}
	}

	if opts.FailOnUnhealthy && failed > 0 {
		return 1
	}

	// For monitoring/observability use cases, return success by default
	// when health checks complete successfully, regardless of individual check results.
	// The actual health status is captured in the structured logs for parsing by
	// monitoring systems like New Relic, Splunk, etc.
	// Only return non-zero exit codes for application/configuration errors.
	return 0
}
