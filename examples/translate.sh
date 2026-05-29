#!/bin/bash
set -euo pipefail

# Two-stage translation pipeline:
# Stage 1: Bulk translate with Haiku (cheap workhorse)
# Stage 2: QA/enhance with Sonnet (smart editor)
# Both at 50% batch pricing.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

cat > /tmp/strings.txt << 'EOF'
Welcome to our platform
Your order has been confirmed
Please enter your email address
Something went wrong. Please try again.
Your session has expired
EOF

echo "=== Stage 1: Translate with Haiku ==="
cheapshot prepare -p anthropic \
  -m claude-haiku-4-5 \
  --max-tokens 256 \
  -s "Translate to French. Output ONLY the translation, nothing else." \
  -i /tmp/strings.txt \
  | cheapshot run -p anthropic -i - -o /tmp/draft-fr.jsonl

echo ""
echo "=== Stage 2: Review with Sonnet ==="
# Pair originals with drafts for QA
paste -d'|' /tmp/strings.txt <(jq -r '.result.message.content[0].text' /tmp/draft-fr.jsonl) \
  | awk -F'|' '{print "{\"original\":\"" $1 "\",\"draft\":\"" $2 "\"}"}' \
  | cheapshot prepare -p anthropic \
      -m claude-sonnet-4-6 \
      --max-tokens 256 \
      -s "Review this English-to-French translation. Fix errors, improve naturalness. Output ONLY the final French translation." \
      -t 'Original: {{.original}}\nDraft: {{.draft}}' \
  | cheapshot run -p anthropic -i - -o /tmp/final-fr.jsonl

echo ""
echo "=== Results ==="
jq -r '.result.message.content[0].text' /tmp/final-fr.jsonl
