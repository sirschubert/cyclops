package hosts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestProbe() *HTTPProbe {
	return NewHTTPProbe(5, "TestAgent/1.0")
}

func TestProbeHost_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "TestServer/1.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><head><title>Hello</title></head><body></body></html>`))
	}))
	defer server.Close()

	probe := newTestProbe()
	host, err := probe.ProbeHost(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("ProbeHost() error: %v", err)
	}
	if host.StatusCode != 200 {
		t.Errorf("expected 200, got %d", host.StatusCode)
	}
	if host.Title != "Hello" {
		t.Errorf("expected title 'Hello', got %q", host.Title)
	}
	if host.Server != "TestServer/1.0" {
		t.Errorf("expected Server header, got %q", host.Server)
	}
}

func TestProbeHost_LimitsBodyPreview(t *testing.T) {
	bigBody := strings.Repeat("A", bodyPreviewStoreLimit*10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(bigBody))
	}))
	defer server.Close()

	probe := newTestProbe()
	host, err := probe.ProbeHost(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("ProbeHost() error: %v", err)
	}
	if len(host.BodyPreview) > bodyPreviewStoreLimit {
		t.Errorf("BodyPreview exceeds limit: got %d bytes", len(host.BodyPreview))
	}
}

func TestProbeHost_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newTestProbe().ProbeHost(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestProbeHosts_ConcurrentBoundedWorkers(t *testing.T) {
	requests := make(chan struct{}, 200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	urls := make([]string, 50)
	for i := range urls {
		urls[i] = server.URL
	}

	probe := newTestProbe()
	results, err := probe.ProbeHosts(context.Background(), urls, 5)
	if err != nil {
		t.Fatalf("ProbeHosts() error: %v", err)
	}
	if len(results) != 50 {
		t.Errorf("expected 50 results, got %d", len(results))
	}
}

func TestProbeHost_AddsHTTPSchemeIfMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Strip "http://" prefix to test auto-addition.
	rawAddr := strings.TrimPrefix(server.URL, "http://")
	probe := newTestProbe()
	host, err := probe.ProbeHost(context.Background(), rawAddr)
	if err != nil {
		t.Fatalf("ProbeHost() with bare address error: %v", err)
	}
	if host.StatusCode != 200 {
		t.Errorf("expected 200, got %d", host.StatusCode)
	}
}

func TestExtractTitle(t *testing.T) {
	cases := []struct {
		html  string
		title string
	}{
		{`<html><head><title>My Site</title></head></html>`, "My Site"},
		{`<html><head></head></html>`, ""},
		{`<title>  Trimmed  </title>`, "Trimmed"},
		{"no html here", ""},
	}
	for _, tc := range cases {
		got := extractTitle(tc.html)
		if got != tc.title {
			t.Errorf("extractTitle(%q) = %q, want %q", tc.html, got, tc.title)
		}
	}
}
