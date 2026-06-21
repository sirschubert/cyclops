// Package checkpoint persists scan progress so a scan can be resumed after an
// interrupt, crash, or timeout without redoing completed phases.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

// Phase identifies the last fully-completed scan phase recorded in a checkpoint.
const (
	PhaseSubdomains = "subdomains"
	PhaseHosts      = "hosts"
	PhaseEndpoints  = "endpoints"
)

// Rank returns an ordering for a phase so callers can decide what to skip on
// resume. An unknown/empty phase ranks 0 (nothing completed).
func Rank(phase string) int {
	switch phase {
	case PhaseSubdomains:
		return 1
	case PhaseHosts:
		return 2
	case PhaseEndpoints:
		return 3
	default:
		return 0
	}
}

// Checkpoint is the on-disk representation of a scan's progress.
type Checkpoint struct {
	Domain  string        `json:"domain"`
	Mode    string        `json:"mode"`
	Phase   string        `json:"phase"`
	Result  models.Result `json:"result"`
	SavedAt time.Time     `json:"saved_at"`
}

// PathFor returns the deterministic checkpoint file path for a domain inside dir.
func PathFor(dir, domain string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_").Replace(domain)
	return filepath.Join(dir, safe+".cyclops-checkpoint.json")
}

// Save writes the checkpoint for the given phase to dir, creating dir if needed.
// It returns the path written.
func Save(dir, domain, mode, phase string, result models.Result) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create checkpoint dir: %w", err)
	}
	cp := Checkpoint{
		Domain:  domain,
		Mode:    mode,
		Phase:   phase,
		Result:  result,
		SavedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal checkpoint: %w", err)
	}
	path := PathFor(dir, domain)
	// Write atomically via a temp file + rename so an interrupt mid-write can't
	// leave a corrupt checkpoint.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write checkpoint: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("finalize checkpoint: %w", err)
	}
	return path, nil
}

// Load reads and parses a checkpoint file.
func Load(path string) (*Checkpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	return &cp, nil
}
