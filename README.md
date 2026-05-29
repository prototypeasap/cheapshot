# cheapshot

[![CI](https://github.com/prototypeasap/cheapshot/actions/workflows/ci.yml/badge.svg)](https://github.com/prototypeasap/cheapshot/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/prototypeasap/cheapshot)](https://goreportcard.com/report/github.com/prototypeasap/cheapshot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

- LLM pipelines with Unix pipes
- Same prompts across providers and models
- Half-price batch via OpenAI and Anthropic

*Agents drive it like any CLI.*

cheapshot is plumbing, not policy. It forwards requests, captures responses, and surfaces provider metrics. It does not bundle pricing, interpret response semantics, or maintain vendor schemas.

Handles file upload, polling, pagination, crash recovery, and result download for the OpenAI and Anthropic Batch APIs — so you can think in pipelines instead of API lifecycle.

For providers without a batch API (DeepSeek, local models, any OpenAI-compatible endpoint), direct mode sends concurrent requests through the chat API. Tools like Claude Code or Codex can pipe structured output between stages without any SDK integration.

**Key concept:** `prepare` owns the request body (model, messages, parameters). `run` owns delivery (base_url, api_key, mode, concurrency). `run` never modifies the body — it forwards it verbatim.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/prototypeasap/cheapshot/main/install.sh | sh
```

Detects OS and architecture automatically. Works on macOS and Linux (amd64/arm64). Set `CHEAPSHOT_INSTALL_DIR` to change the install path (default: `/usr/local/bin`).

Or via Homebrew:

```bash
brew install prototypeasap/tap/cheapshot
```

Or with Go:

```bash
go install github.com/prototypeasap/cheapshot/cmd/cheapshot@latest
```

**Windows (PowerShell):** Download the `.zip` from [releases](https://github.com/prototypeasap/cheapshot/releases) and add to your PATH:

```powershell
Invoke-WebRequest -Uri "https://github.com/prototypeasap/cheapshot/releases/latest/download/cheapshot_windows_amd64.zip" -OutFile cheapshot.zip
Expand-Archive cheapshot.zip -DestinationPath "$env:LOCALAPPDATA\cheapshot"
$env:PATH += ";$env:LOCALAPPDATA\cheapshot"
```

Or grab any binary directly from [releases](https://github.com/prototypeasap/cheapshot/releases).

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
2. `extract --json` parses that JSON and promotes each field to a top-level JSONL key (the raw response is also preserved in `text`)
3. The next `prepare -t '{{.fieldName}}'` references those promoted keys by name

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

## Local servers

Works out of the box with vLLM, llama.cpp, and Ollama. No API key needed — just point at the server:

```yaml
providers:
  qwen-vllm:
    base_url: http://192.168.1.97:8000  # host only — cheapshot appends /v1/chat/completions
    model: "/models/Qwen3.5-122B-A10B-NVFP4"
    format: openai
    mode: direct
    concurrency: 16

  qwen-llamacpp:
    base_url: http://192.168.1.160:8080  # NOT http://…:8080/v1
    model: "Qwen3.6-35B-A3B-UD-Q4_K_XL.gguf"
    format: openai
    mode: direct
    concurrency: 1

  llama-ollama:
    base_url: http://localhost:11434  # Ollama default port
    model: "llama3:8b-instruct"
    format: openai
    mode: direct
    concurrency: 4
```

> **`base_url` is the server host only** — do not include `/v1`. cheapshot appends `/v1/chat/completions` automatically. Including `/v1` in `base_url` produces a double-path (`/v1/v1/...`) and a silent 404. This differs from the OpenAI Python SDK convention where `base_url` includes `/v1`.

Model names with slashes, colons, and `.gguf` extensions all work. Vendor-specific fields (like `chat_template_kwargs` for Qwen) can be passed via `extra_body` in the provider config or `--extra-body` on the CLI.

## Extra body fields

Pass vendor-specific fields into the request body at three levels. **Per-line wins over CLI, CLI wins over config. Nested objects are deep-merged.**

```yaml
# 1. Provider config — base defaults
providers:
  qwen-vllm:
    extra_body:
      chat_template_kwargs:
        enable_thinking: false
        thinking_budget: 100
```

```bash
# 2. CLI --extra-body — overrides config (one key at a time)
cheapshot prepare -p qwen-vllm -m qwen3 \
  --extra-body 'chat_template_kwargs={"enable_thinking":true}' \
  --extra-body 'repetition_penalty=1.05'
```

```jsonl
# 3. Per-line extra_body in JSONL input — overrides CLI
{"text":"prompt here", "extra_body": {"chat_template_kwargs": {"thinking_budget": 500}}}
```

Result body after merging all three:
```json
{
  "chat_template_kwargs": {"enable_thinking": true, "thinking_budget": 500},
  "repetition_penalty": 1.05
}
```

`enable_thinking` came from CLI (overrode config's `false`). `thinking_budget` came from per-line (overrode config's `100`). `repetition_penalty` came from CLI (no override). Reserved per-line keys (`temperature`, `top_p`, `seed`, `stop`, `max_tokens`, `presence_penalty`, `frequency_penalty`, `min_p`) are set directly on the body, not nested under `extra_body`.

## Configuration

Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` to get started. For multiple providers, use a config file:

### Config profiles

`~/.cheapshot/config.yaml`:

```yaml
providers:
  openai:
    model: gpt-4.1-nano
    mode: batch                        # 50% off via batch API
  anthropic:
    model: claude-haiku-4-5-20251001
    mode: batch
  deepseek:
    base_url: https://api.deepseek.com # host only, no /v1
    model: deepseek-chat
    mode: direct                       # no batch API
    concurrency: 10
    api_key_env: DEEPSEEK_API_KEY      # reads key from this env var
    format: openai                     # wire format (openai or anthropic)
  local:
    base_url: http://localhost:8080    # no api_key_env = no auth header sent
    model: Qwen3-35B
    mode: direct
    concurrency: 1
    format: openai

default: openai
extract_meta: true  # always include model/tokens/latency in extract output
```

Providers with a `base_url` and no `api_key_env` skip the auth header entirely — local servers just work.

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
| `prepare` | Turn text or JSONL into provider-native batch format. `-p` accepts a config profile name or format (`openai`, `anthropic`). |
| `run` | Submit + poll + download (batch) or concurrent execution (direct). **Body is forwarded verbatim from prepare** — `-p` only resolves transport (base_url, api_key, mode, concurrency). To change the model, re-run `prepare`. |
| `extract` | Pull response text. See [extract output](#extract-output) below. |
| `submit` | Submit a pre-built batch file |
| `status` | Check batch status (`--watch` for polling) |
| `results` | Download completed batch results |
| `list` | Show tracked batches |
| `cancel` | Cancel a running batch |
| `recover` | Resume batches after a crash |

### Extract output

`extract` default: one line of plain text per result.

| Flags | Output format | Notes |
|-------|--------------|-------|
| *(none)* | `Paris` | Raw text, one line per result |
| `--with-id` | `{"id":"line-1", "text":"Paris"}` | JSONL with custom_id |
| `--json` | `{"id":"line-1", "text":"{...}", "fen":"...", "theme":"..."}` | Parses JSON response, promotes fields to top-level. `text` key preserved with raw response. |
| `--meta` | `{"text":"Paris", "model":"gpt-4.1-nano", "input_tokens":15, "output_tokens":3, "finish_reason":"stop"}` | Adds model, tokens, finish_reason. `latency_ms` included for direct mode only (meaningless for async batch). |
| `--json --meta --with-id` | All of the above combined | Flags compose freely |
| `--field name` | Extracts a specific JSON field from the response text | |

Set `extract_meta: true` in config to always enable `--meta` without the flag.

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
