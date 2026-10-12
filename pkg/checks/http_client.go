// This file constructs the shared bounded HTTP client used by network checks.
// It depends on standard TLS and transport settings without retaining credentials.
package checks

import (
	"crypto/tls"
	"net/http"
	"time"
)

// HTTPClientConfig holds configuration for HTTP client creation.
type HTTPClientConfig struct {
	Timeout         time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int
	TLSMode         string // "skip-verify" or "required" (default)
}

// DefaultHTTPClientConfig returns sensible defaults for HTTP client configuration.
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:         30 * time.Second,
		MaxIdleConns:    10,
		MaxConnsPerHost: 2,
		TLSMode:         "required", // Default to secure TLS validation
	}
}

// NewHTTPClient creates a new HTTP client with the specified configuration.
func NewHTTPClient(config HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxConnsPerHost,
	}

	// Configure TLS based on mode
	if config.TLSMode == "skip-verify" {
		// The caller explicitly requested insecure verification for this client.
		// This is intentional for diagnostic or legacy environments. Keep the
		// option narrow and documented. We still set a reasonable minimum TLS
		// version to avoid extremely old protocol versions.
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // #nosec G402 - explicit user opt-in
			MinVersion:         tls.VersionTLS12,
		}
	} else {
		// Default to secure TLS validation and require at least TLS1.2
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}
}
