package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/runner"
	"github.com/prototypeasap/cheapshot/internal/store"
	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	var (
		providerFlag string
		inputFile    string
		outputFile   string
		endpoint     string
		window       string
		pollInterval time.Duration
		modeFlag     string
		concurrency  int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Submit, poll, and download results in one command",
		Long: `Run a batch or direct LLM job end-to-end.

Modes:
  batch (default):  Upload to provider batch API, poll, download. 50% off.
  direct:           Send requests concurrently via chat API. No discount, instant results.

The mode is set via --mode, provider config profile, or defaults to batch.
Use direct mode for small jobs or providers without batch APIs (e.g. DeepSeek).

The request body from prepare is forwarded verbatim — -p only resolves
transport (base_url, api_key, format, mode, concurrency). To change the
model, re-run prepare.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			cfg, err := config.ResolveRunConfig(providerFlag, modeFlag, concurrency)
			if err != nil {
				return err
			}

			inputPath, displayName, cleanup, err := resolveInput(inputFile)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}

			var r runner.Runner

			switch cfg.Mode {
			case "direct":
				r = runner.NewDirectRunner(cfg.APIKey, cfg.BaseURL, cfg.Format, cfg.Concurrency)
			default:
				input, err := os.ReadFile(inputPath)
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				prov, err := config.ResolveProvider(providerFlag)
				if err != nil {
					return err
				}
				if err := prov.Validate(input); err != nil {
					return fmt.Errorf("validation failed: %w", err)
				}
				db, err := store.Open(config.DBPath())
				if err != nil {
					return err
				}
				defer db.Close() //nolint:errcheck // best-effort cleanup
				r = runner.NewBatchRunner(prov, db, displayName)
			}

			result, err := r.Run(ctx, inputPath, outputFile, runner.RunOpts{
				Endpoint:         endpoint,
				CompletionWindow: window,
				PollInterval:     int(pollInterval.Seconds()),
			})
			if err != nil {
				return err
			}
			if result == nil {
				return nil
			}

			return runner.PrintRunResult(cfg.Name, outputFile, result)
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider name (openai, anthropic, or a config profile)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSONL file (or - for stdin)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output JSONL file path")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "OpenAI endpoint")
	cmd.Flags().StringVar(&window, "completion-window", "", "Completion window")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 15*time.Second, "Poll interval (batch mode)")
	cmd.Flags().StringVar(&modeFlag, "mode", "", "Execution mode: batch or direct")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 0, "Concurrent requests (direct mode, default 10)")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func resolveInput(inputFile string) (path, displayName string, cleanup func(), err error) {
	if inputFile != "-" {
		abs, _ := filepath.Abs(inputFile)
		return abs, abs, nil, nil
	}
	tmpFile, err := os.CreateTemp("", "cheapshot-input-*.jsonl")
	if err != nil {
		return "", "", nil, err
	}
	if _, err := tmpFile.ReadFrom(os.Stdin); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", "", nil, err
	}
	_ = tmpFile.Close()
	return tmpFile.Name(), "(stdin)", func() { _ = os.Remove(tmpFile.Name()) }, nil
}
