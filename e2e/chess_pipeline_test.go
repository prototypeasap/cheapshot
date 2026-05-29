package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cheapshot(t *testing.T, args ...string) string {
	t.Helper()
	bin := os.Getenv("CHEAPSHOT_BIN")
	if bin == "" {
		bin = "cheapshot"
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cheapshot %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func cheapshotPipe(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	bin := os.Getenv("CHEAPSHOT_BIN")
	if bin == "" {
		bin = "cheapshot"
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cheapshot %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestChessPipeline(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set — skipping live API test")
	}

	root := findRepoRoot(t)
	schemasDir := filepath.Join(root, "examples", "schemas")
	promptsDir := filepath.Join(root, "examples", "prompts")
	tmpDir := t.TempDir()

	model := os.Getenv("CHEAPSHOT_TEST_MODEL")
	if model == "" {
		model = "gpt-4.1-nano"
	}

	numPuzzles := 3

	// Stage 1: Generate puzzles with structured output schema
	t.Log("Stage 1: Generating puzzles...")
	var prompts []string
	for i := 0; i < numPuzzles; i++ {
		prompts = append(prompts, fmt.Sprintf("Create chess puzzle #%d. Theme: %s", i+1, themes[i%len(themes)]))
	}
	input := strings.Join(prompts, "\n")

	stage1File := filepath.Join(tmpDir, "stage1.jsonl")
	prepared := cheapshotPipe(t, input, "prepare",
		"-p", "openai",
		"-m", model,
		"--system-file", filepath.Join(promptsDir, "chess-generate.txt"),
		"--json-schema", filepath.Join(schemasDir, "chess-puzzle.json"),
	)
	if prepared == "" {
		t.Fatal("prepare produced no output")
	}

	cheapshotPipe(t, prepared, "run", "-p", "openai", "-i", "-", "-o", stage1File)

	// Extract and validate Stage 1 output
	stage1Extracted := cheapshot(t, "extract", "-i", stage1File, "--json")
	puzzles := parseJSONLines(t, stage1Extracted)
	if len(puzzles) != numPuzzles {
		t.Fatalf("expected %d puzzles, got %d", numPuzzles, len(puzzles))
	}

	for i, p := range puzzles {
		t.Logf("Puzzle %d: theme=%s difficulty=%v", i+1, p["theme"], p["difficulty"])
		assertFieldPresent(t, p, i, "fen")
		assertFieldPresent(t, p, i, "difficulty")
		assertFieldPresent(t, p, i, "theme")
		assertFieldPresent(t, p, i, "hint")
		assertFieldPresent(t, p, i, "solution")

		diff, ok := p["difficulty"].(float64)
		if !ok {
			t.Errorf("puzzle %d: difficulty is not a number: %v", i+1, p["difficulty"])
		} else if diff < 1 || diff > 5 {
			t.Errorf("puzzle %d: difficulty %v out of range [1,5]", i+1, diff)
		}
	}

	// Stage 2: Rate puzzles with structured schema
	t.Log("Stage 2: Rating puzzles...")
	stage2File := filepath.Join(tmpDir, "stage2.jsonl")
	rateInput := cheapshotPipe(t, stage1Extracted, "prepare",
		"-p", "openai",
		"-m", model,
		"--system-file", filepath.Join(promptsDir, "chess-rate.txt"),
		"--template", "FEN: {{.fen}}\nHint: {{.hint}}\nSolution: {{.solution}}",
		"--json-schema", filepath.Join(schemasDir, "chess-rating.json"),
	)
	cheapshotPipe(t, rateInput, "run", "-p", "openai", "-i", "-", "-o", stage2File)

	stage2Extracted := cheapshot(t, "extract", "-i", stage2File, "--json")
	ratings := parseJSONLines(t, stage2Extracted)
	if len(ratings) != numPuzzles {
		t.Fatalf("expected %d ratings, got %d", numPuzzles, len(ratings))
	}

	for i, r := range ratings {
		t.Logf("Rating %d: correctness=%v creativity=%v", i+1, r["correctness"], r["creativity"])
		assertFieldPresent(t, r, i, "correctness")
		assertFieldPresent(t, r, i, "creativity")
		assertFieldPresent(t, r, i, "verdict")

		for _, field := range []string{"correctness", "creativity"} {
			score, ok := r[field].(float64)
			if !ok {
				t.Errorf("rating %d: %s is not a number: %v", i+1, field, r[field])
			} else if score < 0 || score > 10 {
				t.Errorf("rating %d: %s=%v out of range [0,10]", i+1, field, score)
			}
		}
	}

	// Stage 3: Solve puzzles with structured schema
	t.Log("Stage 3: Solving puzzles...")
	stage3File := filepath.Join(tmpDir, "stage3.jsonl")
	solveInput := cheapshotPipe(t, stage1Extracted, "prepare",
		"-p", "openai",
		"-m", model,
		"--system-file", filepath.Join(promptsDir, "chess-solve.txt"),
		"--template", "FEN: {{.fen}}\nHint: {{.hint}}",
		"--json-schema", filepath.Join(schemasDir, "chess-solution.json"),
	)
	cheapshotPipe(t, solveInput, "run", "-p", "openai", "-i", "-", "-o", stage3File)

	stage3Extracted := cheapshot(t, "extract", "-i", stage3File, "--json")
	solutions := parseJSONLines(t, stage3Extracted)
	if len(solutions) != numPuzzles {
		t.Fatalf("expected %d solutions, got %d", numPuzzles, len(solutions))
	}

	for i, s := range solutions {
		t.Logf("Solution %d: valid=%v analysis=%v", i+1, s["is_valid"], s["analysis"])
		assertFieldPresent(t, s, i, "analysis")
		assertFieldPresent(t, s, i, "solution")
		assertFieldPresent(t, s, i, "is_valid")

		if _, ok := s["is_valid"].(bool); !ok {
			t.Errorf("solution %d: is_valid is not a bool: %v (%T)", i+1, s["is_valid"], s["is_valid"])
		}
	}

	t.Logf("Pipeline complete: %d puzzles generated, rated, and solved", numPuzzles)
}

var themes = []string{"fork", "pin", "back-rank mate"}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

func parseJSONLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	var result []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(s)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid JSON line: %s\nerror: %v", line, err)
		}
		result = append(result, obj)
	}
	return result
}

func assertFieldPresent(t *testing.T, obj map[string]any, idx int, field string) {
	t.Helper()
	val, ok := obj[field]
	if !ok {
		t.Errorf("item %d: missing required field %q", idx+1, field)
		return
	}
	if s, ok := val.(string); ok && s == "" {
		t.Errorf("item %d: field %q is empty", idx+1, field)
	}
}
