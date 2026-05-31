#!/bin/bash
set -euo pipefail

# Use CHEAPSHOT env var to override the binary path, e.g.:
#   CHEAPSHOT=./cheapshot ./compare.sh ...
CHEAPSHOT="${CHEAPSHOT:-cheapshot}"

# Compare model responses across providers.
#
# Usage:
#   ./compare.sh prompts.txt qwen-local gemma-local openai
#   ./compare.sh prompts.txt qwen-local gemma-local --system "Be concise."
#   echo "What is 2+2?" | ./compare.sh - qwen-local gemma-local
#
# Options (must come after provider list):
#   --system "..."        System prompt
#   --template "..."      Prompt template
#   --max-tokens N        Max tokens per response
#   --strip-thinking      Strip <think> blocks from output
#   --extra-body K=V      Extra body fields (repeatable)
#   --outdir DIR          Output directory (default: /tmp/cheapshot-compare)

usage() {
  echo "Usage: $0 <input-file|-> <provider1> <provider2> [provider3...] [options]"
  echo ""
  echo "  input-file    Text file (one prompt per line), JSONL, or - for stdin"
  echo "  providers     Config profile names (e.g. qwen-local gemma-local openai)"
  echo ""
  echo "Options:"
  echo "  --system STR          System prompt"
  echo "  --template STR        Prompt template with {{.Field}} placeholders"
  echo "  --max-tokens N        Max output tokens"
  echo "  --strip-thinking      Strip <think>...</think> from output"
  echo "  --extra-body K=V      Extra body fields (repeatable)"
  echo "  --outdir DIR          Output directory (default: /tmp/cheapshot-compare)"
  exit 1
}

[ $# -lt 3 ] && usage

INPUT="$1"; shift

PROVIDERS=()
while [ $# -gt 0 ] && [[ ! "$1" == --* ]]; do
  PROVIDERS+=("$1"); shift
done

[ ${#PROVIDERS[@]} -lt 2 ] && { echo "Error: need at least 2 providers to compare"; exit 1; }

SYSTEM=""
TEMPLATE=""
MAX_TOKENS=""
STRIP_THINKING=""
EXTRA_BODY=()
OUTDIR="/tmp/cheapshot-compare"

while [ $# -gt 0 ]; do
  case "$1" in
    --system)        SYSTEM="$2"; shift 2 ;;
    --template)      TEMPLATE="$2"; shift 2 ;;
    --max-tokens)    MAX_TOKENS="$2"; shift 2 ;;
    --strip-thinking) STRIP_THINKING="1"; shift ;;
    --extra-body)    EXTRA_BODY+=("$2"); shift 2 ;;
    --outdir)        OUTDIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; usage ;;
  esac
done

mkdir -p "$OUTDIR"

# Handle stdin
if [ "$INPUT" = "-" ]; then
  INPUT="$OUTDIR/input.txt"
  cat > "$INPUT"
fi

echo "Comparing ${#PROVIDERS[@]} providers on $(wc -l < "$INPUT" | tr -d ' ') prompts"
echo "Output: $OUTDIR"
echo ""

# Prepare and run for each provider
for PROV in "${PROVIDERS[@]}"; do
  PREPARED="$OUTDIR/prepared-${PROV}.jsonl"
  RESULTS="$OUTDIR/results-${PROV}.jsonl"

  PREP_ARGS=(-p "$PROV")
  [ -n "$SYSTEM" ]     && PREP_ARGS+=(-s "$SYSTEM")
  [ -n "$TEMPLATE" ]   && PREP_ARGS+=(-t "$TEMPLATE")
  [ -n "$MAX_TOKENS" ] && PREP_ARGS+=(--max-tokens "$MAX_TOKENS")
  for EB in "${EXTRA_BODY[@]+"${EXTRA_BODY[@]}"}"; do
    [ -n "$EB" ] && PREP_ARGS+=(--extra-body "$EB")
  done

  echo "--- $PROV ---"
  $CHEAPSHOT prepare "${PREP_ARGS[@]}" < "$INPUT" > "$PREPARED"
  $CHEAPSHOT run -p "$PROV" -i "$PREPARED" -o "$RESULTS" 2>&1
  echo ""
done

# Extract and build comparison
echo "=== Results ==="
echo ""

EXTRACT_ARGS=(--meta --with-id)
[ -n "$STRIP_THINKING" ] && EXTRACT_ARGS+=(--strip-thinking)

for PROV in "${PROVIDERS[@]}"; do
  RESULTS="$OUTDIR/results-${PROV}.jsonl"
  EXTRACTED="$OUTDIR/extracted-${PROV}.jsonl"
  $CHEAPSHOT extract "${EXTRACT_ARGS[@]}" -i "$RESULTS" > "$EXTRACTED"
done

# Print summary table
printf "%-20s %8s %8s %8s %10s\n" "Provider" "In_tok" "Out_tok" "Latency" "Finish"
printf "%-20s %8s %8s %8s %10s\n" "--------" "------" "-------" "-------" "------"

for PROV in "${PROVIDERS[@]}"; do
  EXTRACTED="$OUTDIR/extracted-${PROV}.jsonl"
  python3 -c "
import json, sys
lines = [json.loads(l) for l in open('$EXTRACTED') if l.strip()]
if not lines:
    print(f'$PROV: no results')
    sys.exit()
in_tok = sum(l.get('input_tokens', 0) for l in lines)
out_tok = sum(l.get('output_tokens', 0) for l in lines)
lat = sum(float(l.get('latency_ms', 0)) for l in lines)
finishes = set(l.get('finish_reason', '?') for l in lines)
n = len(lines)
print(f\"{'$PROV':<20s} {in_tok:>8d} {out_tok:>8d} {lat/1000:>7.1f}s {','.join(finishes):>10s}\")
" 2>/dev/null || echo "$PROV: extraction failed"
done

echo ""

# Side-by-side responses
echo "=== Side-by-side (first 3 prompts) ==="
echo ""

for PROV in "${PROVIDERS[@]}"; do
  EXTRACTED="$OUTDIR/extracted-${PROV}.jsonl"
  echo "--- $PROV ---"
  head -3 "$EXTRACTED" | python3 -c "
import json, sys
for line in sys.stdin:
    obj = json.loads(line)
    text = obj.get('text', '')
    if len(text) > 200:
        text = text[:200] + '...'
    print(f\"  [{obj.get('id','?')}] {text}\")
" 2>/dev/null
  echo ""
done

echo "Full results: $OUTDIR/extracted-*.jsonl"
