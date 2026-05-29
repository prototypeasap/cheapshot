package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/provider"
	"github.com/prototypeasap/cheapshot/internal/store"
	"github.com/spf13/cobra"
)

func NewSubmitCmd() *cobra.Command {
	var (
		providerFlag string
		inputFile    string
		endpoint     string
		window       string
	)

	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a batch to OpenAI or Anthropic",
		Long: `Submit JSONL input as a batch job. Returns immediately with the batch ID.
Use 'cheapshot status' to poll and 'cheapshot results' to download.

This is the non-blocking counterpart to 'cheapshot run'. Ideal for agents
that want to submit work and check back later.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			prov, err := config.ResolveProvider(providerFlag, 0)
			if err != nil {
				return err
			}

			var inputPath string
			if inputFile == "-" {
				tmpFile, err := os.CreateTemp("", "cheapshot-submit-*.jsonl")
				if err != nil {
					return err
				}
				defer os.Remove(tmpFile.Name()) //nolint:errcheck // best-effort cleanup
				if _, err := tmpFile.ReadFrom(os.Stdin); err != nil {
					_ = tmpFile.Close()
					return err
				}
				_ = tmpFile.Close()
				inputPath = tmpFile.Name()
			} else {
				inputPath = inputFile
			}

			input, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			if err := prov.Validate(input); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			db, err := store.Open(config.DBPath())
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // best-effort cleanup

			absInput, _ := filepath.Abs(inputPath)

			fmt.Fprintf(os.Stderr, "Submitting to %s...\n", prov.Name())

			result, err := prov.Submit(ctx, absInput, provider.SubmitOpts{
				Endpoint:         endpoint,
				CompletionWindow: window,
			})
			if err != nil {
				return err
			}

			batchDBID, err := db.CreateBatch(prov.Name(), absInput, result.RequestCount, endpoint)
			if err != nil {
				return err
			}

			if result.RemoteFileID != "" {
				_ = db.MarkUploaded(batchDBID, result.RemoteFileID)
			}
			_ = db.MarkSubmitted(batchDBID, result.RemoteBatchID)

			out := map[string]any{
				"batch_id":      result.RemoteBatchID,
				"provider":      prov.Name(),
				"request_count": result.RequestCount,
				"local_id":      batchDBID,
			}
			if result.RemoteFileID != "" {
				out["file_id"] = result.RemoteFileID
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider: openai or anthropic (auto-detected from API keys)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSONL file (or - for stdin)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "OpenAI endpoint (default: /v1/chat/completions)")
	cmd.Flags().StringVar(&window, "completion-window", "", "Completion window (default: 24h)")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}
