package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/prototypeasap/cheapshot/internal/config"
	"github.com/prototypeasap/cheapshot/internal/store"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	var (
		providerFlag string
		jsonOutput   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tracked batches",
		RunE: func(_ *cobra.Command, _ []string) error {
			db, err := store.Open(config.DBPath())
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // best-effort cleanup

			batches, err := db.ListBatches(providerFlag)
			if err != nil {
				return err
			}

			if len(batches) == 0 {
				fmt.Fprintln(os.Stderr, "No batches found.")
				return nil
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				for _, b := range batches {
					_ = enc.Encode(map[string]any{
						"local_id":  b.ID,
						"batch_id":  b.RemoteBatchID.String,
						"provider":  b.Provider,
						"status":    b.Status,
						"requests":  b.RequestCount,
						"succeeded": b.SucceededCount,
						"failed":    b.FailedCount,
						"created":   b.CreatedAt.Format("2006-01-02 15:04"),
					})
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tPROVIDER\tSTATUS\tREQUESTS\tOK\tFAIL\tBATCH ID\tCREATED")
			for _, b := range batches {
				batchID := "-"
				if b.RemoteBatchID.Valid {
					batchID = b.RemoteBatchID.String
					if len(batchID) > 20 {
						batchID = batchID[:20] + "..."
					}
				}
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
					b.ID, b.Provider, b.Status, b.RequestCount,
					b.SucceededCount, b.FailedCount,
					batchID, b.CreatedAt.Format("2006-01-02 15:04"),
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Filter by provider")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}
