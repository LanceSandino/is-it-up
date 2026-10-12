package checks

import (
	"net/http"
	"testing"
	"time"
)

// TestDefaultHTTPClientConfig tests that default configuration has sensible values.
func TestDefaultHTTPClientConfig(t *testing.T) {
	config := DefaultHTTPClientConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", config.Timeout)
	}
	if config.MaxIdleConns != 10 {
		t.Errorf("expected MaxIdleConns 10, got %d", config.MaxIdleConns)
	}
	if config.MaxConnsPerHost != 2 {
		t.Errorf("expected MaxConnsPerHost 2, got %d", config.MaxConnsPerHost)
	}
}

// TestNewHTTPClient tests that HTTP client is created with correct configuration.
func TestNewHTTPClient(t *testing.T) {
	config := HTTPClientConfig{
		Timeout:         5 * time.Second,
		MaxIdleConns:    20,
		MaxConnsPerHost: 5,
	}

	client := NewHTTPClient(config)

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.Timeout != 5*time.Second {
		t.Errorf("expected client timeout 5s, got %v", client.Timeout)
	}

	// Verify transport settings
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if transport.MaxIdleConns != 20 {
		t.Errorf("expected transport MaxIdleConns 20, got %d", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 5 {
		t.Errorf("expected transport MaxIdleConnsPerHost 5, got %d", transport.MaxIdleConnsPerHost)
	}
}

// TestHTTPClientConfig_CustomTimeout tests creating clients with custom timeouts.
func TestHTTPClientConfig_CustomTimeout(t *testing.T) {
	config := DefaultHTTPClientConfig()
	config.Timeout = 1 * time.Second

	client := NewHTTPClient(config)

	if client.Timeout != 1*time.Second {
		t.Errorf("expected custom timeout 1s, got %v", client.Timeout)
	}
}
