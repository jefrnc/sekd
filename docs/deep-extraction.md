# Deep Extraction — Developer Notes

This document covers the **internals** of `sekd report TICKER --deep`: how the prompt is structured, how the cache key works, when to bump the prompt version, and how to extend the extraction schema. For the user-facing description of the feature, see the [README](../README.md#deep-analysis---deep).

## Goal of deep extraction

The base `sekd report` command already tells you **that** a ticker has a shelf registration and recent ATM activity — it counts S-3 and 424B filings from EDGAR's submissions feed. What it can't tell you without reading the actual filings is:

- The **dollar amount** of the shelf and how much has been drawn down
- The **remaining ATM capacity** under an active sales agreement
- **Warrants outstanding** with strike prices and expiration dates
- **Convertibles outstanding** with conversion prices and maturities

Parsing those fields out of an S-3 or 10-Q reliably with regex is a losing battle — the language varies by issuer and by counsel. An LLM is a much better tool for the job, but at a cost per call. Deep extraction solves this by:

1. Only running on a small set of highly relevant filings (most recent S-3, recent 424Bs, latest 10-Q)
2. Caching every per-filing result forever, keyed by accession number
3. Merging per-filing extracts into a single `DeepDilution` object that the builder stores on the report

## Filing selection

Handled by `buildDeep` in `internal/report/builder.go`:

| Filing type | How many | Why |
|---|---|---|
| `S-3`, `S-3/A` | Last 2 | Shelf ceiling and remaining capacity |
| `424B2`, `424B3`, `424B5` | Up to 5, last 180 days | Shelf drawdowns, ATM takedown activity |
| `10-Q` | Last 1 | Current warrant and convertible tables (MD&A + notes) |

Filings are sorted by filing date **descending** before extraction. This matters for the merge step.

## Extraction prompt

Defined in `internal/analysis/deep.go` as `deepPromptHeader`. The prompt is deliberately:

- **Per-filing, not per-ticker.** We send one filing at a time so the LLM's attention isn't split and the cache can key on a single accession number.
- **Strict schema, no prose.** The prompt asks for a specific JSON shape with numeric fields defaulting to 0 and lists defaulting to empty arrays. We parse the result into `DeepExtract` in Go.
- **No-hallucination guidance.** The prompt explicitly instructs the model to return 0 or empty for anything not stated in the filing, and to exclude historical warrants that have already been exercised or expired.
- **Guidance lines per field**, so the model knows the difference between shelf_total, shelf_used, and ATM capacity.

Text truncation: the cleaned filing text is truncated to **40 000 characters** before being prepended to the prompt. This is an empirical limit that fits a 10-Q's warrant section for most small-caps. If you find it cutting off relevant content, raise it carefully — token cost scales linearly.

## Cache keys and versioning

Cache key format:

```
deep:<ACCESSION_NUMBER>:<DEEP_PROMPT_VERSION>
```

- **`DEEP_PROMPT_VERSION`** is a constant at the top of `deep.go`. It currently reads `v1`.
- **Filings are immutable once filed on EDGAR**, so for a given accession the correct extract depends only on the prompt that produced it.
- Cache entries are written as JSON-serialized `DeepExtract` structs and live under `~/.sekd/cache/` with a TTL of 10 years — effectively forever.
- When a cache miss happens, the LLM is called once, the result is parsed, and if parsing succeeds, it's written to the cache before returning.

### When to bump the prompt version

**Bump `DeepPromptVersion` to `v2`, `v3`, etc. any time you change:**

- The text of `deepPromptHeader` in any way that could alter the output (adding a field, reordering guidance, tightening hallucination instructions)
- The shape of the `DeepExtract` JSON schema
- The text truncation length (since truncation changes what the model sees)

**Do not bump the version for:**

- Changes to `MergeDeep` or `MarkInTheMoney` — those run in Go on the cached output, so fixing a merge bug doesn't require re-hitting the LLM
- Changes to risk flag thresholds — those live in `EvaluateDeepRiskFlags` and also run on cached data
- Renames in Go structs that don't change the JSON tags

Bumping the version silently invalidates every cached result, so the next `--deep` run on each ticker will incur fresh LLM calls. Budget accordingly.

## Merge semantics

`MergeDeep` in `deep.go` consolidates multiple `DeepExtract`s (one per filing) into a single `DeepDilution` attached to the report. The rules:

1. **First-non-zero wins for scalar fields** (`ShelfTotalUSD`, `ShelfUsedUSD`, `ShelfRemainingUSD`, `ATMCapacityRemainingUSD`). Because the input is sorted newest-first, this means the most recent filing's numbers take precedence — exactly what you want, because an S-3 filed last week supersedes one from two years ago.
2. **Warrants and convertibles are union-with-dedupe.** The dedupe key is `strike|shares|expiration` for warrants and `conversion_price|principal|maturity` for convertibles. This prevents counting the same warrant tranche twice when it appears in both an S-3 and a 10-Q.
3. **Rows where both strike and shares are zero are dropped.** LLMs occasionally emit empty placeholder rows when they can't find anything; this step cleans them out.
4. **`ShelfRemainingUSD` is derived** if the LLM didn't give it to us directly: `total - used`, clamped to 0.

After the merge, `MarkInTheMoney(currentPrice)` is called on the result. It walks the warrants and convertibles, flips the `InTheMoney` bool when `strike <= currentPrice`, and populates `ITMWarrantShares` as the sum of shares across all ITM warrants. This number is what the `Warrants In The Money` risk flag reads.

## Risk flags that depend on deep data

These only fire when `--deep` was used. Defined in `EvaluateDeepRiskFlags` in `internal/analysis/risk.go`:

| Flag | Trigger | Severity |
|---|---|---|
| `Warrants In The Money` | `ITMWarrantShares > 0` | HIGH if `ITMWarrantShares / FloatShares >= 0.05`, else MEDIUM |
| `Large Shelf Capacity Remaining` | `ShelfRemainingUSD / MarketCap >= 0.25` | HIGH at `>= 0.5`, MEDIUM between `0.25` and `0.5` |

These thresholds are deliberate but tunable. If you change them, update the thresholds **and** the tests in `deep_test.go` (`TestEvaluateDeepRiskFlags_*`).

## Extending the schema

If you want to extract a new field — say, "preferred stock outstanding" — here's the full checklist:

1. Add the field to `DeepExtract` and `DeepDilution` in `deep.go`, with the matching JSON tags.
2. Update `deepPromptHeader` to ask for the new field and describe what it means.
3. **Bump `DeepPromptVersion`** (see above).
4. Update `MergeDeep` to combine the field across filings if it's a scalar, or to dedupe if it's a list.
5. If the new field should drive a risk flag, add a new `Flag*` constant in `analysis/types.go` and wire it into `EvaluateDeepRiskFlags`.
6. Update the renderers in `internal/report/terminal.go` and `internal/report/markdown.go` to show the field (JSON renderer gets it for free).
7. Add tests: at least a `parseDeepExtract` case with the new field present, and a `MergeDeep` case to verify combination logic.
8. Update the README's "Deep Dilution Detail" section.

## Cost estimate

A rough cost model assuming `gpt-4o-mini`, which is the default:

- ~15k-30k input tokens per filing (depending on length and truncation)
- ~500 output tokens per filing
- ~3 filings per ticker on first run

With `gpt-4o-mini` pricing this works out to **well under a cent per ticker** on the first run. Subsequent runs against the same filings cost **zero** because of the cache. Running `--deep` on a 20-ticker watchlist where you've already run it once costs nothing.

## Testing

Pure functions are tested in `internal/analysis/deep_test.go`:

- `parseDeepExtract` — clean JSON, JSON wrapped in prose/fences, invalid input
- `MergeDeep` — first-non-zero wins, dedupe, derivation of remaining, drop of empty rows, nil input
- `MarkInTheMoney` — strike at/below/above price, zero price, nil receiver
- `EvaluateDeepRiskFlags` — all severity branches, zero/missing inputs

`AnalyzeDeepFiling` itself is not unit-tested because it performs network I/O to the LLM. To test it manually:

```bash
export OPENAI_API_KEY=sk-proj-...
./sekd report SOUN --deep   # first run — hits the LLM
./sekd report SOUN --deep   # second run — cache-only, instant
```

The stderr log will tell you how many extractions came from cache vs fresh LLM calls.

If you want to add proper unit tests for `AnalyzeDeepFiling`, factor `CallLLMJSON` behind an interface in `analysis/ai.go` and inject a fake in the test. That refactor is out of scope today but is the right next step if the feature grows.
