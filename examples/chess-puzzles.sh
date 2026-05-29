#!/bin/bash
set -euo pipefail

# Chess puzzle showdown: OpenAI generates, Anthropic rates, both solve.
# Pure batch — all at 50% off.

echo "=== Step 1: Generate 10 chess puzzles (OpenAI) ==="
seq 10 | xargs -I{} echo "Create a unique chess puzzle #{}. Provide: 1) FEN position 2) Difficulty 1-5 3) Theme (fork/pin/skewer/mate/etc) 4) Solution in algebraic notation. Format as JSON." \
  | cheapshot prepare -p openai -m gpt-4.1-nano \
      -s "You create chess puzzles. Always respond with valid JSON: {\"fen\": \"...\", \"difficulty\": N, \"theme\": \"...\", \"solution\": \"...\"}" \
  | cheapshot run -p openai -i - -o /tmp/puzzles.jsonl

echo ""
echo "=== Step 2: Rate creativity (Anthropic judges OpenAI) ==="
jq -r '.response.body.choices[0].message.content' /tmp/puzzles.jsonl \
  | cheapshot prepare -p anthropic -m claude-sonnet-4-6 --max-tokens 200 \
      -s "Rate this chess puzzle for creativity and instructional value. Format: SCORE/10 | One sentence reason." \
  | cheapshot run -p anthropic -i - -o /tmp/ratings.jsonl

echo ""
echo "=== Step 3: Solve puzzles (Anthropic) ==="
jq -r '.response.body.choices[0].message.content' /tmp/puzzles.jsonl \
  | cheapshot prepare -p anthropic -m claude-sonnet-4-6 --max-tokens 500 \
      -s "You are a chess grandmaster. Analyze the position and find the best continuation. Explain your reasoning." \
  | cheapshot run -p anthropic -i - -o /tmp/solutions.jsonl

echo ""
echo "=== Results ==="
echo "Puzzles generated: $(wc -l < /tmp/puzzles.jsonl)"
echo "Ratings:"
jq -r '.result.message.content[0].text' /tmp/ratings.jsonl | head -10
echo ""
echo "Solutions:"
jq -r '.result.message.content[0].text' /tmp/solutions.jsonl | head -20
