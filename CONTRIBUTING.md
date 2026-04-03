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
cmd/            CLI commands (cobra)
internal/
  analysis/     Dilution analysis, risk flags, scoring, AI integration
  cache/        File-based HTTP response cache
  config/       Configuration management (~/.sekd/config.json)
  edgar/        SEC EDGAR API client (submissions, XBRL, documents)
  finviz/       Finviz market data scraper
  report/       Report renderers (terminal, JSON, markdown)
  tui/          Interactive terminal UI (bubbletea)
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
- Run `go vet ./...` and fix all warnings

## Reporting issues

Open an issue on GitHub with:
- What you expected
- What happened
- Steps to reproduce
- `sekd version` output
