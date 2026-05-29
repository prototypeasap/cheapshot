# Show HN: cheapshot — Unix-style LLM batch processing at 50% off

## Title options (pick one)

- **Show HN: cheapshot — Unix-style LLM batch processing, 50% off every request**
- **Show HN: cheapshot — pipe LLM requests like Unix text, pay half price**
- **Show HN: cheapshot — I built a CLI to chain LLM batches with Unix pipes at 50% off**

---

## Post body

When Anthropic launched Claude Code, they marketed it as a tool that ["follows the Unix philosophy. Pipe logs into it, run it in CI, or chain it with other tools."](https://code.claude.com/docs/en/overview) That resonated with a lot of us. Unix composability was the differentiating pitch — not another IDE plugin, but a real CLI citizen.

Starting June 15, Anthropic is splitting `claude -p` (the pipe-friendly, scriptable mode) out of the Max subscription and billing it at API rates. The subsidized 15-30x discount on programmatic usage is ending. Users will get $20-200/month in API credits depending on their plan, and pay standard rates beyond that.

This isn't surprising — those economics were never sustainable — but it does mean that if you're running LLM workloads programmatically, cost matters again. And both OpenAI and Anthropic have been quietly offering a way to cut that cost in half: their Batch APIs.

The problem is that batch APIs are tedious. You upload JSONL files in provider-specific formats, poll for completion, paginate results, handle crashes. It's enough friction that most people just pay full price for sync calls.

So I built **cheapshot**: a single Go binary that handles the batch lifecycle and lets you think in Unix pipelines instead.

### What it looks like

```bash
# Ask 1000 questions at 50% off
cat questions.txt \
  | cheapshot prepare -m gpt-4.1-nano \
  | cheapshot run -i - -o answers.jsonl

cheapshot extract -i answers.jsonl
```

`prepare` turns plain text into provider-native JSONL. `run` uploads, polls, downloads. `extract` pulls out the response text. Everything pipes through stdin/stdout. Progress goes to stderr. Standard Unix.

### The interesting part: structured JSON pipelines

Where this gets useful is chaining stages. The output of one LLM becomes the structured input to the next — no glue scripts, no intermediate parsing:

```bash
# Stage 1: Generate chess puzzles as structured JSON
seq 10 | cheapshot prepare -m gpt-4.1-nano \
    -s "Create a chess puzzle. Respond in JSON." \
    -t "Create chess puzzle #{{.text}}" \
    --json-schema schemas/chess-puzzle.json \
  | cheapshot run -i - -o puzzles.jsonl

# Stage 2: Rate them (extract --json promotes JSON fields to JSONL keys)
cheapshot extract -i puzzles.jsonl --json \
  | cheapshot prepare -m gpt-4.1-nano \
    -s "Rate this puzzle 1-10. Respond in JSON." \
    -t 'FEN: {{.fen}}\nTheme: {{.theme}}\nSolution: {{.solution}}' \
    --json-schema schemas/chess-rating.json \
  | cheapshot run -i - -o ratings.jsonl
```

The pipeline contract is simple:
1. `--json-schema` forces the model to return structured JSON
2. `extract --json` parses that JSON and promotes each field to a top-level JSONL key
3. The next `prepare -t '{{.fieldName}}'` references those keys by name

This is the daisy-chain. Each stage has a typed contract. No regex, no "parse this markdown," no hoping the model remembered the format. It's structured data all the way through.

### Translation: cheap model drafts, smart model reviews

This is the pattern I use in production for news translation. A cheap model does the bulk work, a better model does QA:

```bash
# Stage 1: Translate German political news with the cheapest model
cat articles.txt \
  | cheapshot prepare -m gpt-4.1-nano \
    -s "Translate this German news article to English." \
  | cheapshot run -i - -o draft.jsonl

# Stage 2: QA pass with a smarter model
cheapshot extract -i draft.jsonl --with-id \
  | paste -d'|' articles.txt - \
  | cheapshot prepare -m gpt-4.1 \
    -s "Review this German-to-English translation. Fix errors." \
    -t 'Original: {{.original}}\nDraft: {{.draft}}' \
  | cheapshot run -i - -o final.jsonl
```

Cheap models make predictable mistakes on proper nouns. In German political news, nano-class models will sometimes translate "Merz" (the current chancellor, Friedrich Merz) as "Merkel" — the more famous name pattern-matches harder. A QA pass with a smarter model catches these. Two cheap batches cost less than one expensive sync run, and you get better results.

(The tool is called cheapshot, after all.)

### Direct mode: DeepSeek, local models, anything

Not every provider has a batch API. For DeepSeek, local vLLM, Ollama, or any OpenAI-compatible endpoint, there's direct mode — concurrent HTTP requests, no batching:

```bash
cheapshot run -p deepseek --mode direct -i batch.jsonl -o results.jsonl
```

Same input format, same output format, same `extract`. You can prepare a batch once and run it against multiple providers to compare results:

```bash
cat prompts.txt | cheapshot prepare -m gpt-4.1-nano > batch.jsonl

cheapshot run -p openai    -i batch.jsonl -o results-openai.jsonl
cheapshot run -p deepseek  -i batch.jsonl -o results-deepseek.jsonl
cheapshot run -p anthropic -i batch.jsonl -o results-anthropic.jsonl
```

This is how I test which model works best for a specific task before committing to it. Same prompts, structured output, compare the results with `diff` or `jq`.

### Config profiles

Provider defaults live in `~/.cheapshot/config.yaml`:

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

default: openai
```

### Benchmarks

Three-stage chess pipeline (10 puzzles — generate, rate, solve):

| Provider | Mode | Model | Total | Stage 1 | Stage 2 | Stage 3 |
|----------|------|-------|-------|---------|---------|---------|
| OpenAI | batch | gpt-4.1-nano | 4m 11s | 1m 19s | 17s | 2m 34s |
| Anthropic | batch | claude-haiku-4-5 | 7m 23s | 2m 02s | 2m 33s | 2m 48s |
| DeepSeek | direct | deepseek-chat | 8s | 3s | 2s | 3s |

Batch mode has queuing overhead (you're in a queue behind other batch users) but costs 50% less. Direct mode is instant but full price. DeepSeek direct is both fast and extremely cheap ($0.14/M input, $0.28/M output).

### For agents

`cheapshot` is designed to be driven by AI agents. Claude Code, Codex, or any LLM-powered tool that can run shell commands can use it:

- Deterministic CLI interface (no interactive prompts, no TUI)
- JSON on stdout, progress on stderr
- Exit codes for success/failure
- Crash recovery via `cheapshot recover`
- Structured output schemas for predictable parsing

Here's what it looks like when Claude Code drives a two-stage translation pipeline autonomously: [agent demo video]

No custom function schemas, no SDK integration. The agent reads `--help`, figures out the flags, composes `cat | prepare | run | extract` chains, and handles the output. The same way a human would, except it doesn't get bored after the third pipeline.

If you're building agentic workflows that need to run thousands of LLM requests as part of a larger pipeline, this is the plumbing layer.

### Security

API keys are env-only — read from environment variables, never stored in config files or the database. The config file's `api_key_env` field takes an env var *name*, not a key value; cheapshot rejects it if you accidentally paste a raw key. The `~/.cheapshot/` directory is created 0700 and cheapshot warns if config files are world-readable. No key ever appears in logs, stderr output, or the SQLite state database.

### Technical details

- Single Go binary, no CGO, no external dependencies
- Pure-Go SQLite for batch state tracking
- Crash-safe: batches continue server-side; `cheapshot recover` picks them up
- Handles OpenAI and Anthropic format differences (including the Anthropic structured-output-via-tool-use trick)
- Retries with exponential backoff on transient failures

GitHub: https://github.com/prototypeasap/cheapshot

---

## Timing and strategy

**Target release: June 10-12, 2026** (3-5 days before the June 15 Anthropic billing change)

This timing is intentional:
- The June 15 `claude -p` billing split will generate HN discussion about programmatic LLM costs
- cheapshot is directly relevant: if you're paying API rates anyway, batch pricing cuts that in half
- The Unix philosophy angle ties back to what made `claude -p` appealing in the first place
- Being live a few days before the change means the tool exists when people start looking for alternatives
- Anthropic's ["dynamic workflows" blog post](https://claude.com/blog/introducing-dynamic-workflows-in-claude-code) (May 2026) announced Claude Code spawning hundreds of parallel subagents per session — token consumption is exploding and cheapshot's 50% batch discount is the cost control layer underneath

### The dynamic workflows spin

Anthropic just shipped "dynamic workflows" in Claude Code — the agent now orchestrates tens to hundreds of parallel subagents for large tasks (codebase migrations, security audits, dead code cleanup). Their own blog post warns: "Workflows consume substantially more tokens than standard sessions."

This is the perfect complement angle for cheapshot:

**cheapshot is the cost layer underneath agentic workflows.** When an AI agent fires off hundreds of LLM requests as part of a migration, those requests don't need to be synchronous. Pipe them through `cheapshot run` in batch mode and pay half price. The agent doesn't care about the 5-minute queue delay — it's working on other files anyway.

The stack:
```
AI agent (Claude Code, Codex, custom)  ← decides what to do
    ↓ drives
cheapshot CLI                           ← does it at half price
    ↓ calls
OpenAI/Anthropic batch APIs             ← 50% discount
```

Possible HN comment hook if the dynamic workflows post is still being discussed:

> "We built cheapshot specifically for this use case — when agents are spawning hundreds of parallel LLM requests, batch pricing cuts your bill in half. Same input format, same output format, just `cheapshot run` instead of direct API calls. The agent doesn't notice the difference; your wallet does."

**Posting strategy:**
- Post as "Show HN" on a Tuesday or Wednesday morning (US time), ideally 9-10am ET
- Title should be short and specific — lead with the Unix/pipe angle, not just "batch API wrapper"
- Have the README, examples, and release binary ready before posting
- Respond to early comments quickly (first 1-2 hours are critical for HN traction)

---

## Part 2: Daily model report — "The cheapshot daily"

A daily automated benchmark that runs the same tasks across models and publishes a comparison report. This is both useful content and ongoing marketing for the tool.

### Concept

Every day at 6am UTC, a GitHub Actions cron job runs cheapshot pipelines against multiple models on the same set of tasks. Results are committed as JSON + markdown, and a GitHub Pages site renders them as an updating dashboard. Each day's report is a snapshot; the dashboard shows trends over time.

Reference architecture: [daily-bench](https://github.com/jacobphillips99/daily-bench) — runs HELMLite benchmarks 4x/day, publishes to GitHub Pages. We do the same thing but with fun, relatable tasks instead of academic benchmarks.

### Daily tasks (rotate or run all)

**1. Translation gauntlet — "Lost in translation"**
- 10 German political news sentences with tricky proper nouns (Merz, Scholz, Habeck, Baerbock, Weidel)
- Run through each model as a single-shot translation
- Score: does the model keep proper nouns correct? Does it hallucinate "Merkel" for "Merz"?
- Bonus: run the two-stage pipeline (cheap draft + smart QA) and show correction rate
- This is the signature cheapshot benchmark — proper noun accuracy in political translation

**2. Chess puzzle quality — "Can your LLM play chess?"**
- Generate 5 puzzles per model with structured JSON output
- Cross-validate: does the FEN parse? Is the solution legal? Is the difficulty rating sane?
- Score: valid puzzle rate, solution correctness (checked by a reference model or Stockfish)
- Show the `--json-schema` pipeline in action

**3. Structured extraction — "Read the article"**
- Feed the same 5 news articles to each model
- Extract: headline, summary, named entities, sentiment — via `--json-schema`
- Compare: entity overlap, summary quality (judged by a reference model)
- Shows the structured output daisy-chain working across providers

**4. Idiom translation — "Translate this literally"**
- 10 idioms in various languages that are notoriously hard to translate
- Score: does the model translate the meaning or just the words?
- Fun, shareable, people love arguing about idiom translations

**5. Code review — "Find the bug"**
- 5 code snippets with intentional bugs (off-by-one, null deref, SQL injection)
- Score: does the model find the bug? Does it give the right fix?
- Practical and relatable for HN audience

### Technical implementation

```
.github/workflows/daily-report.yml
```

```yaml
name: Daily Model Report
on:
  schedule:
    - cron: '0 6 * * *'
  workflow_dispatch:  # manual trigger for testing

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build cheapshot
        run: go build -o cheapshot ./cmd/cheapshot

      - name: Run benchmarks
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          DEEPSEEK_API_KEY: ${{ secrets.DEEPSEEK_API_KEY }}
        run: ./bench/run-daily.sh

      - name: Generate report
        run: ./bench/generate-report.sh

      - name: Deploy to GitHub Pages
        uses: peaceiris/actions-gh-pages@v4
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./bench/site
```

### Report output structure

```
bench/
├── run-daily.sh              # orchestrator script
├── generate-report.sh        # turns JSON results into markdown + HTML
├── tasks/
│   ├── translation/
│   │   ├── input.txt         # 10 German political sentences (fixed)
│   │   ├── scoring.sh        # proper noun accuracy checker
│   │   └── system-prompt.txt
│   ├── chess/
│   │   ├── generate.sh
│   │   ├── validate.sh       # FEN validation, move legality
│   │   └── schemas/
│   ├── extraction/
│   ├── idioms/
│   └── code-review/
├── results/
│   └── 2026-06-10/
│       ├── raw/              # raw JSONL outputs per model per task
│       ├── scores.json       # aggregated scores
│       └── report.md         # human-readable daily report
└── site/
    ├── index.html            # dashboard with charts
    ├── data/                 # historical scores.json per day
    └── reports/              # rendered daily reports
```

### Dashboard design

The GitHub Pages site shows:

1. **Today's scorecard** — table of models x tasks with color-coded scores
2. **Trend charts** — line charts per task showing each model's score over time (uses Chart.js or similar)
3. **Daily report** — rendered markdown with specific examples (the funniest mistranslations, the worst chess puzzles, etc.)
4. **Cost column** — actual API cost per model per task (cheapshot can log token counts)

Example daily scorecard:

| Model | Translation | Chess | Extraction | Idioms | Code Review | Cost |
|-------|------------|-------|------------|--------|-------------|------|
| gpt-4.1-nano | 7/10 | 3/5 valid | 85% entities | 6/10 | 3/5 bugs | $0.02 |
| gpt-4.1-mini | 9/10 | 4/5 valid | 92% entities | 8/10 | 4/5 bugs | $0.15 |
| claude-haiku-4-5 | 8/10 | 4/5 valid | 88% entities | 7/10 | 4/5 bugs | $0.08 |
| deepseek-chat | 8/10 | 3/5 valid | 80% entities | 7/10 | 3/5 bugs | $0.01 |

### What makes this different from existing leaderboards

- **Not academic benchmarks** — fun, relatable tasks that normal people care about
- **Daily cadence** — catches model regressions and updates (models change silently)
- **Shows the tool** — every report is a live demo of cheapshot pipelines
- **Cost-aware** — tracks actual spend per model, not just quality
- **Reproducible** — the exact cheapshot commands are in the report; anyone can re-run them
- **Cheap to run** — batch pricing + nano-class models means the whole daily suite costs cents

### Content strategy

Each daily report is also a social media post:

> "Today's cheapshot daily: gpt-4.1-nano translated 'Bundeskanzler Merz' as 'Chancellor Merkel' in 3/10 sentences. DeepSeek got it right every time. Full report: [link]"

This is the kind of thing that gets retweeted by ML people. The Merz/Merkel thing is a running gag that builds brand for cheapshot.

---

## Part 3: Repo polish and launch assets

Everything below needs to be done before the Show HN post. Organized by priority.

### Quick wins (1-2 hours each)

**1. Hero GIF at the top of README**

Use [Charmbracelet VHS](https://github.com/charmbracelet/vhs) — write a `.tape` file that scripts the terminal demo. Commit the `.tape` so the GIF is reproducible in CI.

The `.tape` script should show:
1. `cat questions.txt` (show the input)
2. `cat questions.txt | cheapshot prepare -m gpt-4.1-nano` (show the JSONL)
3. Full pipeline: `prepare | run -i - -o answers.jsonl` (show polling)
4. `cheapshot extract -i answers.jsonl` (show the answers)

VHS generates a `.gif` file. Embed it at the top of the README. There's also a [VHS GitHub Action](https://github.com/charmbracelet/vhs-action) to regenerate on every release.

**2. README badges**

```markdown
[![Go](https://github.com/prototypeasap/cheapshot/actions/workflows/ci.yml/badge.svg)](https://github.com/prototypeasap/cheapshot/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/prototypeasap/cheapshot)](https://goreportcard.com/report/github.com/prototypeasap/cheapshot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
```

Requires CI workflow first (item 6).

**3. CONTRIBUTING.md + issue templates**

Create `.github/ISSUE_TEMPLATE/bug_report.md` and `feature_request.md`. Short, templated. Signals "real project, contributions welcome."

**4. Shell completions**

Cobra generates these for free. Add a `cheapshot completion bash/zsh/fish` subcommand or bundle them in the release archive via GoReleaser.

### Medium effort (half-day each)

**5. Pipeline diagram**

Horizontal flow diagram showing the three-stage chess pipeline. Each box is a cheapshot command, arrows are files/pipes.

```
[input.txt] → prepare → [batch.jsonl] → run → [puzzles.jsonl]
                                                      ↓
                                            extract --json
                                                      ↓
                                        [JSONL: {fen, theme, ...}]
                                                      ↓
                                  prepare -t '{{.fen}}' → run → [ratings.jsonl]
```

Style: dark terminal aesthetic, monospace. Excalidraw (hand-drawn, suits HN) or Mermaid with dark theme. SVG for README, PNG 2x for social.

**6. CI workflow + GoReleaser**

`.github/workflows/ci.yml`: test + lint on push/PR.
`.goreleaser.yaml`: multi-platform builds (linux/darwin x amd64/arm64), checksums, changelog, shell completions in archive.

Tag a release → GoReleaser builds everything → GitHub Release with assets.

**7. Homebrew tap**

Create `imp/homebrew-tap` repo. GoReleaser auto-publishes the formula on tag push. This removes the biggest install friction. Simon Willison's `llm`, Charm tools, and `aichat` all offer `brew install`.

**8. Agent demo recording**

Record Claude Code (or Codex) autonomously driving a multi-stage cheapshot pipeline. This is the strongest visual for the "built for agents" claim.

**Scenario:** Give Claude Code: "Use cheapshot to translate these 50 German news headlines to English with gpt-4.1-nano, then run a QA pass with gpt-4.1 to catch proper noun errors. Show me the corrections."

Record it building the `prepare | run | extract` pipeline, waiting for batch completion, constructing the QA stage from extracted output, and reporting results. All without human intervention.

Tips:
- OBS or screen capture, not asciinema (want full IDE/terminal chrome)
- Split screen option: agent reasoning left, terminal execution right
- 90-120s, speed up batch polling (2-4x), keep command construction at real speed
- Cut a 15-20s GIF loop for README embed, full video linked separately

**9. Cost comparison table**

Compute actual costs for a 1000-request workload:

| Approach | Model | Cost | Time |
|----------|-------|------|------|
| Sync API | gpt-4.1-nano | $X | sequential |
| cheapshot batch | gpt-4.1-nano | $X/2 | ~5 min |
| cheapshot direct | deepseek-chat | $Y | ~30s |
| Two-stage (nano draft + 4.1 QA) | combo | $Z | ~10 min |

Use real per-token pricing. Highlight the 50% savings.

**10. Social card / OG image**

1200x630 PNG for link previews. "cheapshot" in large monospace, tagline, a short pipeline snippet. Dark background, terminal green accent. Set as `og:image` in repo settings.

**11. Terminal recording (VHS)**

60-second demo of the full chess pipeline end-to-end with real API calls. Use the VHS `.tape` file from item 1 but expanded to show all three stages. Host on asciinema.org or embed the GIF directly.

### Post-launch

**12. Examples gallery**

Move `examples/` scripts into browsable docs (GitHub Pages with plain markdown or mkdocs). Each example is copy-pasteable: translation, chess, classification, summarization, code review.

**13. Recipe/pattern library**

Inspired by [Fabric's patterns](https://github.com/danielmiessler/fabric). Ship reusable pipeline templates as `cheapshot/recipes/` that users can contribute to. Each recipe: a system prompt, a template, a schema, and a one-liner to run it.

### Pre-launch checklist

- [ ] Hero GIF embedded at top of README (VHS `.tape` committed)
- [ ] README badges (Go, Go Report Card, MIT license)
- [ ] All examples in README tested end-to-end and working
- [ ] `go install` path works from a clean machine
- [ ] GoReleaser config: linux/darwin x amd64/arm64 builds
- [ ] CI workflow: test + lint on push
- [ ] `cheapshot --version` prints the release tag
- [ ] GitHub Release with changelog and cross-platform binaries
- [ ] Homebrew tap formula
- [ ] Shell completions (bash/zsh/fish)
- [ ] CONTRIBUTING.md + issue templates
- [ ] Pipeline diagram in README
- [ ] Cost table with real computed numbers
- [ ] OG image set in repo
- [ ] Agent demo recording (Claude Code or Codex)
- [ ] 15-20s GIF loop from agent demo for README
- [ ] Config file example in README matches actual schema
- [ ] Proofread all examples for copy-paste correctness
- [ ] Test translation pipeline with tricky text for Merz/Merkel mistranslation
- [ ] Daily report infrastructure (bench/ directory, GitHub Actions cron, GitHub Pages)
- [ ] First daily report published and linked from README
