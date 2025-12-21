# Cortex LLM Benchmark Results

Run timestamp: 2025-12-21_004613
Script: benchmark-llm-models.js
Endpoint: http://localhost:11434

## Models Tested
- phi3 (3.8GB, 3.8B)
- llama3.2 (2GB, 3.2B)
- mistral (4.1GB, 7B)
- codellama (3.8GB, 7B)
- qwen2.5 (?, 7B)
- llama3.1 (?, 8B)
- deepseek-coder (?, 7B)

## Tests
- Tag Suggestion
- Strict JSON Tags
- Project Suggestion
- File Summary
- Stream Summary (reports first token latency)

## Summary Table
| Model | Avg Time (s) | Tokens/sec | Quality |
|---|---:|---:|---|
| phi3 | 0.73 | 100.6 | Excellent |
| llama3.2 | 0.59 | 106.9 | Excellent |
| mistral | 1.75 | 52.8 | Excellent |
| codellama | 2.01 | 56.9 | Excellent |
| qwen2.5 | 1.14 | 53.4 | Excellent |
| llama3.1 | 1.34 | 49.5 | Excellent |
| deepseek-coder | 0.63 | 214.0 | Excellent |

## Notable Behavior
- llama3.2: Consistent, fast, and follows strict output formats well.
- phi3: Fast and mostly compliant, but occasionally adds code fences.
- qwen2.5: Good instruction following, moderate speed.
- mistral: High quality, slower responses.
- codellama: Tends to add extra commentary before the requested format.
- llama3.1: Often adds preambles and code fences despite strict format requests.
- deepseek-coder: Extremely fast but frequently adds extra prose around outputs.

## Streaming (First Token)
- phi3: 91ms
- llama3.2: 119ms
- mistral: 167ms
- codellama: 153ms
- qwen2.5: 191ms
- llama3.1: 219ms
- deepseek-coder: 45ms

## Recommendations
- Fastest: llama3.2 (0.59s avg)
- Balanced (speed + quality + format compliance): llama3.2
- Speed + streaming: deepseek-coder is very fast but less format-compliant

## Suggested Configuration
{
  "cortex.llm.enabled": true,
  "cortex.llm.endpoint": "http://localhost:11434",
  "cortex.llm.model": "llama3.2",
  "cortex.llm.maxContextTokens": 2000
}
