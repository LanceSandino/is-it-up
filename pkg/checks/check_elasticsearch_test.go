package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCheckElasticsearch_ClusterHealth tests successful Elasticsearch cluster health check.
// Verifies that the check passes when the server returns a 200 status with valid JSON response.
func TestCheckElasticsearch_ClusterHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"cluster_name":"test","status":"green"}`)); err != nil {
			t.Errorf("Failed to write test response: %v", err)
		}
	}))
	defer srv.Close()

	ok, msg := checkElasticsearch(context.Background(), srv.URL, "_cluster/health")
	if !ok {
		t.Fatalf("expected ok, got false msg=%q", msg)
	}
}

// TestCheckElasticsearch_BadStatus tests Elasticsearch check failure handling.
// Verifies that the check fails when the server returns a 500 HTTP status code.
func TestCheckElasticsearch_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	ok, _ := checkElasticsearch(context.Background(), srv.URL, "_cluster/health")
	if ok {
		t.Fatalf("expected failure on 500")
	}
}

func TestCheckElasticsearch_ResponseVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
		text string
	}{
		{name: "yellow", body: `{"status":"yellow"}`, want: true, text: "yellow"},
		{name: "red", body: `{"status":"red"}`, want: false, text: "red"},
		{name: "unknown", body: `{"status":"blue"}`, want: true, text: "cluster ok"},
		{name: "non-json", body: `healthy`, want: true, text: "cluster ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestedPath = r.URL.Path
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			ok, msg := checkElasticsearch(context.Background(), srv.URL+"/", "")
			if ok != tt.want || !strings.Contains(msg, tt.text) {
				t.Fatalf("got ok=%v msg=%q, want ok=%v containing %q", ok, msg, tt.want, tt.text)
			}
			if requestedPath != "/_cluster/health" {
				t.Fatalf("unexpected default path: %q", requestedPath)
			}
		})
	}
}

func TestCheckElasticsearch_RequestErrorsAreRedacted(t *testing.T) {
	ok, msg := checkElasticsearch(context.Background(), "://user:secret@example.test", "/health")
	if ok || strings.Contains(msg, "secret") {
		t.Fatalf("expected redacted request failure, got ok=%v msg=%q", ok, msg)
	}

	ok, msg = checkElasticsearch(context.Background(), "http://user:secret@127.0.0.1:1", "/health")
	if ok || strings.Contains(msg, "secret") {
		t.Fatalf("expected redacted connection failure, got ok=%v msg=%q", ok, msg)
	}
}
