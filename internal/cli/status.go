package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/store"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	var (
		providerFlag string
		batchID      string
		watch        bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check batch status",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

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

			prov, err := config.ResolveProvider(providerFlag, 0)
			if err != nil {
				return err
			}

			for {
				status, err := prov.Status(ctx, batchID)
				if err != nil {
					return err
				}

				_ = db.UpdatePollStatus(batch.ID, status.Raw)

				if status.Terminal {
					_ = db.MarkCompleted(batch.ID, status.Succeeded, status.Failed)
				}

				out := map[string]any{
					"batch_id":  batchID,
					"provider":  prov.Name(),
					"status":    status.Raw,
					"succeeded": status.Succeeded,
					"failed":    status.Failed,
					"total":     status.Total,
					"terminal":  status.Terminal,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(out)

				if !watch || status.Terminal {
					return nil
				}

				fmt.Fprintf(os.Stderr, "Polling again in 15s...\n")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(15 * time.Second):
				}
			}
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider: openai or anthropic")
	cmd.Flags().StringVarP(&batchID, "batch", "b", "", "Remote batch ID")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Poll until terminal state")
	_ = cmd.MarkFlagRequired("batch")

	return cmd
}
