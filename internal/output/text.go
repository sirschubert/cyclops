package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirschubert/cyclops/pkg/models"
)

// TextFormatter formats scan results as human-readable text.
type TextFormatter struct{}

// NewTextFormatter creates a new text formatter.
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{}
}

// Format converts the scan result to human-readable text.
func (tf *TextFormatter) Format(result models.Result) []byte {
	var sb strings.Builder

	totalHosts := 0
	totalEndpoints := 0
	for _, sub := range result.Subdomains {
		totalHosts += len(sub.Hosts)
		for _, h := range sub.Hosts {
			totalEndpoints += len(h.Endpoints)
		}
	}

	fmt.Fprintf(&sb, "╔══════════════════════════════════════╗\n")
	fmt.Fprintf(&sb, "║       Cyclops Scan Report            ║\n")
	fmt.Fprintf(&sb, "╚══════════════════════════════════════╝\n\n")
	fmt.Fprintf(&sb, "  Target     : %s\n", result.Domain)
	mode := result.ScanMode
	if mode == "" {
		mode = "normal"
	}
	fmt.Fprintf(&sb, "  Mode       : %s\n", mode)
	fmt.Fprintf(&sb, "  Scan Time  : %s\n", result.ScanTime.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&sb, "  Subdomains : %d\n", len(result.Subdomains))
	fmt.Fprintf(&sb, "  Live Hosts : %d\n", totalHosts)
	fmt.Fprintf(&sb, "  Endpoints  : %d\n", totalEndpoints)
	fmt.Fprintf(&sb, "\n")

	if len(result.Subdomains) == 0 {
		fmt.Fprintf(&sb, "  (no results)\n")
		return []byte(sb.String())
	}

	for _, sub := range result.Subdomains {
		fmt.Fprintf(&sb, "┌─ [SUBDOMAIN] %s\n", sub.Name)
		if sub.IP != "" {
			fmt.Fprintf(&sb, "│   IP      : %s\n", sub.IP)
		}
		if len(sub.Sources) > 0 {
			fmt.Fprintf(&sb, "│   Sources : %s\n", strings.Join(sub.Sources, ", "))
		}

		for hi, host := range sub.Hosts {
			connector := "├"
			if hi == len(sub.Hosts)-1 && len(host.Endpoints) == 0 {
				connector = "└"
			}
			fmt.Fprintf(&sb, "│\n%s── [HOST] %s  [%d]\n", connector, host.URL, host.StatusCode)
			if host.Title != "" {
				fmt.Fprintf(&sb, "│     Title  : %s\n", host.Title)
			}
			if host.Server != "" {
				fmt.Fprintf(&sb, "│     Server : %s\n", host.Server)
			}
			if len(host.Tech) > 0 {
				fmt.Fprintf(&sb, "│     Tech   : %s\n", strings.Join(host.Tech, ", "))
			}
			if host.ContentLength > 0 {
				fmt.Fprintf(&sb, "│     Size   : %d bytes\n", host.ContentLength)
			}

			for _, ep := range host.Endpoints {
				statusStr := ""
				if ep.StatusCode > 0 {
					statusStr = fmt.Sprintf("  [%d]", ep.StatusCode)
				}
				fmt.Fprintf(&sb, "│     ↳ %s%s  (%s)\n", ep.URL, statusStr, ep.Source)
			}
		}
		fmt.Fprintf(&sb, "\n")
	}

	return []byte(sb.String())
}

// WriteToFile writes the text result to a file.
func (tf *TextFormatter) WriteToFile(result models.Result, filename string) error {
	return os.WriteFile(filename, tf.Format(result), 0644)
}

// WriteToStdout writes the text result to stdout.
func (tf *TextFormatter) WriteToStdout(result models.Result) error {
	fmt.Print(string(tf.Format(result)))
	return nil
}
