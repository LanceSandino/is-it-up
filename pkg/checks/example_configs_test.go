// This file validates the shipped example configurations against the check
// types the CLI activates, including compile-time plugins.
package checks_test

import (
	"testing"

	_ "github.com/LanceSandino/is-it-up/v2/pkg/checkplugins/dns"
	"github.com/LanceSandino/is-it-up/v2/pkg/checks"
)

func TestExampleConfigurationsValidate(t *testing.T) {
	for _, path := range []string{
		"../../example/config.example.yaml",
		"../../example/aws-secret.example.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			cfg, format, err := checks.LoadConfigAutoNoValidation(path)
			if err != nil {
				t.Fatalf("load example: %v", err)
			}
			if format != "yaml" {
				t.Fatalf("format = %q, want yaml", format)
			}
			if err := checks.ValidateConfig(cfg); err != nil {
				t.Fatalf("validate example: %v", err)
			}
		})
	}
}
