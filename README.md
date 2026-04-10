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
- **Risk Scoring** — Automated flags for high dilution, recent ATM offerings, low float, high short interest, warrants-in-the-money, large shelf capacity
- **Filing Reader** — Download and read actual SEC filing documents as clean text
- **AI Analysis** (optional) — Analyze filings with OpenAI or Anthropic to extract offering amounts, warrants, red flags
- **Deep Dilution Extraction** (optional) — `--deep` uses an LLM to pull structured shelf totals, ATM capacity, warrants (with ITM detection) and convertibles out of the actual S-3 / 424B / 10-Q filings, with aggressive caching so repeat runs are free
- **Watchlist Scan** — Track a set of tickers and re-scan them on demand to see which ones have new filings, score drops, or new risk flags since your last check

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

# Deep report — LLM extracts shelf/ATM/warrants/convertibles from filings
sekd report SOUN --deep
sekd report MARA --deep --md > mara-deep.md

# Browse and read SEC filings
sekd filings SOUN
sekd filings SOUN --type S-3
sekd filings SOUN --type 424B5 --read 0
sekd filings SOUN --read 0 --analyze
```

### Deep analysis (`--deep`)

`--deep` is an opt-in flag that pulls **structured dilution data** out of the actual filings using an LLM. Without the flag you get the normal EDGAR + Finviz report. With the flag you also get a **Deep Dilution Detail** section in the output showing:

- **Shelf Total / Used / Remaining** — the aggregate dollar ceiling of the current S-3 and how much has been taken down
- **ATM Capacity Remaining** — dollars still available under any active At-The-Market sales agreement
- **Warrants** — outstanding warrants with strike, underlying shares, expiration, and an **ITM** flag when the strike is at or below the current market price
- **Convertibles** — outstanding convertible notes / preferred / debentures with conversion price, principal, maturity, and the same ITM flag

Two risk flags only fire when `--deep` is used, because they need the extracted data:

| Flag | Trigger |
|------|---------|
| **Warrants In The Money** | Sum of ITM warrant shares > 0. HIGH if ≥5% of float, MEDIUM otherwise |
| **Large Shelf Capacity Remaining** | Shelf remaining ≥25% of market cap (MEDIUM) or ≥50% (HIGH) |

**Requires an AI provider configured** — see [AI analysis](#ai-analysis-optional) below. Without a key, `--deep` prints a warning and the rest of the report renders normally.

**Caching**: the first `--deep` run against a ticker hits the LLM once per relevant filing (typically 3: the active S-3, the last 10-Q, and recent 424B takedowns). Results are cached by accession number + prompt version in `~/.sekd/cache/`, so subsequent runs are instant and cost nothing. The cache auto-invalidates if the prompt version is bumped in a future release.

## Interactive commands

Start typing `/` to get a **live command palette** below the input — it filters as you type and `Tab` cycles through matches.

| Command | Description |
|---------|-------------|
| `TICKER` | Run full due diligence report |
| `/compare T1 T2` | Side-by-side comparison of two tickers |
| `/last` | Re-run the last ticker |
| `/filings TICKER [type]` | List SEC filings, optionally filtered by form type |
| `/read N` | Read filing at index N from the last `/filings` list |
| `/analyze N` | Analyze filing N with AI |
| `/watchlist` | Show watchlist |
| `/watchlist add TICKER [note]` | Add a ticker to the watchlist |
| `/watchlist remove TICKER` | Remove a ticker from the watchlist |
| `/watchlist scan` | **Re-scan all watched tickers and show deltas since last scan** |
| `/history` | Show command history |
| `/recent` | Show recently-queried tickers |
| `/session` | Show current session info |
| `/export [path]` | Export last output to a file |
| `/copy` | Copy last output to clipboard |
| `/config` | Show current configuration |
| `/config set KEY VAL` | Set a config value |
| `/config clear KEY` | Remove a config value |
| `/json` | Toggle JSON output |
| `/md` | Toggle Markdown output |
| `/clear` | Clear the screen |
| `/help` | Show all commands |
| `/quit` | Exit |

### Watchlist scan

The watchlist isn't just a bookmark list — `/watchlist scan` is what makes it actually useful. It rebuilds a fresh report for every ticker you track, compares each one against the snapshot it saved on the previous scan, and surfaces **only the ones that changed**:

```
─── Watchlist Scan (12 tickers) ───

  ⚠ SOUN   D 55 (-8 vs last)
      → new filing: 424B5 on 2025-02-14
      + new flags: Warrants In The Money
  ⚠ MARA   C 62 (-13 vs last)
      + new flags: High Dilution
  🔴 PRSO  F 32 (-24 vs last)
      → new filing: S-3 on 2025-02-10
      + new flags: Recent ATM, Large Shelf Capacity Remaining
  • TSLA   baseline: A 95 — first scan, snapshot saved

  ✓ 8 unchanged

  Snapshots updated. Type a flagged ticker to see full DD, or /filings TICKER.
```

**What a delta means**:
- **`→ new filing`** — a filing appeared that wasn't there the last time you scanned. sekd only remembers the latest accession number per ticker, so this fires the first time any new filing of any type shows up.
- **`+ new flags`** — a risk flag is now active that wasn't active on the previous scan (e.g. a new ATM takedown triggered `Recent ATM`).
- **`− cleared flags`** — a flag that used to be active no longer triggers.
- **Score delta** — if your DD Score moved vs the last snapshot it's shown as `(+3)` or `(-8)`; drops of 15+ points get the 🔴 marker.

Snapshots are stored in `~/.sekd/watchlist.json` next to the ticker entries, so deltas persist across sessions and machines if you sync that file.

**Pairing with `--deep`**: scan today uses the normal fast path (no LLM). If you want the full structured dilution data across your watchlist, you can run `sekd report TICKER --deep` on each flagged ticker after a scan — the deep cache amortizes cost across the whole list since filings are shared.

## Configuration

### AI analysis (optional)

AI analysis powers two optional features:

1. **`/analyze N`** — one-shot LLM summary of a specific filing (offering amount, warrants, red flags)
2. **`sekd report TICKER --deep`** — structured extraction of shelf / ATM / warrants / convertibles across multiple filings

Both require an API key from OpenAI or Anthropic. Without a key, `sekd` works fully — you just won't get these two features.

**Option A — interactive `/config`** (after you've launched `sekd`):
```bash
# Inside interactive mode
/config set openai-key sk-proj-...
/config set openai-model gpt-4o          # optional, default: gpt-4o-mini

# Or for Anthropic
/config set anthropic-key sk-ant-...
/config set anthropic-model claude-haiku-4-5-20251001   # optional
```

**Option B — write the config file directly** (useful for scripting / first-time setup without launching the TUI):
```bash
mkdir -p ~/.sekd && cat > ~/.sekd/config.json <<'EOF'
{"openai_key":"sk-proj-YOUR-KEY-HERE","openai_model":"gpt-4o-mini"}
EOF
chmod 600 ~/.sekd/config.json
```

The config file is stored at `~/.sekd/config.json` with restricted permissions (0600). sekd loads it on startup and copies the values into `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` in the process environment, so both the CLI and interactive mode pick it up automatically.

**Option C — environment variables or a `.env` file** (env vars take precedence over the config file):
```bash
export OPENAI_API_KEY=sk-proj-...
# or
export ANTHROPIC_API_KEY=sk-ant-...
```

**Which provider to pick**: for `--deep` either works fine. OpenAI's `gpt-4o-mini` is cheap and fast; Anthropic's `claude-haiku-4-5` is roughly comparable. A single `--deep` run costs fractions of a cent per ticker because of aggressive caching — after the first run, repeat scans of the same filings cost **nothing**.

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

| Flag | Severity | Trigger | Source |
|------|----------|---------|--------|
| High Dilution | HIGH | >20% share increase in 12 months | EDGAR XBRL |
| Recent ATM | HIGH | 424B filing in last 90 days | EDGAR |
| Massive Authorized | HIGH | Authorized/Outstanding ratio > 3x | EDGAR XBRL |
| Shelf Registration | MEDIUM | Active S-3 on file | EDGAR |
| High Short Interest | MEDIUM | Short float > 20% | Finviz |
| Low Float | MEDIUM | Float < 10M shares | Finviz |
| Warrants In The Money | HIGH (≥5% of float) / MEDIUM | Outstanding warrant strike ≤ current price | `--deep` (LLM) |
| Large Shelf Capacity Remaining | HIGH (≥50% mcap) / MEDIUM (≥25%) | Shelf dollars still available vs market cap | `--deep` (LLM) |

The last two flags **only appear when you run with `--deep`** because they need the LLM-extracted data to compute. All the others run on every `sekd report` call and cost nothing.

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
