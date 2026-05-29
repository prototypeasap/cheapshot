# cheapshot

[![CI](https://github.com/prototypeasap/cheapshot/actions/workflows/ci.yml/badge.svg)](https://github.com/prototypeasap/cheapshot/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/prototypeasap/cheapshot)](https://goreportcard.com/report/github.com/prototypeasap/cheapshot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

- LLM pipelines with Unix pipes
- Same prompts across providers and models
- Half-price batch via OpenAI and Anthropic

*Agents drive it like any CLI.*

Handles file upload, polling, pagination, crash recovery, and result download for the OpenAI and Anthropic Batch APIs — so you can think in pipelines instead of API lifecycle.

For providers without a batch API (DeepSeek, local models, any OpenAI-compatible endpoint), direct mode sends concurrent requests through the chat API. Tools like Claude Code or Codex can pipe structured output between stages without any SDK integration.

## Install

```bash
go install github.com/prototypeasap/cheapshot/cmd/cheapshot@latest
```

Or grab a binary from [releases](https://github.com/prototypeasap/cheapshot/releases).

## Quick start

```bash
export OPENAI_API_KEY=sk-...

# One question, end to end
echo "What is the capital of France?" \
  | cheapshot prepare -m gpt-4.1-nano \
  | cheapshot run -i - -o answers.jsonl

# Read the answer
cheapshot extract -i answers.jsonl
```

`prepare` formats your input into provider-native JSONL. `run` submits the batch, polls, and downloads results. `extract` pulls out the response text. Machine-readable output goes to stdout, progress to stderr.

## Structured JSON pipelines

You can chain stages where each model's structured JSON output becomes the next stage's input:

```bash
# Stage 1: Generate chess puzzles as JSON
seq 10 | cheapshot prepare -m gpt-4.1-nano \
    -s "Create a chess puzzle. Respond in JSON." \
    -t "Create chess puzzle #{{.text}}" \
    --json-schema examples/schemas/chess-puzzle.json \
  | cheapshot run -i - -o puzzles.jsonl

# Stage 2: Rate them — extract promotes JSON fields to JSONL keys
cheapshot extract -i puzzles.jsonl --json \
  | cheapshot prepare -m gpt-4.1-nano \
    -s "Rate this chess puzzle 1-10 for creativity. Respond in JSON." \
    -t 'FEN: {{.fen}}\nTheme: {{.theme}}\nSolution: {{.solution}}' \
    --json-schema examples/schemas/chess-rating.json \
  | cheapshot run -i - -o ratings.jsonl
```

How it works:
1. `--json-schema` forces structured JSON output from the model
2. `extract --json` parses that JSON and promotes each field to a top-level JSONL key
3. The next `prepare -t '{{.fieldName}}'` references those keys by name

## Translation example

A cheap model translates, a better model reviews:

```bash
# Stage 1: Translate with the cheapest model
cat articles.txt \
  | cheapshot prepare -m gpt-4.1-nano \
    -s "Translate this German news to English." \
  | cheapshot run -i - -o draft.jsonl

# Stage 2: QA with a smarter model
cheapshot extract -i draft.jsonl --with-id \
  | paste -d'|' articles.txt - \
  | cheapshot prepare -m gpt-4.1 \
    -s "Review this translation. Fix errors." \
    -t 'Original: {{.original}}\nDraft: {{.draft}}' \
  | cheapshot run -i - -o final.jsonl
```

## Cross-provider comparison

Prepare once, run against multiple providers:

```bash
cat prompts.txt | cheapshot prepare -p openai -m gpt-4.1-nano > batch.jsonl

cheapshot run -p openai    -i batch.jsonl -o results-openai.jsonl
cheapshot run -p anthropic -i batch.jsonl -o results-anthropic.jsonl
cheapshot run -p deepseek --mode direct -i batch.jsonl -o results-deepseek.jsonl  # no batch API
```

## Modes

| Mode | Flag | How it works | Pricing |
|------|------|-------------|---------|
| **batch** (default) | `--mode batch` | Upload → poll → download via batch API | 50% off (OpenAI, Anthropic) |
| **direct** | `--mode direct` | Concurrent HTTP requests via chat API | Standard pricing |

Direct mode works with any OpenAI-compatible API — DeepSeek, Together, vLLM, Ollama, etc.

## Configuration

Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` to get started. For multiple providers, use a config file:

### Config profiles

`~/.cheapshot/config.yaml`:

```yaml
providers:
  openai:
    model: gpt-4.1-nano
    mode: batch
  anthropic:
    model: claude-haiku-4-5-20251001
    mode: batch
  deepseek:
    base_url: https://api.deepseek.com
    model: deepseek-chat
    mode: direct
    concurrency: 10
    api_key_env: DEEPSEEK_API_KEY
    format: openai
  local:
    base_url: http://localhost:8080
    model: Qwen3-35B
    mode: direct
    concurrency: 1
    format: openai

default: openai
```

Providers with a `base_url` don't need an API key.

### Environment variables

| Variable | Purpose |
|----------|---------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_BASE_URL` | Override OpenAI base URL |
| `ANTHROPIC_BASE_URL` | Override Anthropic base URL |
| `CHEAPSHOT_PROVIDER` | Default provider |
| `CHEAPSHOT_DB` | Custom database path |
| `CHEAPSHOT_CONFIG` | Custom config file path |

## Commands

| Command | What it does |
|---------|-------------|
| `prepare` | Turn text or JSONL into provider-native batch format |
| `run` | Submit + poll + download (batch) or concurrent execution (direct) |
| `extract` | Pull response text; `--json` promotes structured fields to JSONL keys |
| `submit` | Submit a pre-built batch file |
| `status` | Check batch status (`--watch` for polling) |
| `results` | Download completed batch results |
| `list` | Show tracked batches |
| `cancel` | Cancel a running batch |
| `recover` | Resume batches after a crash |

## Crash recovery

Batch state is tracked in a local SQLite database (`~/.cheapshot/cheapshot.db`). Batches continue server-side if your process dies:

```bash
cheapshot recover
```

## Benchmarks

Three-stage chess pipeline (10 puzzles: generate, rate, solve). Batch requests are queued server-side — higher latency but 50% cheaper.

| Provider | Mode | Model | Total |
|----------|------|-------|-------|
| OpenAI | batch | gpt-4.1-nano | 251s |
| Anthropic | batch | claude-haiku-4-5 | 443s |
| DeepSeek | direct | deepseek-chat | 8s |

DeepSeek is fast and cheap for iteration.

## License

MIT
