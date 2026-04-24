
```
                 ██████╗██╗   ██╗ ██████╗██╗      ██████╗ ██████╗ ███████╗
                ██╔════╝╚██╗ ██╔╝██╔════╝██║     ██╔═══██╗██╔══██╗██╔════╝
                ██║      ╚████╔╝ ██║     ██║     ██║   ██║██████╔╝███████╗
                ██║       ╚██╔╝  ██║     ██║     ██║   ██║██╔═══╝ ╚════██║
                ╚██████╗   ██║   ╚██████╗███████╗╚██████╔╝██║     ███████║
                 ╚═════╝   ╚═╝    ╚═════╝╚══════╝ ╚═════╝ ╚═╝     ╚══════╝
                 
  ~ "Welcome aboard Captain, all systems online."                                         
```
A fast, all-in-one reconnaissance tool for bug bounty hunters and penetration testers. Cyclops combines subdomain enumeration, host discovery, and endpoint crawling into a single binary — no more juggling three separate tools.

> *"Be advised: the Cyclops is designed to be operated by a three-person crew. Only experienced helms-people should attempt to pilot this vehicle solo."*
> — Alterra

---

## Installation

### go install (recommended)
```bash
go install github.com/sirschubert/cyclops/cmd/cyclops@latest
```

The binary will be placed in `$GOPATH/bin` (or `$HOME/go/bin`). Make sure that directory is on your `$PATH`.

### From source
```bash
git clone https://github.com/sirschubert/cyclops.git
cd cyclops
go build -o cyclops ./cmd/cyclops
```

### Requirements
Go 1.22 or higher.

---

## Features

### Subdomain Enumeration
- **Certificate Transparency** — query crt.sh for certificate-based subdomain discovery
- **DNS Brute Force** — multi-threaded DNS resolution with a built-in or custom wordlist
- **Zone Transfer** — attempt DNS zone transfers across all nameservers
- **Wildcard Detection** — detects wildcard DNS records and skips useless brute force
- **Multi-Resolver Support** — round-robin DNS resolution across multiple nameservers

### Host Discovery
- **HTTP/HTTPS Probing** — concurrent probing with configurable timeouts and redirect following
- **Response Analysis** — status codes, page titles, headers, content length
- **Technology Fingerprinting** — identify server software from response headers and body signatures

### Endpoint Discovery
- **Web Crawler** — breadth-first crawling with configurable depth
- **robots.txt / sitemap.xml** — extract disallowed paths and nested sitemap entries
- **Link Extraction** — follows `href`, `src`, and `form action` attributes

### Scan Modes
Three built-in modes tune concurrency, rate, and stealth behaviour simultaneously:

| Mode | Workers | Rate | Depth | Extras |
|------|---------|------|-------|--------|
| `normal` (default) | 50 | 500 req/s | 2 | — |
| `stealth` | 5 | 10 req/s | 2 | passive-only, 1–4 s random delay, browser UA rotation |
| `aggressive` | 200 | 2000 req/s | 4 | all sources enabled |

Explicit `-t`, `-r`, or `-depth` flags always override mode defaults.

### Auto-Tune Rate Limiter
`-autotune` starts at your configured rate and adjusts it automatically during the scan:
- Every 30 seconds with no errors: `rate += 50 req/s`
- On any 429 or 503 response: `rate /= 2` immediately
- Floor: 10 req/s — Ceiling: 2000 req/s

### Interactive Mode
`-interactive` pauses between each phase so you can review results and pick which targets to continue with — useful for large scopes where you want to focus on specific subdomains or hosts before crawling.

### Output
- **JSON** — structured, machine-readable, suitable for piping into other tools
- **HTML** — dark-themed terminal-style report with collapsible sections, status-code badges, and technology tags; no external dependencies

---

## Usage

### Basic scan
```bash
cyclops -d example.com
```

### Stealth scan — passive sources only, slow and quiet
```bash
cyclops -d example.com -mode stealth -format json -o results.json
```

### Aggressive scan — maximum speed
```bash
cyclops -d example.com -mode aggressive -format html -o report.html
```

### Auto-tune the rate limiter
```bash
cyclops -d example.com -autotune -v
```

### Interactive mode — review and select between phases
```bash
cyclops -d example.com -interactive
```

### HTML report
```bash
cyclops -d example.com -format html -o report.html
```

### Custom wordlist and resolvers
```bash
cyclops -d example.com -w wordlists/custom.txt -resolvers "8.8.8.8,1.1.1.1,9.9.9.9"
```

### Through a proxy with verbose output
```bash
cyclops -d example.com -proxy http://127.0.0.1:8080 -v
```

---

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-d` | Target domain **(required)** | — |
| `-mode` | Scan mode: `normal`, `stealth`, `aggressive` | `normal` |
| `-t` | Concurrent workers | 50 |
| `-r` | Max requests per second | 500 |
| `-depth` | Crawl depth for endpoint discovery | 2 |
| `-autotune` | Dynamically adjust rate based on server responses | false |
| `-interactive` | Pause between phases to review and select targets | false |
| `-passive-only` | Skip DNS brute force (passive sources only) | false |
| `-w` | Subdomain wordlist file | built-in |
| `-resolvers` | Comma-separated DNS resolvers (e.g. `8.8.8.8,1.1.1.1`) | `8.8.8.8:53` |
| `-format` | Output format: `json`, `html` | `json` |
| `-o` | Output file (default: stdout) | — |
| `-proxy` | HTTP proxy URL | — |
| `-timeout` | Per-request timeout in seconds | 10 |
| `-scan-timeout` | Total scan timeout in minutes (0 = no limit) | 30 |
| `-user-agent` | Custom User-Agent string | `Cyclops/1.0` |
| `-v` | Verbose output | false |

---

## Output example

```json
{
  "domain": "example.com",
  "scan_time": "2026-04-24T08:59:30Z",
  "scan_mode": "normal",
  "subdomains": [
    {
      "subdomain": "www.example.com",
      "sources": ["mixed"],
      "hosts": [
        {
          "url": "https://www.example.com",
          "scheme": "https",
          "status_code": 200,
          "title": "Example Domain",
          "server": "nginx",
          "tech": ["Nginx", "WordPress"],
          "content_length": 1256,
          "endpoints": [
            {
              "url": "https://www.example.com/robots.txt",
              "method": "GET",
              "status_code": 200,
              "source": "robots"
            }
          ]
        }
      ]
    }
  ]
}
```

---

## Author

Sir Schubert ([@sirschubert](https://github.com/sirschubert))
> from one subnautica fan to the whole world with ❤️

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Disclaimer

This tool is intended for legitimate security testing and research purposes only. Users are responsible for complying with all applicable laws and regulations and obtaining proper authorization before scanning any systems.
