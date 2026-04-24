package output

import (
	"bytes"
	"fmt"
	"html/template"
	"os"

	"github.com/sirschubert/cyclops/pkg/models"
)

const reportTmpl = `<!DOCTYPE html>
<html>
<head>
    <title>Cyclops Scan Results for {{.Domain}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f9f9f9; }
        h1, h2, h3 { color: #333; }
        .subdomain { margin-bottom: 20px; border: 1px solid #ddd; padding: 10px; background: #fff; }
        .host { margin-left: 20px; margin-top: 10px; border: 1px solid #eee; padding: 10px; }
        .endpoint { margin-left: 40px; margin-top: 5px; font-size: 0.9em; }
        .status-2xx { color: green; }
        .status-3xx { color: blue; }
        .status-4xx { color: orange; }
        .status-5xx { color: red; }
        .tech { background-color: #f0f0f0; padding: 2px 5px; margin-right: 5px; border-radius: 3px; }
        .source { font-size: 0.8em; color: #666; }
    </style>
</head>
<body>
    <h1>Cyclops Scan Results</h1>
    <h2>Target: {{.Domain}}</h2>
    <p>Scan Time: {{.ScanTime}}</p>

    <h3>Discovered Subdomains ({{len .Subdomains}})</h3>
    {{range .Subdomains}}
    <div class="subdomain">
        <h4>{{.Name}}</h4>
        <p>Sources: {{range .Sources}}{{.}} {{end}}</p>
        {{if .Hosts}}
        <h5>Hosts:</h5>
        {{range .Hosts}}
        <div class="host">
            <strong><a href="{{.URL}}" target="_blank">{{.URL}}</a></strong>
            <span class="status-{{.StatusCode | statusClass}}">[{{.StatusCode}}]</span>
            {{if .Title}}<br>Title: {{.Title}}{{end}}
            {{if .Server}}<br>Server: {{.Server}}{{end}}
            {{if .Tech}}
            <br>Tech: {{range .Tech}}<span class="tech">{{.}}</span>{{end}}
            {{end}}
            {{if .ContentLength}}<br>Content Length: {{.ContentLength}}{{end}}

            {{if .Endpoints}}
            <br><br>Endpoints ({{len .Endpoints}}):
            {{range .Endpoints}}
            <div class="endpoint">
                <a href="{{.URL}}" target="_blank">{{.URL}}</a>
                <span class="status-{{.StatusCode | statusClass}}">[{{.StatusCode}}]</span>
                <span class="source">({{.Source}})</span>
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}
        {{end}}
    </div>
    {{end}}
</body>
</html>`

// reportTemplate is parsed once at init; template.Must panics at startup on
// syntax errors in the static template string, making the bug immediately
// visible rather than mid-scan.
var reportTemplate = template.Must(
	template.New("report").Funcs(template.FuncMap{
		"statusClass": func(code int) string {
			switch {
			case code >= 200 && code < 300:
				return "2xx"
			case code >= 300 && code < 400:
				return "3xx"
			case code >= 400 && code < 500:
				return "4xx"
			case code >= 500:
				return "5xx"
			default:
				return ""
			}
		},
	}).Parse(reportTmpl),
)

// HTMLFormatter formats scan results as an HTML report.
type HTMLFormatter struct{}

// NewHTMLFormatter creates a new HTML formatter.
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{}
}

// Format converts the scan result to HTML using the report template.
func (hf *HTMLFormatter) Format(result models.Result) ([]byte, error) {
	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, result); err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteToFile writes the HTML result to a file.
func (hf *HTMLFormatter) WriteToFile(result models.Result, filename string) error {
	data, err := hf.Format(result)
	if err != nil {
		return fmt.Errorf("failed to format HTML: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

// WriteToStdout writes the HTML result to stdout.
func (hf *HTMLFormatter) WriteToStdout(result models.Result) error {
	data, err := hf.Format(result)
	if err != nil {
		return fmt.Errorf("failed to format HTML: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
