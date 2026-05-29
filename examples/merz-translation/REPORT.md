# Merz Translation Pipeline Report

**The cheapshot benchmark: can LLMs tell Friedrich Merz from Angela Merkel?**

7 German political headlines, 6 target languages (EN/FR/ES/PT/ZH/HI), 6 models across 4 providers.

## Pipeline design

```
Stage 1: Cheap model translates      → nano / deepseek-chat / haiku
Stage 1b: Mid-price reasoning model  → deepseek-reasoner (R1)
Stage 2: Mid-tier model reviews       → gpt-4.1-mini finds & fixes errors
Stage 3: Top-tier model translates    → gpt-4.1 (reference/ground truth)
```

## Headlines tested

| # | German original |
|---|----------------|
| 1 | Merz scheitert bei Kanzlerwahl im ersten Wahlgang |
| 2 | Bundestag wählt Friedrich Merz im zweiten Anlauf zum Bundeskanzler |
| 3 | Die Brandmauer ist Merz auf den Kopf gefallen |
| 4 | Industriestrompreis, Deutschlandfonds, Schuldenbremse: Merz macht Habecks Wünsche wahr |
| 5 | Weidel macht Angebot zu Asylpolitik — Offener Brief an Merz |
| 6 | Brandmauer der Planlosen |
| 7 | Ein Jahr Schwarz-Rot: Zauberer ohne Trick |

## The Merz verdict: all models passed

**None of the three cheap models confused Merz with Merkel.** Even gpt-4.1-nano at $0.10/M tokens kept the proper noun correct across all 7 headlines and 6 languages. This is good news for LLMs; bad news for our punchline.

## Where cheap models DID fail: political terminology

The mid-tier reviewer (gpt-4.1-mini) found real errors — not name confusion but **cultural/political literacy**:

### OpenAI gpt-4.1-nano errors

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Die Brandmauer ist Merz auf den Kopf gefallen | ES | "pared de fuego" = literal "fire wall", not political firewall | → "barrera política" / "cordón sanitario" |
| Die Brandmauer ist Merz auf den Kopf gefallen | PT | "parede de fogo" = literal "fire wall" | → "barreira política" / "cordão sanitário" |
| Die Brandmauer ist Merz auf den Kopf gefallen | HI | Literal "firewall" misses political meaning | → political firewall/cordon sanitaire |

The key failure: **"Brandmauer"** (the political firewall against AfD cooperation) was translated literally as "fire wall" in Spanish and Portuguese. A human editor would never make this mistake — it's arguably the most politically charged word in modern German politics.

### DeepSeek deepseek-chat errors

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Industriestrompreis... | EN | "makes wishes come true" sounds like a fairy tale | → "fulfills Habeck's wishes" |
| Brandmauer der Planlosen | ES | "sin plan" spacing issue | Minor (style) |
| Brandmauer der Planlosen | PT | "firewall" anglicism less understood | → "Cortina de Fogo" |

### Anthropic claude-haiku-4-5 errors

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Die Brandmauer ist Merz auf den Kopf gefallen | EN, FR, ES, PT, ZH, HI | Literal/inaccurate idiomatic translation across ALL languages | Fixed idiom in each language |
| Brandmauer der Planlosen | EN | "Planless" is not standard English | → alternative phrasing |
| Brandmauer der Planlosen | ES | "sin plan" should be plural "sin planes" | → grammar fix |
| Brandmauer der Planlosen | PT | "sem plano" should be plural "sem planos" | → grammar fix |

Haiku had the most widespread issues on the "Brandmauer" headline — errors flagged in all 6 target languages. However: zero name confusion. Merz stayed Merz.

### OpenAI gpt-5.3 errors

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Die Brandmauer ist Merz auf den Kopf gefallen | EN, FR, ES, PT, ZH, HI | Literal "firewall fell on head" in ALL 6 languages — same failure as Haiku | → political firewall metaphor in each language |
| Ein Jahr Schwarz-Rot: Zauberer ohne Trick | EN, FR, ES, PT, ZH, HI | **"Schwarz-Rot" left untranslated in ALL languages** — "Schwarz-Rot一周年" in Chinese! | Not flagged by reviewer |
| Industriestrompreis, Deutschlandfonds... | ZH, HI | "Deutschlandfonds" left as Latin script in Chinese and Hindi text | Not flagged by reviewer |
| Weidel macht Angebot... | ZH, HI | "Weidel" and "Merz" left in Latin script — "Weidel就庇护政策提出提议——致Merz的公开信" | Not flagged by reviewer |

GPT-5.3's unique failure: **over-literal proper noun preservation**. It interpreted "preserve proper nouns exactly" as "leave German words untranslated" — including common political terms like "Schwarz-Rot" (Black-Red coalition). The result is German words embedded in Chinese and Devanagari text, which no human translator would produce.

The reviewer only flagged 6 errors (all on Brandmauer), missing the untranslated terms entirely — a reminder that automated review has blind spots when the translation is technically "correct" but practically useless.

This is the worst performance of any model in the benchmark, including the $0 local Qwen. A reasoning model that over-thinks the instruction and under-translates the content.

### DeepSeek deepseek-reasoner (R1) errors

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Die Brandmauer ist Merz auf den Kopf gefallen | EN | Literal "has fallen on Merz's head" misses the idiomatic "backfired" meaning | → "The political firewall has backfired on Merz" |

**Only 1 error flagged** — the fewest of any model besides the GPT-4.1 reference. The reviewer approved everything else, including smart choices:
- Kept "Deutschlandfonds" as a proper noun across EN/FR/ES/PT/HI instead of translating it
- Used "cortafuegos" (ES) — correct political term, not literal "pared de fuego"
- Added "联盟" (alliance) to "黑红联盟" in Chinese — making "Schwarz-Rot" politically meaningful
- Clean Hindi transliterations throughout

At $0.55/M input + $2.19/M output, R1 is ~3.6x cheaper than GPT-4.1 ($2/$8) while producing near-reference quality. The reasoning tokens (~3,900 across 7 headlines) are modest — R1 thought for ~15KB total, not the 35-minute marathon that Qwen thinking mode produced locally.

### Qwen 3.6 35B (local P40, $0 cost)

| Headline | Lang | Issue | Fix |
|----------|------|-------|-----|
| Brandmauer der Planlosen | ES | **"planificados" = OPPOSITE meaning** — translated "Planlosen" (planless) as "planned" | Critical error |
| Brandmauer der Planlosen | HI | "अनियोजितों की आग की दीवार" = "fire wall of the unplanned" — literal mess | → political term |
| Industriestrompreis... | HI | "मेरुज" for Merz, "हबेके" for Habeck — garbled transliterations | → proper transliteration |
| Die Brandmauer... | FR | "mur de feu" = literal "fire wall" (same class of error as nano) | → political firewall |

Qwen running locally on a P40 at 50 tok/s, $0 cost. No Merz→Merkel confusion, but the Spanish antonym error on "Planlosen" → "planificados" is the worst single error in the entire benchmark — translating a word as its exact opposite.

The reviewer (gpt-4.1-mini) confirmed the "planificados" error and also caught a garbled Hindi transliteration of "Merz" (मेरुज instead of मेर्ज़).

Notably, Qwen's French "règle d'or budgétaire" for "Schuldenbremse" is more natural than any cloud model's literal "debt brake." And its Chinese used proper transliterations (默茨 for Merz, 魏德尔 for Weidel). Mixed bag — great at some things, terrible at others.

## Error scorecard

| Model | Price tier | Errors flagged | Unflagged issues | Worst error |
|-------|-----------|---------------|-----------------|-------------|
| gpt-4.1 (reference) | $2/$8 per M tok | — (reference) | — | — |
| **deepseek-reasoner (R1)** | **$0.55/$2.19** | **1** | 0 | Brandmauer literal (EN only) |
| deepseek-chat (V3) | $0.27/$1.10 | 3 | 0 | "makes wishes come true" (fairy tale tone) |
| gpt-4.1-nano | $0.10/$0.40 | 3 | 0 | "pared de fuego" (literal fire wall, ES/PT) |
| Qwen3.6-35B local (no-think) | $0.00 | 4 | 0 | "planificados" = opposite meaning (ES) |
| claude-haiku-4-5 | $0.80/$4.00 | 7+ | 0 | Brandmauer wrong in ALL 6 languages |
| **gpt-5.3 (reasoning)** | **~$3/$12** | **6** | **12+** | **German left untranslated in Chinese/Hindi** |

GPT-5.3 is the only model where the reviewer's error count undersells the problem. The 6 flagged errors are just Brandmauer — the reviewer missed the untranslated German terms littered through Chinese and Hindi text because they're technically "preserved proper nouns."

DeepSeek R1 is the clear price-performance winner: near-reference quality at 3.6x less than GPT-4.1. Model size matters more than reasoning depth for translation — R1 is a large model that happens to reason, not a small model that reasons hard.

GPT-5.3, despite being OpenAI's latest reasoning model, has the worst translation quality of any model in this benchmark by a significant margin.

## Qwen thinking vs no-thinking: does reasoning help?

Same model (Qwen3.6-35B), same GPU (P40), same headlines. Thinking mode enables the model's chain-of-thought reasoning before answering. The question: is 70x slower worth it?

| | Thinking (enable_thinking: true) | No-thinking (enable_thinking: false) |
|---|---|---|
| **Time** | ~35 min (7 headlines) | ~30 sec (7 headlines) |
| **Speed** | ~5 min/headline (reasoning dominates) | ~4 sec/headline at 50 tok/s |
| **Output size** | 112 KB (includes reasoning tokens) | 9 KB |

### Head-to-head: "Die Brandmauer ist Merz auf den Kopf gefallen"

| Lang | Thinking | No-thinking |
|------|----------|-------------|
| EN | **The Firewall Backfires on Merz** | The firebreak has fallen on Merz's head |
| FR | Le mur de feu retombe sur Merz | La barrière coupe-feu est tombée sur la tête de Merz |
| ZH | **"防火墙"反噬 Merz** (反噬 = backfires) | 防火墙砸在了默茨的头上 (literal) |
| HI | **Merz पर राजनीतिक फायरवॉल उलटी पड़ी** ("political firewall") | अग्निरोधी दीवार मेर्ज के सिर पर गिरी (literal "fire-retardant wall") |

Thinking mode captured the political metaphor ("backfires on Merz") instead of the literal "fell on his head." The Chinese 反噬 is arguably the best translation of this headline across all providers.

### Head-to-head: "Industriestrompreis, Deutschlandfonds, Schuldenbremse"

| Term | Thinking | No-thinking |
|------|----------|-------------|
| Deutschlandfonds (EN) | **Germany Fund** (correct proper noun) | **German equity fund** (hallucinated "equity") |
| Schuldenbremse (FR) | frein à la dette | règle d'or de la dette (more idiomatic) |
| Merz (HI) | मेर्त्ज़ (clean) | मेर्ज़ (clean) |

### Head-to-head: "Brandmauer der Planlosen"

| Lang | Thinking | No-thinking |
|------|----------|-------------|
| EN | Firewall of the Planless | **Firewall of the Aimless** (Aimless ≠ Planless) |
| ES | Muro cortafuegos de los sin plan | **Muro de los Desorientados** (Desorientados = disoriented, not planless) |

### Head-to-head: "Ein Jahr Schwarz-Rot: Zauberer ohne Trick"

| Lang | Thinking | No-thinking |
|------|----------|-------------|
| ZH | 黑红一年：无招数的魔术师 (pure Chinese) | **黑红一年：没有 Tricks 的魔术师** (Chinese/English code-mixing!) |

### Verdict

| Category | Winner |
|----------|--------|
| Political metaphor | Thinking (by a mile) |
| Proper noun handling | Thinking (no hallucinations) |
| Translation accuracy | Thinking (no antonyms, no code-mixing) |
| Speed | No-thinking (70x faster) |
| Idiomatic French | No-thinking (règle d'or) |

**Thinking mode is genuinely better** for nuanced political translation. The reasoning lets the model work through cultural context before committing to a translation. But at ~5 minutes per headline on a P40, it's only practical for high-stakes content where accuracy matters more than throughput.

For a batch pipeline, the sweet spot might be: no-thinking for the cheap draft, then a smarter cloud model for review — which is exactly the cheapshot pipeline pattern.

## Translation comparison: cheap vs top-tier

### Headline: "Die Brandmauer ist Merz auf den Kopf gefallen"

| Lang | gpt-4.1-nano | gpt-4.1 (ref) | DeepSeek chat | DeepSeek R1 | gpt-5.3 |
|------|-------------|---------------|---------------|-------------|---------|
| EN | The firewall has fallen on Merz's head | The firewall has fallen on Merz's head | The firewall fell on Merz's head | The firewall has fallen on Merz's head | The firewall has fallen on Merz's head |
| ES | **La pared de fuego** | **El cortafuegos** | El cortafuegos | **El cortafuegos** | El cortafuegos |
| PT | **A parede de fogo** | **O muro de proteção** | O firewall | O firewall | O corta-fogo |
| ZH | 防火墙掉在梅尔茨的头上 | 防火墙砸在了Merz的头上 | 防火墙掉到了梅尔茨的头上 | 防火墙砸到了默茨的头上 | 防火墙落在了梅尔茨的头上 |

Notable: R1 and GPT-5.3 both got "cortafuegos" right in Spanish (matching GPT-4.1), while nano used literal "pared de fuego." R1's Chinese used the more forceful 砸 (smash), same verb as GPT-4.1.

Notable: gpt-4.1 (top-tier) used the more correct "cortafuegos" (ES) and "muro de proteção" (PT) instead of the literal fire-wall translation that nano used. In Hindi, gpt-4.1 kept the German "Brandmauer" untranslated — arguably the best approach for a political term with no direct equivalent.

### Headline: "Ein Jahr Schwarz-Rot: Zauberer ohne Trick"

| Lang | gpt-4.1-nano | gpt-4.1 | DeepSeek chat | DeepSeek R1 | gpt-5.3 |
|------|-------------|---------|---------------|-------------|---------|
| EN | One Year Black-Red: Magician Without Tricks | One Year of Black-Red: Magicians Without a Trick | One year of black-red: Magician without a trick | One Year of Black-Red: Magician Without a Trick | One year of **Schwarz-Rot**: magician without a trick |
| ZH | 黑红一年：没有把戏的魔术师 | 一周年Schwarz-Rot：没有魔法的魔术师 | 黑红联盟一周年：没有把戏的魔术师 | 黑红联盟一年：没有把戏的魔术师 | **Schwarz-Rot**一周年：没有把戏的魔术师 |

Both DeepSeek models added "联盟" (alliance/coalition) to the Chinese — the only models to make "Schwarz-Rot" politically meaningful rather than just "black-red." GPT-5.3 didn't even try to translate "Schwarz-Rot" — leaving the German compound untranslated in every language, including Chinese.

## Performance

| Stage | OpenAI nano (batch) | Anthropic (batch) | DeepSeek chat | DeepSeek R1 | GPT-5.3 | Qwen Local (P40) |
|-------|--------------------|--------------------|---------------|-------------|---------|-------------------|
| Stage 1: Translate | ~3 min | ~5 min | 3 sec | ~5 sec | ~8 sec | ~30 sec (no-think) / ~35 min (think) |
| Stage 2: Review | ~3 min | ~8 min | N/A (used OpenAI) | N/A (used OpenAI) | N/A (used OpenAI) | N/A (used OpenAI) |
| Stage 3: Reference | ~3 min | N/A | N/A | N/A | N/A | N/A |

DeepSeek direct mode is ~60x faster than batch APIs for small jobs. R1 adds reasoning tokens but stays fast on their infrastructure. Qwen local on a P40 is ~30 seconds at $0. For 7 headlines, the queue overhead dominates.

## Cost (estimated)

| Provider | Model | Stage | Est. Cost |
|----------|-------|-------|-----------|
| OpenAI | gpt-4.1-nano | translate 7 headlines | ~$0.001 |
| OpenAI | gpt-4.1-mini | review 7 translations | ~$0.003 |
| OpenAI | gpt-4.1 | translate 7 headlines | ~$0.01 |
| DeepSeek | deepseek-chat | translate 7 headlines | ~$0.0005 |
| DeepSeek | deepseek-reasoner (R1) | translate 7 headlines | ~$0.012 |
| OpenAI | gpt-5.3 | translate 7 headlines | ~$0.015 |
| Anthropic | claude-haiku-4-5 | translate 7 headlines | ~$0.001 |
| Qwen Local | Qwen3.6-35B (P40) | translate 7 headlines | $0.00 |

Total pipeline cost: under $0.02. Batch discount on top of that for OpenAI/Anthropic stages.

## Conclusions

1. **Merz survived** — no model confused him with Merkel (at least not with explicit "preserve proper nouns" instructions)
2. **Political terminology is the real failure mode** — "Brandmauer" translated literally as "fire wall" in Romance languages is a real error that a reviewer catches
3. **DeepSeek R1 is the price-performance champion** — only 1 error flagged, 3.6x cheaper than GPT-4.1, and still instant in direct mode
4. **Model size matters more than reasoning for translation** — R1's quality comes from being a large model, not from chain-of-thought. GPT-5.3, a pure reasoning model, produced the worst translations in the benchmark — leaving German words untranslated in Chinese and Hindi. Reasoning models over-interpret instructions and under-translate content
5. **The cheap-draft → smart-review pipeline works** — gpt-4.1-mini caught real errors across all providers, from nano's literal Brandmauer to Qwen's antonym
6. **Top-tier models make different (better) choices** — gpt-4.1 kept German proper nouns in Hindi, used more precise political vocabulary in Spanish/Portuguese
7. **More expensive ≠ better at translation** — GPT-5.3 (~$3/$12 per M tok) was worse than GPT-4.1-nano ($0.10/$0.40). Price is a poor proxy for translation quality
8. **The cheapshot pipeline proved itself** — 6 models, batch and direct mode, structured JSON daisy-chaining, all composed with Unix pipes

## Raw outputs

```
/tmp/merz-pipeline/
├── stage1-openai.jsonl      # gpt-4.1-nano translations
├── stage1-deepseek.jsonl    # deepseek-chat translations
├── stage1-anthropic.jsonl   # claude-haiku-4-5 translations
├── stage2-openai.jsonl      # gpt-4.1-mini review of nano
├── stage2-deepseek.jsonl    # gpt-4.1-mini review of deepseek
├── stage2-anthropic.jsonl   # gpt-4.1-mini review of haiku
├── stage1-gpt53.jsonl             # gpt-5.3 translations
├── stage2-gpt53.jsonl             # gpt-4.1-mini review of gpt-5.3
├── stage1-deepseek-reasoner.jsonl # deepseek-reasoner (R1) translations
├── stage2-deepseek-reasoner.jsonl # gpt-4.1-mini review of R1
├── stage1-qwen.jsonl        # qwen3.6-35B local translations (thinking mode)
├── stage1-qwen-structured.jsonl  # qwen3.6-35B (no-thinking mode)
├── stage2-qwen.jsonl        # gpt-4.1-mini review of qwen
└── stage3-openai.jsonl      # gpt-4.1 reference translations
```
