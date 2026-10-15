// This file implements configurable HTTP and HTTPS endpoint health checks.
// It depends on the shared HTTP client and response validation helpers.
package checks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// checkHTTP performs a simple GET against the provided address and path.
// It treats 2xx and 3xx responses as success and returns a human-readable
// message containing the HTTP status code.

func checkHTTP(ctx context.Context, cfg CheckConfig) (bool, string) {
	config := DefaultHTTPClientConfig()
	config.Timeout = timeoutFromContext(ctx, 4*time.Second)

	// Configure TLS based on URL scheme and check configuration
	address := cfg.Address.String()
	tlsMode := strings.TrimSpace(cfg.TLSMode)
	if tlsMode == "required" && !strings.HasPrefix(strings.ToLower(address), "https://") {
		return false, "tls_mode required requires https:// address"
	}

	// For HTTPS URLs, apply TLS configuration
	if tlsMode == "" {
		// Default to secure TLS validation for HTTPS
		config.TLSMode = "required"
	} else {
		config.TLSMode = tlsMode
	}

	client := NewHTTPClient(config)
	url := address
	if cfg.Path.String() != "" && !strings.HasSuffix(address, "/") {
		if strings.HasPrefix(cfg.Path.String(), "/") {
			url = address + cfg.Path.String()
		} else {
			url = address + "/" + cfg.Path.String()
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, NewCheckError("http", redactAddress(url), "create request", errors.New(redactText(err.Error()))).Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		// Return structured error information for better debugging
		checkErr := NewCheckError("http", redactAddress(url), "request", errors.New(redactText(err.Error())))
		return false, checkErr.Error()
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically not critical for read operations
			// but could indicate connection issues - we ignore them here
			_ = err
		}
	}()
	// Accept 2xx and 3xx as success. Treat 4xx and 5xx as failure.
	if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
		return true, fmt.Sprintf("status %d", resp.StatusCode)
	}
	return false, fmt.Sprintf("status %d", resp.StatusCode)
}
