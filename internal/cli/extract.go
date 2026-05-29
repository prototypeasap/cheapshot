package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/prototypeasap/cheapshot/internal/jsonl"
	"github.com/spf13/cobra"
)

func NewExtractCmd() *cobra.Command {
	var (
		inputFile string
		field     string
		withID    bool
		jsonParse bool
	)

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract text content from batch results",
		Long: `Pull the response text out of provider-native result JSONL.
Auto-detects OpenAI vs Anthropic format. One line of output per result.

This is the key plumbing command for daisy-chaining batches:
  cheapshot run ... -o stage1.jsonl
  cheapshot extract -i stage1.jsonl | cheapshot prepare ... | cheapshot run ...

Use --with-id to preserve custom_id as JSONL: {"id": "...", "text": "..."}
Use --json to parse JSON responses and promote fields to top-level JSONL keys.
  This is the key to pipeline contracts: structured output from stage N
  becomes named fields for stage N+1 templates.
Use --field to extract a specific JSON field from the response instead of raw text.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			var input *os.File
			if inputFile == "" || inputFile == "-" {
				input = os.Stdin
			} else {
				f, err := os.Open(inputFile)
				if err != nil {
					return err
				}
				defer f.Close() //nolint:errcheck // best-effort cleanup
				input = f
			}

			scanner := jsonl.NewScanner(input)
			writer := bufio.NewWriter(os.Stdout)
			defer func() { _ = writer.Flush() }()

			for scanner.Scan() {
				line := scanner.Bytes()
				if len(bytes.TrimSpace(line)) == 0 {
					continue
				}

				customID, text, err := extractContent(line)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
					continue
				}

				if field != "" {
					text = extractField(text, field)
				}

				switch {
				case jsonParse:
					out := parseStructured(customID, text)
					b, _ := json.Marshal(out)
					_, _ = writer.Write(b)
				case withID:
					out := map[string]string{"id": customID, "text": text}
					b, _ := json.Marshal(out)
					_, _ = writer.Write(b)
				default:
					compact := compactLine(text)
					_, _ = writer.WriteString(compact)
				}
				_, _ = writer.WriteString("\n")
			}
			return scanner.Err()
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "-", "Input results JSONL (default: stdin)")
	cmd.Flags().StringVar(&field, "field", "", "Extract a specific JSON field from the response text")
	cmd.Flags().BoolVar(&withID, "with-id", false, "Output as JSONL with custom_id preserved: {\"id\": \"...\", \"text\": \"...\"}")
	cmd.Flags().BoolVar(&jsonParse, "json", false, "Parse JSON responses and promote fields to top-level JSONL")

	return cmd
}

func extractContent(line []byte) (customID, text string, err error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	if cid, ok := raw["custom_id"]; ok {
		_ = json.Unmarshal(cid, &customID)
	}

	if resp, ok := raw["response"]; ok {
		text, err := extractOpenAI(resp)
		if err == nil {
			return customID, text, nil
		}
	}

	if result, ok := raw["result"]; ok {
		text, err := extractAnthropic(result)
		if err == nil {
			return customID, text, nil
		}
	}

	return customID, "", fmt.Errorf("could not extract content from line (custom_id=%s)", customID)
}

func extractOpenAI(resp json.RawMessage) (string, error) {
	var r struct {
		Body struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"body"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return "", err
	}
	if len(r.Body.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return r.Body.Choices[0].Message.Content, nil
}

func extractAnthropic(result json.RawMessage) (string, error) {
	var r struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return "", err
	}
	if r.Type != "succeeded" {
		return "", fmt.Errorf("result type: %s", r.Type)
	}
	for _, c := range r.Message.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
		if c.Type == "tool_use" && c.Input != nil {
			return string(c.Input), nil
		}
	}
	return "", fmt.Errorf("no text content")
}

func compactLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	var buf bytes.Buffer
	if json.Compact(&buf, []byte(s)) == nil {
		return buf.String()
	}
	return strings.Join(strings.Fields(s), " ")
}

func parseStructured(customID, text string) map[string]any {
	out := map[string]any{"id": customID}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		out["text"] = text
		return out
	}

	for k, v := range parsed {
		if k == "id" {
			out["_"+k] = v
		} else {
			out[k] = v
		}
	}
	return out
}

func extractField(text, field string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return text
	}
	if val, ok := obj[field]; ok {
		switch v := val.(type) {
		case string:
			return v
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	return text
}
