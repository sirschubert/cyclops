package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirschubert/cyclops/pkg/models"
)

// MarkdownFormatter formats scan results as a Markdown document.
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new Markdown formatter.
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format converts the scan result to Markdown.
func (mf *MarkdownFormatter) Format(result models.Result) []byte {
	var sb strings.Builder

	totalHosts := 0
	totalEndpoints := 0
	for _, sub := range result.Subdomains {
		totalHosts += len(sub.Hosts)
		for _, h := range sub.Hosts {
			totalEndpoints += len(h.Endpoints)
		}
	}

	mode := result.ScanMode
	if mode == "" {
		mode = "normal"
	}

	fmt.Fprintf(&sb, "# Cyclops Scan Report\n\n")
	fmt.Fprintf(&sb, "| Field | Value |\n")
	fmt.Fprintf(&sb, "|---|---|\n")
	fmt.Fprintf(&sb, "| Target | `%s` |\n", result.Domain)
	fmt.Fprintf(&sb, "| Mode | %s |\n", mode)
	fmt.Fprintf(&sb, "| Scan Time | %s |\n", result.ScanTime.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&sb, "| Subdomains | %d |\n", len(result.Subdomains))
	fmt.Fprintf(&sb, "| Live Hosts | %d |\n", totalHosts)
	fmt.Fprintf(&sb, "| Endpoints | %d |\n\n", totalEndpoints)

	if len(result.Subdomains) == 0 {
		fmt.Fprintf(&sb, "_No results found._\n")
		return []byte(sb.String())
	}

	fmt.Fprintf(&sb, "## Subdomains\n\n")

	for _, sub := range result.Subdomains {
		fmt.Fprintf(&sb, "### %s\n\n", sub.Name)
		if sub.IP != "" {
			fmt.Fprintf(&sb, "- **IP:** `%s`\n", sub.IP)
		}
		if len(sub.Sources) > 0 {
			fmt.Fprintf(&sb, "- **Sources:** %s\n", strings.Join(sub.Sources, ", "))
		}
		fmt.Fprintln(&sb)

		for _, host := range sub.Hosts {
			fmt.Fprintf(&sb, "#### `%s` — %d\n\n", host.URL, host.StatusCode)
			if host.Title != "" {
				fmt.Fprintf(&sb, "- **Title:** %s\n", host.Title)
			}
			if host.Server != "" {
				fmt.Fprintf(&sb, "- **Server:** `%s`\n", host.Server)
			}
			if len(host.Tech) > 0 {
				fmt.Fprintf(&sb, "- **Tech:** %s\n", strings.Join(host.Tech, ", "))
			}
			if host.ContentLength > 0 {
				fmt.Fprintf(&sb, "- **Size:** %d bytes\n", host.ContentLength)
			}

			if len(host.Endpoints) > 0 {
				fmt.Fprintf(&sb, "\n**Endpoints:**\n\n")
				fmt.Fprintf(&sb, "| URL | Status | Source |\n")
				fmt.Fprintf(&sb, "|---|---|---|\n")
				for _, ep := range host.Endpoints {
					statusStr := ""
					if ep.StatusCode > 0 {
						statusStr = fmt.Sprintf("%d", ep.StatusCode)
					}
					fmt.Fprintf(&sb, "| `%s` | %s | %s |\n", ep.URL, statusStr, ep.Source)
				}
				fmt.Fprintln(&sb)
			} else {
				fmt.Fprintln(&sb)
			}
		}
	}

	return []byte(sb.String())
}

// WriteToFile writes the Markdown result to a file.
func (mf *MarkdownFormatter) WriteToFile(result models.Result, filename string) error {
	return os.WriteFile(filename, mf.Format(result), 0644)
}

// WriteToStdout writes the Markdown result to stdout.
func (mf *MarkdownFormatter) WriteToStdout(result models.Result) error {
	fmt.Print(string(mf.Format(result)))
	return nil
}
