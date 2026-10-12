// This file resolves configured secret references into an in-memory check configuration.
// It depends on the secrets interface and preserves a redacted representation for output.
package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// ResolveConfigSecrets resolves any sm:// secret references found in the
// provided config's string fields using the supplied SecretFetcher. It updates
// the config in-place. Only scalar sm:// URIs are supported (not mapping forms).
// Returns the number of secrets that were resolved.
func ResolveConfigSecrets(ctx context.Context, cfg *Config, fetcher secrets.SecretFetcher) (int, error) {
	if cfg == nil || fetcher == nil {
		return 0, nil
	}
	secretsResolved := 0

	for i := range cfg.Checks {
		c := &cfg.Checks[i]
		// helper to resolve a SecretValue in-place: if it's a ref, fetch and
		// replace Raw with the fetched payload and clear Ref. Log events
		// about resolution attempts (do NOT include secret values).
		resolveField := func(name string, sv *secrets.SecretValue) error {
			if sv == nil {
				return nil
			}
			if sv.IsRef() {
				v, err := sv.Resolve(ctx, fetcher)
				if err != nil {
					return &CheckFieldError{Check: c.Name, Field: name, Err: err}
				}
				// replace with resolved literal
				sv.Raw = v
				sv.Ref = nil
				secretsResolved++
			}
			return nil
		}

		if err := resolveField("address", &c.Address); err != nil {
			return 0, err
		}
		if err := resolveField("protocol", &c.Protocol); err != nil {
			return 0, err
		}
		if err := resolveField("host", &c.Host); err != nil {
			return 0, err
		}
		if err := resolveField("port", &c.Port); err != nil {
			return 0, err
		}
		if err := resolveField("user", &c.User); err != nil {
			return 0, err
		}
		if err := resolveField("password", &c.Password); err != nil {
			return 0, err
		}
		if err := resolveField("path", &c.Path); err != nil {
			return 0, err
		}
		if err := resolveField("database", &c.Database); err != nil {
			return 0, err
		}
		if err := resolveField("vhost", &c.Vhost); err != nil {
			return 0, err
		}
		if err := resolveField("smtp_encryption", &c.SMTPEncryption); err != nil {
			return 0, err
		}
		// If address is still empty but host and port are present, compose address
		if c.Address.IsEmpty() && c.Host.HasValue() && c.Port.HasValue() {
			switch strings.ToLower(c.Type) {
			case "http", "elasticsearch", "opensearch":
				proto := c.Protocol.String()
				if proto == "" {
					proto = "http"
				}
				c.Address.Raw = fmt.Sprintf("%s://%s:%s", proto, c.Host.String(), c.Port.String())
				c.Address.Ref = nil
			default:
				c.Address.Raw = fmt.Sprintf("%s:%s", c.Host.String(), c.Port.String())
				c.Address.Ref = nil
			}
			// Clear host and port fields to avoid validation conflicts
			c.Host = secrets.SecretValue{}
			c.Port = secrets.SecretValue{}
		}
	}
	return secretsResolved, nil
}

// LoadConfigAndResolve loads configuration from path and resolves secrets using the provided fetcher.
func LoadConfigAndResolve(ctx context.Context, path string, fetcher secrets.SecretFetcher) (*Config, string, error) {
	cfg, format, err := LoadConfig(path)
	if err != nil {
		return nil, format, err
	}
	if _, err := ResolveConfigSecrets(ctx, cfg, fetcher); err != nil {
		return nil, format, err
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, format, err
	}
	if err := ValidateResolvedSecrets(cfg); err != nil {
		return nil, format, err
	}
	return cfg, format, nil
}
