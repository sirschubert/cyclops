# Cyclops

A comprehensive reconnaissance tool for bug bounty hunters and penetration testers. Cyclops combines subdomain enumeration, host discovery, and endpoint discovery into a single, efficient binary.

## Features

- **Subdomain Enumeration**: 
  - Certificate transparency lookup (crt.sh)
  - DNS brute force with customizable wordlists
  - Zone transfer detection
  - Wildcard filtering

- **Host Discovery**: 
  - HTTP/HTTPS probing
  - Response analysis (status codes, titles, headers)
  - Technology fingerprinting
  - Content length measurement

- **Endpoint Discovery**: 
  - Web crawler (recursive link following)
  - JavaScript file analysis
  - robots.txt and sitemap.xml parsing
  - Directory brute forcing

- **Output Formats**:
  - JSON (structured, machine-readable)
  - HTML (visual report with styling)

- **Advanced Features**:
  - Configurable rate limiting and concurrency
  - Proxy support
  - Custom DNS resolvers
  - Timeout controls
  - Passive-only mode for stealth

## Installation

```bash
git clone https://github.com/sirschubert/cyclops.git
cd cyclops
go build ./cmd/cyclops
```

## Usage

### Basic Scan
```bash
./cyclops -d example.com
```

### Advanced Scan with Custom Settings
```bash
./cyclops -d example.com -t 100 -r 1000 -timeout 15 -user-agent "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" -format html -o report.html
```

### Passive-only Mode (No DNS brute force)
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

## Command Line Options

```
  -d string
    	Target domain
  -depth int
    	Crawl depth for endpoint discovery (default 2)
  -format string
    	Output format: json, html (default "json")
  -o string
    	Output file
  -passive-only
    	Only use passive sources (no DNS brute force)
  -proxy string
    	HTTP proxy URL
  -r int
    	Max requests per second (default 500)
  -resolvers string
    	Comma-separated list of DNS resolvers
  -t int
    	Concurrent workers (default 50)
  -timeout int
    	Request timeout in seconds (default 10)
  -user-agent string
    	User-Agent header (default "Cyclops/1.0")
  -v	Verbose output
  -w string
    	Subdomain wordlist path
```

## Building from Source

Requirements:
- Go 1.19 or higher

```bash
git clone https://github.com/sirschubert/cyclops.git
cd cyclops
go build ./cmd/cyclops
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License

## Author

Sir Schubert (@sirschubert)

## Disclaimer

This tool is intended for legitimate security testing and research purposes only. Users are responsible for complying with all applicable laws and regulations.