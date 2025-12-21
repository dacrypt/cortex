# LLM Benchmark Reports

This folder stores benchmark runs from `benchmark-llm-models.js`.

## Files
- `benchmark-results-<timestamp>.json`: full raw results, including per-test outputs.
- `benchmark-results-<timestamp>.csv`: summary table for quick comparison and plotting.

## How to Use
- Use the CSV to track speed, tokens/sec, and compliance across runs.
- Use the JSON if you need to inspect exact model outputs per test.

## Timestamp Format
`YYYY-MM-DD_HHMMSS`

## Recommended Baseline
Keep one known-good run and compare new runs against it for regressions.
