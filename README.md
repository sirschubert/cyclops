## Cyclops
 
A fast, all-in-one reconnaissance tool for bug bounty hunters and penetration testers. Cyclops combines subdomain enumeration, host discovery, and endpoint crawling into a single binary — no more juggling three separate tools.
 
> *"Be advised: the Cyclops is designed to be operated by a three-person crew. Only experienced helms-people should attempt to pilot this vehicle solo."*
> — Alterra
 
---
# Features

### Subdomain Enumeration
- **Certificate Transparency**: Query crt.sh for certificate-based subdomain discovery
- **DNS Brute Force**: Multi-threaded DNS resolution with customizable wordlists
- **Zone Transfer**: Attempt DNS zone transfers for exposed subdomains
- **Wildcard Detection**: Automatically detects and warns about wildcard DNS records
- **Multi-Resolver Support**: Round-robin DNS resolution across multiple nameservers

### Host Discovery
- **HTTP/HTTPS Probing**: Fast concurrent probing with configurable timeouts
- **Response Analysis**: Capture status codes, titles, headers, and content length
- **Technology Fingerprinting**: Identify server technologies from headers

### Endpoint Discovery
- **Web Crawler**: Recursive breadth-first crawling with configurable depth
- **Robots.txt Parsing**: Extract disallowed paths and sitemap references
- **Sitemap.xml Parsing**: Parse XML sitemaps including nested sitemap indexes
- **Link Extraction**: Parse HTML for href, src, and action attributes

### Output Formats
- **JSON**: Structured, machine-readable output for easy integration
- **HTML**: Visual report with color-coded status codes and organized layout

### Advanced Features
- **Rate Limiting**: Configurable requests per second with token bucket algorithm
- **Proxy Support**: Route traffic through HTTP/HTTPS proxies
- **Custom DNS Resolvers**: Use specific nameservers for DNS queries
- **Timeout Controls**: Per-request and total scan timeout support
- **Passive-Only Mode**: Skip active DNS brute force for stealth scanning
- **Structured Logging**: Debug and warning levels with `log/slog`

## Installation

### From Source
```bash
git clone https://github.com/sirschubert/cyclops.git
cd cyclops
go build ./cmd/cyclops
```

### Requirements
- Go 1.19 or higher

## Usage

### Basic Scan
```bash
./cyclops -d example.com
```

### Advanced Scan with Custom Settings
```bash
./cyclops -d example.com \
  -t 100 \
  -r 1000 \
  -timeout 15 \
  -scan-timeout 30 \
  -user-agent "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
  -format html \
  -o report.html
```

### Passive-Only Mode
Skip active DNS brute force for stealth scanning:
```bash
./cyclops -d example.com -passive-only -format json -o results.json
```

### With Proxy Support
```bash
./cyclops -d example.com -proxy http://127.0.0.1:8080
```

### Custom DNS Resolvers
```bash
./cyclops -d example.com -resolvers "8.8.8.8,1.1.1.1,9.9.9.9"
```

### With Custom Wordlist
```bash
./cyclops -d example.com -w wordlists/custom.txt -t 50
```

### Verbose Output
```bash
./cyclops -d example.com -v
```

## Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-d` | Target domain (required) | - |
| `-w` | Subdomain wordlist file path | Built-in wordlist |
| `-t` | Concurrent workers | 50 |
| `-r` | Max requests per second | 500 |
| `-o` | Output file | stdout |
| `-format` | Output format: `json`, `html` | json |
| `-depth` | Crawl depth for endpoint discovery | 2 |
| `-passive-only` | Only use passive sources (no DNS brute force) | false |
| `-proxy` | HTTP proxy URL | - |
| `-resolvers` | Comma-separated DNS resolvers (e.g. `8.8.8.8,1.1.1.1`) | 8.8.8.8:53 |
| `-timeout` | Per-request timeout in seconds | 10 |
| `-scan-timeout` | Total scan timeout in minutes (0 = no limit) | 30 |
| `-user-agent` | Custom User-Agent header | Cyclops/1.0 |
| `-v` | Verbose output (debug logging) | false |

## Output Examples

### JSON Output
```json
{
  "domain": "example.com",
  "scan_time": "2026-04-24T08:59:30Z",
  "subdomains": [
    {
      "subdomain": "www.example.com",
      "sources": ["mixed"],
      "ip": "93.184.216.34",
      "hosts": [
        {
          "url": "https://www.example.com",
          "scheme": "https",
          "status_code": 200,
          "title": "Example Domain",
          "server": "nginx",
          "tech": ["nginx"],
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

### HTML Output
The HTML report provides a visual layout with:
- Color-coded status codes (green 2xx, blue 3xx, orange 4xx, red 5xx)
- Organized subdomain → host → endpoint hierarchy
- Technology tags for quick identification
- Direct links to discovered endpoints

## Project Structure

```
cyclops/
├── cmd/cyclops/main.go          # CLI entrypoint with structured logging
├── internal/
│   ├── subdomains/              # DNS enumeration and certificate lookup
│   │   ├── dns.go               # DNS resolver with brute force, zone transfer
│   │   ├── cert.go              # Certificate transparency (crt.sh)
│   │   └── engine.go            # Discovery engine combining all methods
│   ├── hosts/                   # Host discovery
│   │   ├── probe.go             # HTTP/HTTPS probing
│   │   └── fingerprint.go       # Technology detection
│   ├── endpoints/               # Endpoint discovery
│   │   ├── crawler.go           # Web crawler with rate limiting
│   │   ├── robots.go            # robots.txt and sitemap.xml parser
│   │   └── wordlist.go          # Directory brute forcing
│   ├── output/                  # Output formatters
│   │   ├── json.go              # JSON formatter
│   │   └── html.go              # HTML report formatter
│   └── utils/                   # Utility functions
│       ├── pool.go              # Worker pool for concurrency
│       └── rate.go              # Rate limiter with context support
├── pkg/models/types.go          # Shared data structures
├── wordlists/default.txt        # Built-in subdomain wordlist
├── README.md                    # This file
├── go.mod/go.sum                # Go modules
```

## Architecture

The tool follows a three-phase pipeline:

1. **Subdomain Discovery**: Certificate transparency + DNS enumeration + zone transfers
2. **Host Discovery**: Concurrent HTTP/HTTPS probing with technology fingerprinting
3. **Endpoint Discovery**: Web crawling + robots.txt + sitemap.xml parsing

All phases respect context cancellation and rate limiting for controlled scanning.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License

## Author

Sir Schubert ([@sirschubert](https://github.com/sirschubert))

## Disclaimer

This tool is intended for legitimate security testing and research purposes only. Users are responsible for complying with all applicable laws and regulations and obtaining proper authorization before scanning any systems.
