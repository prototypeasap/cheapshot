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

func NewCancelCmd() *cobra.Command {
	var (
		providerFlag string
		batchID      string
	)

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a running batch",
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

			if err := prov.Cancel(ctx, batchID); err != nil {
				return err
			}

			_ = db.MarkCancelled(batch.ID)
			fmt.Fprintf(os.Stderr, "Batch %s cancelled.\n", batchID)

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"batch_id":  batchID,
				"provider":  prov.Name(),
				"cancelled": true,
			})
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Provider: openai or anthropic")
	cmd.Flags().StringVarP(&batchID, "batch", "b", "", "Remote batch ID")
	_ = cmd.MarkFlagRequired("batch")

	return cmd
}
