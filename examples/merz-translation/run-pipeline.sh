#!/bin/bash
set -euo pipefail

# Merz Translation Pipeline — 3 providers, 3 tiers, 6 languages
#
# Stage 1: Cheap models translate German headlines into EN/FR/ES/PT/ZH/HI
# Stage 2: Mid-tier models review translations, flag errors (especially Merz→Merkel)
# Stage 3: Top-tier models retranslate from scratch for comparison
#
# The cheapshot: will cheap models confuse Friedrich Merz with Angela Merkel?

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT="/tmp/merz-pipeline"
mkdir -p "$OUT"

# ── Model tiers ──────────────────────────────────────────────
# Provider       Cheap (Stage 1)       Mid (Stage 2)          Top (Stage 3)
# OpenAI         gpt-4.1-nano          gpt-4.1-mini           gpt-4.1
# DeepSeek       deepseek-chat         deepseek-chat          (same model, only one tier)
# Anthropic      claude-haiku-4-5      claude-sonnet-4-6      claude-sonnet-4-6

echo "╔══════════════════════════════════════════════════════════╗"
echo "║  Merz Translation Pipeline — The Cheapshot Benchmark    ║"
echo "║  7 headlines × 6 languages × 3 providers × 3 stages    ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ═══════════════════════════════════════════════════════════════
# STAGE 1: Cheap translation
# ═══════════════════════════════════════════════════════════════
echo "━━━ STAGE 1: Cheap model translation ━━━"
echo ""

echo "→ OpenAI gpt-4.1-nano (batch)..."
cheapshot prepare -p openai -m gpt-4.1-nano \
  --system-file "$SCRIPT_DIR/translate-system.txt" \
  --json-schema "$SCRIPT_DIR/translation-schema.json" \
  -t 'Translate this German headline: {{.text}}' \
  -i "$SCRIPT_DIR/headlines.txt" \
  | cheapshot run -p openai -i - -o "$OUT/stage1-openai.jsonl"
echo ""

echo "→ DeepSeek deepseek-chat (direct)..."
cheapshot prepare -p deepseek -m deepseek-chat \
  --system-file "$SCRIPT_DIR/translate-system.txt" \
  --json \
  -t 'Translate this German headline into English, French, Spanish, Portuguese, Mandarin Chinese, and Hindi. Return JSON with keys: original, en, fr, es, pt, zh, hi. Headline: {{.text}}' \
  -i "$SCRIPT_DIR/headlines.txt" \
  | cheapshot run -p deepseek --mode direct -i - -o "$OUT/stage1-deepseek.jsonl"
echo ""

echo "→ Anthropic claude-haiku-4-5 (batch)..."
cheapshot prepare -p anthropic -m claude-haiku-4-5-20251001 \
  --system-file "$SCRIPT_DIR/translate-system.txt" \
  --json \
  -t 'Translate this German headline into English, French, Spanish, Portuguese, Mandarin Chinese, and Hindi. Return JSON with keys: original, en, fr, es, pt, zh, hi. Headline: {{.text}}' \
  -i "$SCRIPT_DIR/headlines.txt" \
  | cheapshot run -p anthropic -i - -o "$OUT/stage1-anthropic.jsonl"
echo ""

# ═══════════════════════════════════════════════════════════════
# STAGE 2: Mid-tier review — find errors
# ═══════════════════════════════════════════════════════════════
echo "━━━ STAGE 2: Mid-tier review (find errors) ━━━"
echo ""

echo "→ Reviewing OpenAI translations with gpt-4.1-mini..."
cheapshot extract -i "$OUT/stage1-openai.jsonl" --json \
  | cheapshot prepare -p openai -m gpt-4.1-mini \
    --system-file "$SCRIPT_DIR/review-system.txt" \
    --json-schema "$SCRIPT_DIR/review-schema.json" \
    -t 'Review these translations of the German headline.
Original: {{.original}}
English: {{.en}}
French: {{.fr}}
Spanish: {{.es}}
Portuguese: {{.pt}}
Chinese: {{.zh}}
Hindi: {{.hi}}' \
  | cheapshot run -p openai -i - -o "$OUT/stage2-openai.jsonl"
echo ""

echo "→ Reviewing DeepSeek translations with gpt-4.1-mini..."
cheapshot extract -i "$OUT/stage1-deepseek.jsonl" --json \
  | cheapshot prepare -p openai -m gpt-4.1-mini \
    --system-file "$SCRIPT_DIR/review-system.txt" \
    --json-schema "$SCRIPT_DIR/review-schema.json" \
    -t 'Review these translations of the German headline.
Original: {{.original}}
English: {{.en}}
French: {{.fr}}
Spanish: {{.es}}
Portuguese: {{.pt}}
Chinese: {{.zh}}
Hindi: {{.hi}}' \
  | cheapshot run -p openai -i - -o "$OUT/stage2-deepseek.jsonl"
echo ""

echo "→ Reviewing Anthropic translations with claude-sonnet-4-6..."
cheapshot extract -i "$OUT/stage1-anthropic.jsonl" --json \
  | cheapshot prepare -p anthropic -m claude-sonnet-4-6 \
    --system-file "$SCRIPT_DIR/review-system.txt" \
    --json \
    -t 'Review these translations of the German headline. Return JSON with keys: original, en, fr, es, pt, zh, hi (corrected versions), and errors_found (array of {lang, issue, fix}).
Original: {{.original}}
English: {{.en}}
French: {{.fr}}
Spanish: {{.es}}
Portuguese: {{.pt}}
Chinese: {{.zh}}
Hindi: {{.hi}}' \
  | cheapshot run -p anthropic -i - -o "$OUT/stage2-anthropic.jsonl"
echo ""

# ═══════════════════════════════════════════════════════════════
# STAGE 3: Top-tier fresh translation for comparison
# ═══════════════════════════════════════════════════════════════
echo "━━━ STAGE 3: Top-tier fresh translation (ground truth) ━━━"
echo ""

echo "→ OpenAI gpt-4.1 (batch)..."
cheapshot prepare -p openai -m gpt-4.1 \
  --system-file "$SCRIPT_DIR/translate-system.txt" \
  --json-schema "$SCRIPT_DIR/translation-schema.json" \
  -t 'Translate this German headline: {{.text}}' \
  -i "$SCRIPT_DIR/headlines.txt" \
  | cheapshot run -p openai -i - -o "$OUT/stage3-openai.jsonl"
echo ""

# ═══════════════════════════════════════════════════════════════
# REPORT
# ═══════════════════════════════════════════════════════════════
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  RESULTS                                                ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

echo "── Stage 1: Cheap translations ──"
echo ""
for provider in openai deepseek anthropic; do
  echo "[$provider]"
  cheapshot extract -i "$OUT/stage1-$provider.jsonl" --json \
    | jq -r '"  \(.original)\n    EN: \(.en)\n    FR: \(.fr)\n    ES: \(.es)\n    PT: \(.pt)\n    ZH: \(.zh)\n    HI: \(.hi)\n"'
done

echo "── Stage 2: Errors found by reviewers ──"
echo ""
for provider in openai deepseek anthropic; do
  echo "[$provider translations reviewed]"
  cheapshot extract -i "$OUT/stage2-$provider.jsonl" --json \
    | jq -r 'if (.errors_found | length) > 0 then
      "  \(.original)\n" + (.errors_found | map("    ⚠ [\(.lang)] \(.issue) → \(.fix)") | join("\n")) + "\n"
    else
      "  \(.original)\n    ✓ No errors found\n"
    end'
done

echo "── Stage 3: Top-tier reference translations ──"
echo ""
echo "[gpt-4.1]"
cheapshot extract -i "$OUT/stage3-openai.jsonl" --json \
  | jq -r '"  \(.original)\n    EN: \(.en)\n    FR: \(.fr)\n    ES: \(.es)\n    PT: \(.pt)\n    ZH: \(.zh)\n    HI: \(.hi)\n"'

echo ""
echo "All results saved in $OUT/"
echo "Raw files: stage1-*.jsonl (cheap), stage2-*.jsonl (reviews), stage3-*.jsonl (top-tier)"
