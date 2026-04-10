# Contributing to sekd

Thanks for your interest in contributing to sekd.

## Getting started

1. Fork the repo
2. Clone your fork: `git clone https://github.com/YOUR_USER/sekd.git`
3. Create a branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `make test`
6. Run linter: `go vet ./...`
7. Commit and push
8. Open a Pull Request

## Development setup

```bash
# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run the tool
./sekd
```

## Code style

- Run `go fmt ./...` before committing
- Follow standard Go conventions
- Use table-driven tests
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Keep packages in `internal/` — this is a CLI, not a library

## Project structure

```
cmd/            CLI commands (cobra): report, filings, version, interactive
internal/
  analysis/     Dilution analysis, risk flags, scoring, AI integration,
                deep LLM extraction (shelf/warrants/convertibles)
  cache/        File-based HTTP and LLM response cache (~/.sekd/cache/)
  clipboard/    Cross-platform clipboard copy
  config/       Configuration management (~/.sekd/config.json)
  edgar/        SEC EDGAR API client (submissions, XBRL, documents)
  finviz/       Finviz market data scraper
  history/      Per-session command and ticker history
  notify/       Desktop notifications (macOS/Linux)
  report/       Report builder and renderers (terminal, JSON, markdown)
  session/      Session persistence
  tui/          Interactive terminal UI (bubbletea), including the
                slash command registry/palette
  watchlist/    Watchlist storage + snapshot/diff tracking for scans
```

## Adding a new data source

1. Create a package in `internal/`
2. Add a client struct with constructor accepting `*cache.Cache`
3. Use `context.Context` for all HTTP calls
4. Add the data to the report builder in `internal/report/builder.go`
5. Write tests

## Adding a new command

1. Create a file in `cmd/`
2. Define a `cobra.Command`
3. Register it in `init()` with `rootCmd.AddCommand()`

## Testing

- Tests must pass: `go test -race ./internal/...`
- Use table-driven tests for input/output functions
- Use `t.TempDir()` for tests that need filesystem access
- No network calls in unit tests — use test fixtures or mock data

## Pull Request guidelines

- Keep PRs focused on a single change
- Include tests for new functionality
- Update README if adding user-facing features
- Update `CHANGELOG.md` under the `[Unreleased]` section
- Run `go vet ./...` and fix all warnings
- Do not commit binaries, cached data, or API keys — the `.gitignore` covers the common cases

## Further reading

- [`docs/architecture.md`](docs/architecture.md) — data flow diagrams for the normal report and deep extraction paths, plus where on-disk state lives
- [`docs/deep-extraction.md`](docs/deep-extraction.md) — internals of `--deep`, including the prompt versioning rules you should read **before** touching `internal/analysis/deep.go`

## Working on LLM-dependent features

Two features rely on an AI provider (OpenAI or Anthropic): `/analyze` and `sekd report --deep`. When developing against them:

- Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` in your shell, or use `sekd /config set openai-key ...`
- The deep extractor caches by accession number + prompt version under `~/.sekd/cache/`. If you change the prompt text, **bump `DeepPromptVersion` in `internal/analysis/deep.go`** to invalidate old cached results.
- Avoid making real LLM calls in tests — `AnalyzeDeepFiling` is integration-tested manually; unit tests should target the pure helpers (`MergeDeep`, `MarkInTheMoney`, `parseDeepExtract`, `EvaluateDeepRiskFlags`).

## Reporting issues

Open an issue on GitHub with:
- What you expected
- What happened
- Steps to reproduce
- `sekd version` output
