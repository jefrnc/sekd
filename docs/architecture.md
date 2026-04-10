# Architecture

This document explains **how data flows** through sekd when you run a command. For the user-facing "what does each feature do" breakdown, see the [README](../README.md). For the package-by-package responsibility list, see [CONTRIBUTING.md](../CONTRIBUTING.md#project-structure).

## Request lifecycle — `sekd report TICKER`

```
┌──────────┐   cobra     ┌───────────────┐
│  cmd/    │ ──────────▶ │ report.Builder│
│ report.go│             └───────┬───────┘
└──────────┘                     │
                                 │ fan-out (errgroup)
           ┌────────────┬────────┴────────┬────────────────┐
           ▼            ▼                 ▼                ▼
      ┌─────────┐  ┌──────────┐     ┌──────────┐     ┌──────────┐
      │ EDGAR   │  │ EDGAR    │     │ EDGAR    │     │ Finviz   │
      │ subs    │  │ shares   │     │ authzd   │     │ quote    │
      │         │  │ XBRL     │     │ XBRL     │     │          │
      └────┬────┘  └────┬─────┘     └────┬─────┘     └────┬─────┘
           └────────────┴─────────────────┴────────────────┘
                                │
                                ▼
                     ┌─────────────────────┐
                     │ analysis package    │
                     │                     │
                     │ • AnalyzeDilution   │
                     │ • EvaluateRiskFlags │
                     │ • CalculateScore    │
                     └──────────┬──────────┘
                                │
                                ▼
                     ┌─────────────────────┐
                     │ analysis.Report     │
                     └──────────┬──────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          ▼                     ▼                     ▼
    ┌───────────┐         ┌───────────┐         ┌───────────┐
    │ terminal  │         │ json      │         │ markdown  │
    │ renderer  │         │ renderer  │         │ renderer  │
    └───────────┘         └───────────┘         └───────────┘
```

**Key properties:**

- **Fan-out is concurrent, merge is sequential.** All four data-source calls run in parallel via `golang.org/x/sync/errgroup`. The builder blocks on the whole group and then runs analysis synchronously.
- **Every HTTP call goes through `internal/cache`.** The cache is keyed by URL hash and stored in `~/.sekd/cache/`. TTLs are set by each caller: EDGAR ticker list and filing documents use 7 days, EDGAR submissions and XBRL company facts use 24 hours, Finviz quotes use 15 minutes, deep extraction uses effectively forever (10 years). Rate-limit-friendly by default and bypassable with `--no-cache`.
- **The builder is the only piece that knows about all the sources.** Everything downstream (analysis, renderers) operates only on the neutral `analysis.Report` struct. This is why adding a new risk flag or a new renderer is a local change.

## Deep extraction flow — `--deep`

Deep mode is a side-path off the main builder. It runs **after** the base report is built, not in parallel:

```
          (base report already built)
                    │
                    ▼
          ┌──────────────────┐
          │ buildDeep()      │
          └────────┬─────────┘
                   │ fetch filings
                   ▼
      ┌──────────────────────────┐
      │ S-3 × 2, 424B × N (≤180d)│
      │ + latest 10-Q            │
      └──────────┬───────────────┘
                 │ sort by filing_date desc
                 ▼
      ┌──────────────────────────┐
      │ for each filing:         │
      │   cache lookup           │
      │   (deep:ACCESSION:v1)    │
      │     │                    │
      │     ├─ HIT  → reuse      │
      │     └─ MISS → LLM call   │
      │              + cache.Set │
      └──────────┬───────────────┘
                 │ per-filing DeepExtract
                 ▼
      ┌──────────────────────────┐
      │ MergeDeep                │
      │ • first-non-zero wins    │
      │ • dedupe warrants        │
      │ • derive shelf_remaining │
      └──────────┬───────────────┘
                 │
                 ▼
      ┌──────────────────────────┐
      │ MarkInTheMoney(price)    │
      └──────────┬───────────────┘
                 │
                 ▼
      ┌──────────────────────────┐
      │ EvaluateDeepRiskFlags    │
      │ (WarrantsITM, ShelfCap)  │
      └──────────┬───────────────┘
                 │
                 ▼
       analysis.Report.Deep (optional field)
```

**Why the cache key is `deep:ACCESSION:PROMPT_VERSION`:**

- **Accession** — SEC filings are immutable once filed, so the same accession number always extracts to the same data.
- **Prompt version** — if we change the prompt text in `deep.go`, cached results are silently ignored and regenerated because the key no longer matches. No manual cache clearing required.

See [`docs/deep-extraction.md`](deep-extraction.md) for the prompt design and versioning rules.

## Interactive TUI flow

The TUI is a standard bubbletea `Model` + `Update` loop living in `internal/tui/`. Key things to know:

- **Commands are a flat switch.** `model.handleCommand` dispatches `/foo` to a handler. The registry in `commands.go` is used **only** for the palette and the help screen — it intentionally does not drive dispatch, to keep the hot path simple.
- **Long-running work returns `tea.Msg`s.** Any handler that hits the network (reports, filings, analysis, watchlist scan) returns a `tea.Cmd` that produces a `*DoneMsg` when finished. The top of `Update` is one big switch over message types.
- **Slash palette lives in `View`, not `Update`.** Every frame, if the current input starts with `/`, `renderSlashPalette` filters the registry and draws matches below the input. Tab handling happens in `handleTab`, which cycles through the same matcher.
- **Watchlist scan is sequential on purpose.** For typical watchlist sizes (≤20 tickers) the UX cost is acceptable and we avoid hammering SEC EDGAR past their 10 req/s limit. Parallelism is a future knob, not a current requirement.

## Where state lives

| State | Location | Format |
|---|---|---|
| HTTP / LLM cache | `~/.sekd/cache/<sha256>` | raw bytes |
| Config (API keys, models) | `~/.sekd/config.json` | JSON, `0600` perms |
| Watchlist + snapshots | `~/.sekd/watchlist.json` | JSON |
| Command / ticker history | `~/.sekd/history/<session>.jsonl` | JSONL |
| Session (last ticker, last CIK) | `~/.sekd/session.json` | JSON |

Nothing lives in environment variables except API keys (which sekd copies from the config into the process env on startup, so downstream code reads them uniformly via `os.Getenv`).

## Adding a new data source

1. Create `internal/<source>/` with a `Client` that accepts `*cache.Cache` in its constructor.
2. Add a call to it in `builder.Build`'s errgroup so it runs in parallel with the others.
3. Surface whatever it returns as a new field on `analysis.Report`.
4. If it triggers risk signals, add a new `Flag*` constant in `analysis/types.go` and wire it into `EvaluateRiskFlags` (or `EvaluateDeepRiskFlags` if it only fires under `--deep`).
5. Update the three renderers (`terminal`, `markdown`, `json` gets it for free via struct tags).

That's the whole pipeline. The builder is deliberately the single widening point — everything upstream is narrow and concurrent, everything downstream reads from the neutral report struct.
