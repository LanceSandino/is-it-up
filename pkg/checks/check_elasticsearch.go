// This file implements Elasticsearch-compatible HTTP health checks.
// It depends on the shared HTTP client and sanitized endpoint diagnostics.
package checks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// checkElasticsearch performs a health check against an Elasticsearch/OpenSearch cluster.
// It makes an HTTP GET request to the cluster health endpoint and validates the response.
func checkElasticsearch(ctx context.Context, address, path string) (bool, string) {
	config := DefaultHTTPClientConfig()
	config.Timeout = timeoutFromContext(ctx, 4*time.Second)
	client := NewHTTPClient(config)

	// Default path for cluster health
	if path == "" {
		path = "_cluster/health"
	}

	var url string
	if !strings.HasSuffix(address, "/") && !strings.HasPrefix(path, "/") {
		url = address + "/" + path
	} else if strings.HasPrefix(path, "/") {
		url = address + path
	} else {
		url = address + path
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		checkErr := NewCheckError("elasticsearch", redactAddress(address), "create request", errors.New(redactText(err.Error())))
		return false, checkErr.Error()
	}

	resp, err := client.Do(req)
	if err != nil {
		checkErr := NewCheckError("elasticsearch", redactAddress(address), "request", errors.New(redactText(err.Error())))
		return false, checkErr.Error()
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically not critical for read operations
			// but could indicate connection issues - we ignore them here
			_ = err
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		checkErr := NewCheckError("elasticsearch", redactAddress(address), "read response", err)
		return false, checkErr.Error()
	}

	// Check HTTP status
	if resp.StatusCode != 200 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	// Parse JSON response to check cluster status
	var healthResp struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(body, &healthResp); err != nil {
		// If JSON parsing fails, but we got HTTP 200, consider it a success
		return true, "cluster ok"
	}

	// Check cluster status - green and yellow are considered healthy
	switch strings.ToLower(healthResp.Status) {
	case "green", "yellow":
		return true, fmt.Sprintf("cluster %s", healthResp.Status)
	case "red":
		return false, fmt.Sprintf("cluster %s", healthResp.Status)
	default:
		// Unknown status, but got valid JSON response
		return true, "cluster ok"
	}
}
