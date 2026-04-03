# sekd

**SEC Decoded** — CLI tool for stock due diligence, filing analysis, and dilution risk detection using public SEC EDGAR data.

[![CI](https://github.com/jefrnc/sekd/actions/workflows/ci.yml/badge.svg)](https://github.com/jefrnc/sekd/actions/workflows/ci.yml)
[![Release](https://github.com/jefrnc/sekd/actions/workflows/release.yml/badge.svg)](https://github.com/jefrnc/sekd/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/jefrnc/sekd)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## What it does

Given a US stock ticker, `sekd` generates an automated due diligence report using **only public data**:

- **SEC EDGAR** — Filing history (S-3, 424B5, 8-K, 10-K), XBRL financial facts
- **Finviz** — Float, short interest, institutional ownership, sector/industry
- **Dilution Analysis** — Shares outstanding trend, authorized vs outstanding ratio, ATM/shelf detection
- **Risk Scoring** — Automated flags for high dilution, recent ATM offerings, low float, high short interest
- **Filing Reader** — Download and read actual SEC filing documents as clean text
- **AI Analysis** (optional) — Analyze filings with OpenAI or Anthropic to extract offering amounts, warrants, red flags

## Install

### Homebrew (macOS/Linux)

```bash
brew tap jefrnc/sekd
brew install sekd
```

### From source

```bash
go install github.com/jefrnc/sekd@latest
```

### Binary download

Grab the latest release from [GitHub Releases](https://github.com/jefrnc/sekd/releases).

## Quick start

### Interactive mode (default)

```bash
sekd
```

This opens an interactive terminal with:
- Type a ticker to run a full DD report
- Navigate filings with arrow keys
- Analyze filings with AI
- `/help` for all commands

### CLI mode

```bash
# Full due diligence report
sekd report SOUN
sekd report MARA --json
sekd report SOFI --md > sofi-dd.md

# Browse and read SEC filings
sekd filings SOUN
sekd filings SOUN --type S-3
sekd filings SOUN --type 424B5 --read 0
sekd filings SOUN --read 0 --analyze
```

## Interactive commands

| Command | Description |
|---------|-------------|
| `TICKER` | Run full due diligence report |
| `/filings TICKER` | List SEC filings |
| `/filings TICKER S-3` | Filter by form type |
| `/read N` | Read filing at index N |
| `/analyze N` | Analyze filing with AI |
| `/config` | Show current configuration |
| `/config set KEY VAL` | Set a config value |
| `/config clear KEY` | Remove a config value |
| `/json` | Toggle JSON output |
| `/md` | Toggle Markdown output |
| `/help` | Show all commands |
| `/quit` | Exit |

## Configuration

### AI analysis (optional)

AI analysis requires an API key from OpenAI or Anthropic. Without it, `sekd` works fully — you just won't get AI-powered filing analysis.

```bash
# Inside interactive mode
/config set openai-key sk-proj-...
/config set openai-model gpt-4o          # optional, default: gpt-4o-mini

# Or for Anthropic
/config set anthropic-key sk-ant-...
```

Configuration is stored in `~/.sekd/config.json` with restricted permissions (0600).

You can also use environment variables or a `.env` file:

```bash
OPENAI_API_KEY=sk-proj-...
# or
ANTHROPIC_API_KEY=sk-ant-...
```

### Config keys

| Key | Description | Default |
|-----|-------------|---------|
| `openai-key` | OpenAI API key | — |
| `openai-model` | OpenAI model | gpt-4o-mini |
| `anthropic-key` | Anthropic API key | — |
| `anthropic-model` | Anthropic model | claude-haiku-4-5 |

## Data sources

All data comes from **free, public APIs**. No paid subscriptions required.

| Source | What | Rate limit |
|--------|------|------------|
| [SEC EDGAR](https://www.sec.gov/edgar) | Filings, XBRL financial facts, company info | 10 req/sec |
| [Finviz](https://finviz.com) | Float, short interest, sector, market cap | Best effort |

Responses are cached locally in `~/.sekd/cache/` to avoid repeated API calls.

## Risk flags

`sekd` evaluates these risk signals:

| Flag | Severity | Trigger |
|------|----------|---------|
| High Dilution | HIGH | >20% share increase in 12 months |
| Recent ATM | HIGH | 424B filing in last 90 days |
| Massive Authorized | HIGH | Authorized/Outstanding ratio > 3x |
| Shelf Registration | MEDIUM | Active S-3 on file |
| High Short Interest | MEDIUM | Short float > 20% |
| Low Float | MEDIUM | Float < 10M shares |

## DD Score

Score starts at 100 and deducts points per risk flag:

| Grade | Score | Meaning |
|-------|-------|---------|
| A | 90-100 | Minimal dilution risk |
| B | 75-89 | Low risk |
| C | 60-74 | Moderate risk |
| D | 40-59 | High risk |
| F | 0-39 | Extreme dilution risk |

## Development

```bash
# Build
make build

# Run tests
make test

# Run linter
make lint

# Format code
make fmt
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Disclaimer

This tool is for **informational and educational purposes only**. It is not financial advice. Always do your own research before making investment decisions. The data comes from public sources and may contain errors or delays.

## License

[MIT](LICENSE)
