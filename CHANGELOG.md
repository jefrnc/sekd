# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-04-10

### Changed
- Homebrew packaging migrated from `brews` (deprecated in GoReleaser v2.10) to `homebrew_casks`. The tap repository `jefrnc/homebrew-sekd` now ships `Casks/sekd.rb` instead of `Formula/sekd.rb`. Existing installations are migrated transparently via `tap_migrations.json` in the tap repo — users running `brew upgrade` do not need to re-tap or re-install.
- The Cask includes a post-install hook that strips the `com.apple.quarantine` xattr so the unsigned macOS binary can run without manual Gatekeeper intervention.

## [0.2.0] - 2026-04-10

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

### Fixed
- `sekd report --no-cache` now actually bypasses the cache. The flag was declared but never wired through; it is now backed by a `bypass` field on `cache.Cache` that makes `Get` always miss while still allowing `Set` to warm the cache for subsequent non-bypass runs.
- **Finviz quotes are now actually cached.** The `Scraper` accepted a `*cache.Cache` in its constructor but ignored it on the hot path, so every report call re-fetched Finviz even within the same second. Quotes are now cached for 15 minutes and honor the `--no-cache` bypass flag.
- **Corrupt `watchlist.json` no longer destroys user data.** Previously, if the file failed to parse, `Load` would silently return an empty list and the next `Save` would overwrite the original — losing every watched ticker. Corrupt files are now renamed to `watchlist.json.broken-<timestamp>`, the load returns an error the caller can surface, and `Save` refuses to write until the path is valid.
- **Corrupt `config.json` no longer silently discards the user's API keys.** `config.Load` now returns a descriptive error instead of swallowing the JSON parse failure, and `cmd/root.go` prints a loud warning on startup so the user knows why their keys aren't being read.

### Tests
- New `finviz` package tests covering cache reuse, bypass-mode forced re-fetch, and upstream error handling.
- New `watchlist` regression test locking in the corrupt-file backup behavior.
- New `config` regression test locking in the corrupt-file error path.
- New `cache` tests for the bypass mode (read misses while writes still warm the cache).

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

[0.2.1]: https://github.com/jefrnc/sekd/releases/tag/v0.2.1
[0.2.0]: https://github.com/jefrnc/sekd/releases/tag/v0.2.0
[0.1.0]: https://github.com/jefrnc/sekd/releases/tag/v0.1.0
