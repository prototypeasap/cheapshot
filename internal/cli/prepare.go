package cli

import (
	"encoding/json"
	"fmt"
	"os"

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
  JSONL mode: Any field from the JSON object`,
		RunE: func(_ *cobra.Command, _ []string) error {
			provFormat, resolvedModel, err := config.ResolvePrepareConfig(providerFlag, model)
			if err != nil {
				return err
			}
			model = resolvedModel

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

			return prepare.Run(input, os.Stdout, prepare.Options{
				Provider:   provFormat,
				Model:      model,
				MaxTokens:  maxTokens,
				System:     system,
				Template:   template,
				FileInput:  fileInput,
				JSONMode:   jsonMode || schema != nil,
				JSONSchema: schema,
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

	return cmd
}
