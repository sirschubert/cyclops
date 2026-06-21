# Usage

## Basic scan

```bash
cyclops -d example.com
```

## Stealth scan

Passive sources only, slow and quiet.

```bash
cyclops -d example.com -mode stealth -format json -o results.json
```

## Aggressive scan

Maximum speed.

```bash
cyclops -d example.com -mode aggressive -format html -o report.html
```

## Auto-tune the rate limiter

Dynamically adjust request rate based on server responses.

```bash
cyclops -d example.com -autotune -v
```

## Interactive mode

Pause between each phase to review and select targets.

```bash
cyclops -d example.com -interactive
```

## Directory bruteforce scan

```bash
cyclops -d example.com -extend -wordlist-dir /path/to/directories.txt -format json -o results.json
```

## Disable TLS verification

For self-signed certificates.

```bash
cyclops -d example.com -insecure -v
```

## Disable colored output

```bash
cyclops -d example.com -no-color -o results.txt
```

## Save as HTML report

```bash
cyclops -d example.com -format html -o report.html
```

## Save as Markdown

```bash
cyclops -d example.com -format md -o report.md
```

## Save as JSON

```bash
cyclops -d example.com -format json -o results.json
```

## Custom wordlist and resolvers

```bash
cyclops -d example.com -w wordlists/custom.txt -resolvers "8.8.8.8,1.1.1.1,9.9.9.9"
```

## Through a proxy with verbose output

```bash
cyclops -d example.com -proxy http://127.0.0.1:8080 -v
```

## Silent mode

Clean output for piping.

```bash
cyclops -d example.com -silent -format json | jq '.subdomains[].subdomain'
```

## Resumable scan

Checkpoint after each phase, then resume.

```bash
cyclops -d example.com -checkpoint-dir ./checkpoints
# ... interrupted / timed out ...
cyclops -d example.com -resume ./checkpoints/example.com.cyclops-checkpoint.json
```

## Block SSRF-style metadata redirects

```bash
cyclops -d example.com -block-metadata
```

---

[Home](Home)
