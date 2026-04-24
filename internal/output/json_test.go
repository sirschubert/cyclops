package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

func sampleResult() models.Result {
	return models.Result{
		Domain:   "example.com",
		ScanTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Subdomains: []models.Subdomain{
			{
				Name:    "api.example.com",
				Sources: []string{"cert"},
				Hosts: []models.Host{
					{
						URL:        "https://api.example.com",
						StatusCode: 200,
						Title:      "API",
					},
				},
			},
		},
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	f := NewJSONFormatter()
	data, err := f.Format(sampleResult())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out["domain"] != "example.com" {
		t.Errorf("expected domain=example.com, got %v", out["domain"])
	}
}

func TestJSONFormatter_WriteToFile(t *testing.T) {
	f := NewJSONFormatter()
	tmp := filepath.Join(t.TempDir(), "out.json")
	if err := f.WriteToFile(sampleResult(), tmp); err != nil {
		t.Fatalf("WriteToFile() error: %v", err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("could not read output file: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("file content is not valid JSON: %v", err)
	}
}

func TestJSONFormatter_WriteToStdout(t *testing.T) {
	f := NewJSONFormatter()
	// Should not panic or return an error.
	if err := f.WriteToStdout(sampleResult()); err != nil {
		t.Fatalf("WriteToStdout() error: %v", err)
	}
}
