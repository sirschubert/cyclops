
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

## Quick Start

### Install
```bash
go install github.com/sirschubert/cyclops/cmd/cyclops@latest
```
Requires Go 1.25+.

### Example Commands

**Basic scan:**
```bash
cyclops -d example.com
```

**Stealth mode with JSON output:**
```bash
cyclops -d example.com -mode stealth -format json -o results.json
```

**Aggressive scan:**
```bash
cyclops -d example.com -mode aggressive -format html -o report.html
```

---

## Features

- **Subdomain Enumeration** — Certificate Transparency, DNS brute force, zone transfers, wildcard detection, and multi-resolver support
- **Host Discovery** — HTTP/HTTPS probing with response analysis and technology fingerprinting
- **Endpoint Discovery** — Web crawling with robots.txt/sitemap.xml extraction and directory bruteforce
- **Scan Modes** — Tuned presets: `normal`, `stealth` (passive, slow), and `aggressive` (fast)
- **Checkpoint & Resume** — Save progress with `-checkpoint-dir`, resume with `-resume`
- **SSRF Metadata Guard** — Block connections to cloud-metadata and link-local addresses
- **Silent Mode** — Clean, pipeable output for scripting
- **Multiple Output Formats** — text, JSON, HTML, and Markdown

---

## Documentation

For detailed setup, usage, and configuration:

- [Installation](https://github.com/sirschubert/cyclops/wiki/Installation)
- [Usage & Examples](https://github.com/sirschubert/cyclops/wiki/Usage)
- [Options Reference](https://github.com/sirschubert/cyclops/wiki/Options-Reference)
- [Output Formats](https://github.com/sirschubert/cyclops/wiki/Output-Formats)
- [Advanced Features](https://github.com/sirschubert/cyclops/wiki/Advanced-Features)

---

## Author

Sir Schubert ([@sirschubert](https://github.com/sirschubert))
> from one subnautica fan to the whole world with ❤️

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Disclaimer

This tool is intended for legitimate security testing and research purposes only. Users are responsible for complying with all applicable laws and regulations and obtaining proper authorization before scanning any systems.
