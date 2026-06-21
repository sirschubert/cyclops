# Advanced Features

## Checkpoint & Resume

The checkpoint feature allows you to save progress after each phase and resume from where you left off if the scan is interrupted.

Use `-checkpoint-dir` to specify a directory where checkpoint files will be saved after each phase (subdomain enumeration, host discovery, endpoint crawling):

```bash
cyclops -d example.com -checkpoint-dir ./checkpoints
```

If the scan is interrupted or times out, resume it with `-resume`:

```bash
cyclops -d example.com -resume ./checkpoints/example.com.cyclops-checkpoint.json
```

This loads the previous result and skips any completed phases, continuing from where it left off.

## Silent Mode

The `-silent` flag suppresses the ASCII art banner and progress spinners, outputting only the results. This is ideal for piping output to other tools or integrating Cyclops into scripts:

```bash
cyclops -d example.com -silent -format json | jq '.subdomains[].subdomain'
```

## Metadata SSRF Guard

The `-block-metadata` flag prevents connections and redirects to cloud-metadata and link-local addresses, mitigating SSRF attacks. It blocks:

- **IPv4**: `169.254.0.0/16` (AWS EC2 metadata, link-local)
- **IPv6**: Link-local addresses (fe80::/10)

Blocking is enforced even when a hostname resolves into these ranges:

```bash
cyclops -d example.com -block-metadata
```

## Rate Auto-Tuning

The `-autotune` flag dynamically adjusts the request rate based on server responses. If the server returns 429 (Too Many Requests) or 503 (Service Unavailable), the rate is automatically lowered. If responses are successful, the rate may gradually increase:

```bash
cyclops -d example.com -autotune -v
```

Use `-v` (verbose) to see rate adjustments in real-time.

---

[Home](Home)
