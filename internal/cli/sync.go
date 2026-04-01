package cli

import (
	"fmt"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [dir]",
		Short: "Regenerate types from values.ts and update stdlib types",
		Long:  "Regenerates .ctts/types/k8s/* stdlib types and .ctts/types/values.d.ts from values.ts. Run after editing values.ts or updating ct.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "ct"
			if len(args) > 0 {
				dir = args[0]
			}
			if err := scaffold.Sync(dir); err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced types in %s/\n", dir)
			return nil
		},
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(newSyncCmd())
}
