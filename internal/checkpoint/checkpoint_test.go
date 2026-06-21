package checkpoint

import (
	"path/filepath"
	"testing"

	"github.com/sirschubert/cyclops/pkg/models"
)

func TestRank(t *testing.T) {
	if !(Rank("") < Rank(PhaseSubdomains) &&
		Rank(PhaseSubdomains) < Rank(PhaseHosts) &&
		Rank(PhaseHosts) < Rank(PhaseEndpoints)) {
		t.Fatal("phase ranks are not strictly increasing")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := models.Result{
		Domain:   "example.com",
		ScanMode: "normal",
		Subdomains: []models.Subdomain{
			{Name: "www.example.com", Sources: []string{"cert"}},
		},
	}

	path, err := Save(dir, "example.com", "normal", PhaseHosts, want)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if path != PathFor(dir, "example.com") {
		t.Errorf("Save path = %q, want %q", path, PathFor(dir, "example.com"))
	}

	cp, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cp.Phase != PhaseHosts {
		t.Errorf("phase = %q, want %q", cp.Phase, PhaseHosts)
	}
	if cp.Result.Domain != want.Domain || len(cp.Result.Subdomains) != 1 {
		t.Errorf("result not round-tripped: %+v", cp.Result)
	}
	if cp.Result.Subdomains[0].Name != "www.example.com" {
		t.Errorf("subdomain name = %q", cp.Result.Subdomains[0].Name)
	}
}

func TestPathForSanitizesDomain(t *testing.T) {
	got := PathFor("/tmp", "weird/domain:1234")
	want := filepath.Join("/tmp", "weird_domain_1234.cyclops-checkpoint.json")
	if got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
}
