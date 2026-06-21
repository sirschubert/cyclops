# Output Formats

Cyclops supports multiple output formats for different use cases: text (human-readable), JSON (machine-readable), HTML (report), and Markdown. The format can be specified with `-format` or auto-detected from the `-o` filename extension.

## Text (default)

```
╔══════════════════════════════════════╗
║       Cyclops Scan Report            ║
╚══════════════════════════════════════╝

  Target     : example.com
  Mode       : normal
  Scan Time  : 2026-04-24 08:59:30 UTC
  Subdomains : 1
  Live Hosts : 1
  Endpoints  : 1

┌─ [SUBDOMAIN] www.example.com
│   IP      : 93.184.216.34
│   Sources : mixed
│
└── [HOST] https://www.example.com  [200]
      Title  : Example Domain
      Server : nginx
      Tech   : Nginx, WordPress
      Size   : 1256 bytes
      ↳ https://www.example.com/robots.txt  [200]  (robots)
```

## JSON

Use `-format json` to output machine-readable JSON:

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
          "status_code": 200,
          "title": "Example Domain",
          "server": "nginx",
          "tech": ["Nginx", "WordPress"],
          "content_length": 1256,
          "endpoints": [
            { "url": "https://www.example.com/robots.txt", "status_code": 200, "source": "robots" }
          ]
        }
      ]
    }
  ]
}
```

## HTML and Markdown

HTML (`-format html -o report.html`) and Markdown (`-format md -o report.md`) formats are also supported for detailed report generation.

---

[Home](Home)
