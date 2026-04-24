package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

func htmlSampleResult() models.Result {
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
						Title:      "My API <XSS test>",
						Tech:       []string{"Go"},
					},
				},
			},
		},
	}
}

func TestHTMLFormatter_Format_ContainsDomain(t *testing.T) {
	f := NewHTMLFormatter()
	data, err := f.Format(htmlSampleResult())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Error("HTML output does not contain the domain")
	}
}

func TestHTMLFormatter_Format_EscapesXSS(t *testing.T) {
	f := NewHTMLFormatter()
	data, err := f.Format(htmlSampleResult())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	html := string(data)
	// html/template must escape the raw < > in the title.
	if strings.Contains(html, "<XSS test>") {
		t.Error("HTML output contains unescaped XSS payload — html/template not used")
	}
}

func TestHTMLFormatter_Format_ContainsFullDetail(t *testing.T) {
	f := NewHTMLFormatter()
	data, err := f.Format(htmlSampleResult())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, "api.example.com") {
		t.Error("HTML output missing subdomain")
	}
	if !strings.Contains(html, "200") {
		t.Error("HTML output missing status code")
	}
}

func TestHTMLFormatter_WriteToFile(t *testing.T) {
	f := NewHTMLFormatter()
	tmp := filepath.Join(t.TempDir(), "report.html")
	if err := f.WriteToFile(htmlSampleResult(), tmp); err != nil {
		t.Fatalf("WriteToFile() error: %v", err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("could not read output file: %v", err)
	}
	if !strings.Contains(string(data), "<!DOCTYPE html>") {
		t.Error("output file does not look like HTML")
	}
}
