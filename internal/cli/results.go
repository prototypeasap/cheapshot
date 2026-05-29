package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/store"
	"github.com/spf13/cobra"
)

func NewResultsCmd() *cobra.Command {
	var (
		providerFlag string
		batchID      string
		outputFile   string
	)

	cmd := &cobra.Command{
		Use:   "results",
		Short: "Download batch results",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			prov, err := config.ResolveProvider(providerFlag)
			if err != nil {
				return err
			}

			db, err := store.Open(config.DBPath())
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // best-effort cleanup

			batch, err := db.GetBatchByRemoteID(batchID)
			if err != nil {
				return err
			}
			if batch == nil {
				return fmt.Errorf("batch %s not found in local database", batchID)
			}

			fmt.Fprintf(os.Stderr, "Downloading results...\n")

			summary, err := prov.Results(ctx, batchID, outputFile)
			if err != nil {
				return err
			}

			_ = db.MarkDownloaded(batch.ID, outputFile, summary.Succeeded, summary.Failed)

			out := map[string]any{
				"batch_id":    batchID,
				"provider":    prov.Name(),
				"output_file": summary.OutputPath,
				"succeeded":   summary.Succeeded,
				"failed":      summary.Failed,
				"total":       summary.Total,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(out)

			if summary.Failed > 0 && summary.Succeeded > 0 {
				return fmt.Errorf("partial failure: %d succeeded, %d failed", summary.Succeeded, summary.Failed)
			}
			if summary.Failed > 0 {
				return fmt.Errorf("all %d requests failed", summary.Failed)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider: openai or anthropic")
	cmd.Flags().StringVarP(&batchID, "batch", "b", "", "Remote batch ID")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output JSONL file path")
	_ = cmd.MarkFlagRequired("batch")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}
