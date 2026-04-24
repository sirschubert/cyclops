package output

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

const reportTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Cyclops — {{.Domain}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#0d1117;color:#c9d1d9;font-family:monospace;font-size:14px;padding:24px}
.container{max-width:960px;margin:0 auto}
.site-header{border-bottom:1px solid #30363d;padding-bottom:20px;margin-bottom:24px}
.site-header h1{color:#58a6ff;font-size:22px;margin-bottom:16px;letter-spacing:0.5px}
.meta-table{border-collapse:collapse}
.meta-table td{padding:3px 16px 3px 0;vertical-align:top}
.meta-table td:first-child{color:#58a6ff;min-width:130px}
.meta-table td:last-child{color:#e6edf3}
details{margin-bottom:16px}
details[open]>summary{margin-bottom:12px}
summary{cursor:pointer;color:#58a6ff;font-weight:bold;font-size:13px;padding:8px 0;
        list-style:none;user-select:none;border-bottom:1px solid #21262d}
summary::-webkit-details-marker{display:none}
summary::before{content:"▶ ";font-size:11px}
details[open]>summary::before{content:"▼ "}
.card{background:#161b22;border:1px solid #30363d;border-radius:4px;padding:12px;margin-bottom:8px}
.card-title{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-bottom:4px}
.card a{color:#58a6ff;text-decoration:none}
.card a:hover{text-decoration:underline}
.badge{display:inline-block;padding:1px 7px;border-radius:3px;font-size:12px;font-weight:bold}
.s2xx{background:#1a3a2688;color:#2ea043;border:1px solid #2ea04366}
.s3xx{background:#3a2e1088;color:#d29922;border:1px solid #d2992266}
.s4xx{background:#3a101088;color:#f85149;border:1px solid #f8514966}
.s5xx{background:#3a101088;color:#f85149;border:1px solid #f8514966}
.sunk{background:#21262d;color:#8b949e;border:1px solid #30363d}
.tech-tag{display:inline-block;background:#21262d;color:#8b949e;border:1px solid #30363d;
          border-radius:3px;padding:1px 7px;font-size:12px;margin:2px 2px 2px 0}
.meta-line{color:#8b949e;font-size:12px;margin-top:6px}
.nested{margin-top:10px;margin-left:0px}
.count-badge{display:inline-block;background:#21262d;color:#8b949e;border-radius:10px;
             padding:1px 8px;font-size:11px;margin-left:6px;font-weight:normal}
</style>
</head>
<body>
<div class="container">

  <div class="site-header">
    <h1>Cyclops Scan Report</h1>
    <table class="meta-table">
      <tr><td>Target</td><td>{{.Domain}}</td></tr>
      <tr><td>Scan Mode</td><td>{{if .ScanMode}}{{.ScanMode}}{{else}}normal{{end}}</td></tr>
      <tr><td>Timestamp</td><td>{{.ScanTime | fmtTime}}</td></tr>
      <tr><td>Subdomains</td><td>{{len .Subdomains}}</td></tr>
      <tr><td>Live Hosts</td><td>{{. | totalHosts}}</td></tr>
      <tr><td>Endpoints</td><td>{{. | totalEndpoints}}</td></tr>
    </table>
  </div>

  <details open>
    <summary>[SUBDOMAINS] <span class="count-badge">{{len .Subdomains}}</span></summary>
    {{range .Subdomains}}
    <div class="card">
      <div class="card-title">
        <strong>{{.Name}}</strong>
        {{if .IP}}<span class="meta-line">{{.IP}}</span>{{end}}
      </div>
      {{if .Sources}}<div class="meta-line">sources: {{range .Sources}}{{.}} {{end}}</div>{{end}}

      {{if .Hosts}}
      <div class="nested">
        <details open>
          <summary>[HOSTS] <span class="count-badge">{{len .Hosts}}</span></summary>
          {{range .Hosts}}
          <div class="card">
            <div class="card-title">
              <a href="{{.URL}}" target="_blank" rel="noopener">{{.URL}}</a>
              <span class="badge {{.StatusCode | statusClass}}">{{.StatusCode}}</span>
            </div>
            {{if .Title}}<div class="meta-line">title: {{.Title}}</div>{{end}}
            {{if .Server}}<div class="meta-line">server: {{.Server}}</div>{{end}}
            {{if .ContentLength}}<div class="meta-line">size: {{.ContentLength}} bytes</div>{{end}}
            {{if .Tech}}
            <div style="margin-top:6px">
              {{range .Tech}}<span class="tech-tag">{{.}}</span>{{end}}
            </div>
            {{end}}

            {{if .Endpoints}}
            <div class="nested">
              <details open>
                <summary>[ENDPOINTS] <span class="count-badge">{{len .Endpoints}}</span></summary>
                {{range .Endpoints}}
                <div class="card">
                  <div class="card-title">
                    <a href="{{.URL}}" target="_blank" rel="noopener">{{.URL}}</a>
                    {{if .StatusCode}}<span class="badge {{.StatusCode | statusClass}}">{{.StatusCode}}</span>{{end}}
                    <span class="meta-line">{{.Source}}</span>
                  </div>
                </div>
                {{end}}
              </details>
            </div>
            {{end}}
          </div>
          {{end}}
        </details>
      </div>
      {{end}}
    </div>
    {{end}}
  </details>

</div>
</body>
</html>`

var reportTemplate = template.Must(
	template.New("report").Funcs(template.FuncMap{
		"statusClass": func(code int) string {
			switch {
			case code >= 200 && code < 300:
				return "s2xx"
			case code >= 300 && code < 400:
				return "s3xx"
			case code >= 400 && code < 500:
				return "s4xx"
			case code >= 500:
				return "s5xx"
			default:
				return "sunk"
			}
		},
		"fmtTime": func(t time.Time) string {
			return t.UTC().Format("2006-01-02 15:04:05 UTC")
		},
		"totalHosts": func(r models.Result) int {
			n := 0
			for _, sub := range r.Subdomains {
				n += len(sub.Hosts)
			}
			return n
		},
		"totalEndpoints": func(r models.Result) int {
			n := 0
			for _, sub := range r.Subdomains {
				for _, h := range sub.Hosts {
					n += len(h.Endpoints)
				}
			}
			return n
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
