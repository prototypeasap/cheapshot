package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/prepare"
	"github.com/spf13/cobra"
)

func NewPrepareCmd() *cobra.Command {
	var (
		providerFlag string
		model        string
		maxTokens    int
		system       string
		systemFile   string
		template     string
		templateFile string
		fileInput    bool
		inputFile    string
		jsonMode     bool
		jsonSchema   string
		extraBody    []string
		temperature  float64
	)

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Generate provider-native JSONL from simple input",
		Long: `Turn plain text, file paths, or structured JSONL into provider-native
batch request format. Output goes to stdout for piping into 'cheapshot run'.

Modes:
  Line mode (default):  Each line of input becomes one request.
  File mode (--file-input): Each line is a file path; file content is injected.
  JSONL mode:           If input lines are JSON, fields are available in templates.

Templates use {{.FieldName}} syntax. Available fields:
  Line mode:  {{.text}}, {{.Content}}
  File mode:  {{.Name}}, {{.Path}}, {{.Content}}
  JSONL mode: Any field from the JSON object

Extra body fields are merged into the request body with --extra-body KEY=JSON.
Per-line overrides via "extra_body" in JSONL input take highest precedence.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pc, err := config.ResolvePrepareConfig(providerFlag, model)
			if err != nil {
				return err
			}

			if systemFile != "" {
				data, err := os.ReadFile(systemFile)
				if err != nil {
					return fmt.Errorf("reading system file: %w", err)
				}
				system = string(data)
			}

			if templateFile != "" {
				data, err := os.ReadFile(templateFile)
				if err != nil {
					return fmt.Errorf("reading template file: %w", err)
				}
				template = string(data)
			}

			var input *os.File
			if inputFile != "" && inputFile != "-" {
				f, err := os.Open(inputFile)
				if err != nil {
					return err
				}
				defer f.Close() //nolint:errcheck // best-effort cleanup
				input = f
			} else {
				input = os.Stdin
			}

			var schema json.RawMessage
			if jsonSchema != "" {
				data, err := os.ReadFile(jsonSchema)
				if err != nil {
					return fmt.Errorf("reading JSON schema: %w", err)
				}
				schema = json.RawMessage(data)
			}

			mergedExtra := make(map[string]any)
			if pc.ExtraBody != nil {
				prepare.DeepMerge(mergedExtra, pc.ExtraBody)
			}
			cliExtra, err := parseExtraBody(extraBody)
			if err != nil {
				return err
			}
			prepare.DeepMerge(mergedExtra, cliExtra)

			var temp *float64
			if cmd.Flags().Changed("temperature") {
				temp = &temperature
			}

			return prepare.Run(input, os.Stdout, prepare.Options{
				Provider:    pc.Format,
				Model:       pc.Model,
				MaxTokens:   maxTokens,
				System:      system,
				Template:    template,
				FileInput:   fileInput,
				JSONMode:    jsonMode || schema != nil,
				JSONSchema:  schema,
				ExtraBody:   mergedExtra,
				Temperature: temp,
			})
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider: openai or anthropic")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Model name (required)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max tokens (Anthropic requires this; defaults to 1024)")
	cmd.Flags().StringVarP(&system, "system", "s", "", "System prompt")
	cmd.Flags().StringVar(&systemFile, "system-file", "", "Read system prompt from file")
	cmd.Flags().StringVarP(&template, "template", "t", "", "Prompt template with {{.Field}} placeholders")
	cmd.Flags().StringVar(&templateFile, "template-file", "", "Read template from file")
	cmd.Flags().BoolVar(&fileInput, "file-input", false, "Treat each input line as a file path")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file (default: stdin)")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Request JSON output from the model")
	cmd.Flags().StringVar(&jsonSchema, "json-schema", "", "Path to JSON Schema file for structured output")
	cmd.Flags().StringArrayVar(&extraBody, "extra-body", nil, "Extra request body fields (KEY=JSON, repeatable)")
	cmd.Flags().Float64Var(&temperature, "temperature", 0, "Sampling temperature")

	return cmd
}

func parseExtraBody(args []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, arg := range args {
		idx := strings.IndexByte(arg, '=')
		if idx < 0 {
			return nil, fmt.Errorf("--extra-body %q: expected KEY=JSON format", arg)
		}
		key := arg[:idx]
		valStr := arg[idx+1:]

		var val any
		if err := json.Unmarshal([]byte(valStr), &val); err != nil {
			val = valStr
		}
		result[key] = val
	}
	return result, nil
}
