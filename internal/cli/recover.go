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

func NewRecoverCmd() *cobra.Command {
	var providerFlag string

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Resume and reconcile non-terminal batches",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			db, err := store.Open(config.DBPath())
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // best-effort cleanup

			batches, err := db.ListNonTerminal(providerFlag)
			if err != nil {
				return err
			}

			if len(batches) == 0 {
				fmt.Fprintln(os.Stderr, "No non-terminal batches found.")
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"recovered": 0, "batches": []any{}})
			}

			fmt.Fprintf(os.Stderr, "Found %d non-terminal batch(es)\n", len(batches))

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			var results []map[string]any

			for _, batch := range batches {
				if !batch.RemoteBatchID.Valid || batch.RemoteBatchID.String == "" {
					fmt.Fprintf(os.Stderr, "  Batch #%d: no remote ID, marking failed\n", batch.ID)
					_ = db.MarkFailed(batch.ID, "no remote batch ID — submission was incomplete")
					results = append(results, map[string]any{
						"local_id": batch.ID,
						"status":   "failed",
						"reason":   "no remote batch ID",
					})
					continue
				}

				prov, err := config.ResolveProvider(batch.Provider, 0)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  Batch #%d (%s): cannot resolve provider: %v\n", batch.ID, batch.RemoteBatchID.String, err)
					results = append(results, map[string]any{
						"local_id": batch.ID,
						"batch_id": batch.RemoteBatchID.String,
						"status":   "error",
						"reason":   err.Error(),
					})
					continue
				}

				status, err := prov.Status(ctx, batch.RemoteBatchID.String)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  Batch #%d (%s): poll error: %v\n", batch.ID, batch.RemoteBatchID.String, err)
					results = append(results, map[string]any{
						"local_id": batch.ID,
						"batch_id": batch.RemoteBatchID.String,
						"status":   "error",
						"reason":   err.Error(),
					})
					continue
				}

				_ = db.UpdatePollStatus(batch.ID, status.Raw)

				entry := map[string]any{
					"local_id":  batch.ID,
					"batch_id":  batch.RemoteBatchID.String,
					"provider":  batch.Provider,
					"status":    status.Raw,
					"terminal":  status.Terminal,
					"succeeded": status.Succeeded,
					"failed":    status.Failed,
					"total":     status.Total,
				}

				if status.Terminal {
					_ = db.MarkCompleted(batch.ID, status.Succeeded, status.Failed)
					fmt.Fprintf(os.Stderr, "  Batch #%d (%s): %s — %d succeeded, %d failed\n",
						batch.ID, batch.RemoteBatchID.String, status.Raw, status.Succeeded, status.Failed)
				} else {
					fmt.Fprintf(os.Stderr, "  Batch #%d (%s): still %s (%d/%d)\n",
						batch.ID, batch.RemoteBatchID.String, status.Raw, status.Succeeded+status.Failed, status.Total)
				}
				results = append(results, entry)
			}

			return enc.Encode(map[string]any{"recovered": len(results), "batches": results})
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Filter by provider")

	return cmd
}
