# APOXY — API Proxy Scanner

Fork of [MINOXY](https://github.com/OGRYZOK-dev/MINOXY) — repurposed for finding and validating OpenAI-compatible API endpoints.

## Features

- Scrapes GitHub repos, pastebins, and URL lists for `/v1` API endpoints
- Validates `/v1/models` and `/v1/chat/completions`
- **Unlimited check** — tests if endpoint accepts requests without billing (sends request with invalid key, expects 401)
- **Rate limit detection** — burst-tests endpoint to detect 429 thresholds
- Export: JSON (machine-readable) + Hermes `custom_providers` YAML
- Beautiful TUI with Bubble Tea

## Quick Start

```bash
git clone https://github.com/nikita4a/APOXY.git
cd APOXY
go mod tidy
go build -o apoxy .
./apoxy
```

## Config (config.yaml)

```yaml
threads: 200          # concurrent workers
timeout: 8s           # per-endpoint timeout
check_models: true    # fetch /v1/models
export_path: exports/api_proxies.json
sources:
  - https://raw.githubusercontent.com/cheahjs/free-llm-api-resources/main/README.md
  # add your own URL lists
```

## Two Checks Explained

### 1. Unlimited/Free Check (`Unlimited` field)

Sends a POST to `/v1/chat/completions` with an intentionally invalid API key (`Bearer INVALID_KEY_APOXY_UNLIMITED_CHECK`):

| Response | Meaning |
|----------|---------|
| **401 Unauthorized** | Endpoint is open — anyone with a valid key can use it. Marked as `unlimited: true` |
| **403 Forbidden** | Might be gated but still accessible. Marked `unlimited: true` |
| **429 Too Many Requests** | Rate-limited even without a valid key. `unlimited: false` |
| **Other** | Endpoint is closed/restricted. `unlimited: false` |

### 2. Rate Limit Detection (`rate_limit_rpm` field)

Sends 10 rapid GET requests to `/v1/models` and counts 429 responses:

| Result | Meaning |
|--------|---------|
| **0 hits** | No rate limit detected → `rate_limit_rpm: 999` |
| **1-3 hits** | Low rate limit → use with caution |
| **4-9 hits** | Aggressive rate limiting → not recommended for heavy use |
| **10 hits** | Extremely limited → practically unusable |

## Key Bindings

| Key | Action |
|-----|--------|
| Up/Down | Navigate menu |
| Enter | Start scan / Rescan |
| Space | Pause/Resume during scan |
| Esc | Stop scan |
| Y | Export results as JSON |
| H | Export as Hermes YAML config |
| Q | Quit |

## Export Formats

### JSON (`exports/api_proxies.json`)
```json
{
  "generated": "2026-06-28T12:00:00Z",
  "alive_count": 15,
  "total_models": 342,
  "proxies": [
    {
      "url": "https://api.example.com/v1",
      "alive": true,
      "latency_ms": 45000000,
      "models": ["model-a", "model-b"],
      "models_count": 2,
      "unlimited": true,
      "rate_limit_rpm": 999
    }
  ]
}
```

### Hermes YAML (`exports/hermes_providers.yaml`)
```yaml
custom_providers:
  - name: apoxy-found-0
    base_url: https://api.example.com/v1
    provider: openai
    api_key: ''
    discover_models: false
    models:
      model-a:
        ctx: 131072
      model-b:
        ctx: 131072
```

## Architecture

```
main.go              # Entry point
config/config.go     # YAML config loader
proxy/
  apicheck.go        # /v1/models + /v1/chat validation + unlimited + rate limit
  scraper.go         # URL extraction from markdown/text sources
  runner.go          # Worker pool (configurable threads)
  exporter.go        # JSON + Hermes YAML export
tui/
  tui.go             # Bubble Tea TUI
  styles.go          # Lipgloss styles
```

## Credits

- [MINOXY](https://github.com/OGRYZOK-dev/MINOXY) — original proxy checker that inspired this fork
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework

## License

MIT
