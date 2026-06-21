# Options Reference

## Flags

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
| `-format` | Output format: `text`, `json`, `html`, `md` | `text` |
| `-o` | Output file (default: stdout) | — |
| `-proxy` | HTTP proxy URL | — |
| `-timeout` | Per-request timeout in seconds | 10 |
| `-scan-timeout` | Total scan timeout in minutes (0 = no limit) | 30 |
| `-user-agent` | Custom User-Agent string | `Cyclops/1.1` |
| `-v` | Verbose output | false |
| `-extend` | Enable directory bruteforcing after crawl | false |
| `-wordlist-dir` | Custom directory wordlist path | — |
| `-insecure` | Disable TLS certificate verification | false |
| `-no-color` | Disable colored output | false |
| `-silent` | Suppress banner and progress output (only emit results — ideal for piping) | false |
| `-block-metadata` | Block connections/redirects to cloud-metadata & link-local addresses (`169.254.0.0/16`, IPv6 link-local) | false |
| `-checkpoint-dir` | Directory to write per-phase checkpoint files for resuming | — |
| `-resume` | Resume a scan from a checkpoint file | — |
| `-version` | Print version and exit | — |

## Scan Modes

Three built-in modes tune concurrency, rate, and stealth behaviour simultaneously:

| Mode | Workers | Rate | Depth | Extras |
|------|---------|------|-------|--------|
| `normal` (default) | 50 | 500 req/s | 2 | — |
| `stealth` | 5 | 10 req/s | 2 | passive-only, 1–4 s random delay, browser UA rotation |
| `aggressive` | 200 | 2000 req/s | 4 | all sources enabled |

Explicit `-t`, `-r`, or `-depth` flags always override mode defaults.

## Interactive Mode

The `-interactive` flag pauses between each phase so you can review results and pick which targets to continue with. This is useful for large scopes where you want to focus on specific subdomains or hosts before crawling.

## Output Format Auto-Detection

If you pass `-o` with a recognised extension (`.json`, `.html`, `.md`, `.txt`) and don't set `-format`, the format is inferred from the filename.

---

[Home](Home)
