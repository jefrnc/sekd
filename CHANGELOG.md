# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `sekd report TICKER --deep` flag that extracts structured shelf/ATM/warrant/convertible data from recent S-3, 424B, and 10-Q filings via an LLM (OpenAI or Anthropic). Results are cached per accession number + prompt version so repeat runs cost nothing.
- Two new risk flags gated on `--deep`:
  - **Warrants In The Money** — HIGH when ITM warrant shares exceed 5% of float, MEDIUM otherwise
  - **Large Shelf Capacity Remaining** — HIGH when shelf remaining is ≥50% of market cap, MEDIUM at ≥25%
- `/watchlist scan` interactive command — rebuilds every watched ticker, compares against a stored per-entry snapshot, and surfaces only the tickers with deltas: new filings, score drops, added or cleared risk flags.
- Snapshot fields on `watchlist.Entry` (`LastScore`, `LastGrade`, `LastFlags`, `LastAccession`, `LastFilingDate`, `LastScannedAt`) — backwards compatible, zero values on existing entries.
- Live slash command palette in interactive mode — typing `/` shows a filtered dropdown of commands below the input, with Tab to cycle through matches. Registry (`internal/tui/commands.go`) is the single source of truth shared with `/help` so they can't drift apart.
- `LatestAccession`, `LatestFilingDate`, `LatestFilingForm` fields on `analysis.Report`, populated from EDGAR submissions so watchlist scan can detect new filings without re-fetching.
- `CallLLMJSON` helper in `internal/analysis` — factored out so multiple features (filing analysis, deep extraction) share the same OpenAI/Anthropic plumbing.
- Tests: 13 new tests in `internal/analysis/deep_test.go`, 10 new tests in `internal/tui/commands_test.go`, 2 new watchlist snapshot tests.

### Changed
- `report.Builder` now takes an optional `BuildOptions{Deep bool}` via `BuildWithOptions`. `Build` remains for backwards compat.
- `renderHelp` rewritten to read from the slash command registry.
- Terminal and markdown renderers now show `—` instead of `$0.0000` for missing strike/conversion prices in warrant and convertible tables.

## [0.1.0] - 2026-04-03

### Added
- Interactive terminal mode with bubbletea UI
- Full due diligence reports from SEC EDGAR + Finviz
- Dilution analysis: shares outstanding trend, ATM/shelf detection
- Risk scoring system with 6 flag types and letter grades
- SEC filing browser with arrow-key navigation
- Filing document reader (HTML to clean text)
- AI-powered filing analysis (OpenAI and Anthropic)
- Configuration management (`/config set`, `/config clear`)
- JSON and Markdown output modes
- File-based HTTP cache
- CLI commands: `report`, `filings`, `version`
- Homebrew tap distribution

[0.1.0]: https://github.com/jefrnc/sekd/releases/tag/v0.1.0
