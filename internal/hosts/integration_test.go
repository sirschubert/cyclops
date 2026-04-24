package hosts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirschubert/cyclops/internal/hosts"
	"github.com/sirschubert/cyclops/pkg/models"
)

// TestProbeAndFingerprint_Integration exercises the full probe → fingerprint
// pipeline with a realistic mock server.
func TestProbeAndFingerprint_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.18")
		w.Header().Set("X-Powered-By", "PHP/8.1")
		w.WriteHeader(200)
		w.Write([]byte(`<html>
<head><title>Test App</title></head>
<body>
  wp-content
</body>
</html>`))
	}))
	defer server.Close()

	probe := hosts.NewHTTPProbe(5, "Cyclops/test")
	h, err := probe.ProbeHost(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("ProbeHost() error: %v", err)
	}

	if h.StatusCode != 200 {
		t.Errorf("status: want 200, got %d", h.StatusCode)
	}
	if h.Title != "Test App" {
		t.Errorf("title: want 'Test App', got %q", h.Title)
	}
	if h.Server != "nginx/1.18" {
		t.Errorf("server: want 'nginx/1.18', got %q", h.Server)
	}

	// Run fingerprinting.
	results := hosts.FingerprintHosts([]models.Host{*h})
	if len(results) == 0 {
		t.Fatal("FingerprintHosts returned empty slice")
	}
	fingered := results[0]

	techSet := make(map[string]bool)
	for _, tech := range fingered.Tech {
		techSet[tech] = true
	}

	if !techSet["Nginx"] {
		t.Error("expected Nginx to be detected from Server header")
	}
	if !techSet["PHP"] {
		t.Error("expected PHP to be detected from X-Powered-By header")
	}
	if !techSet["WordPress"] {
		t.Error("expected WordPress to be detected from body (wp-content)")
	}
}
