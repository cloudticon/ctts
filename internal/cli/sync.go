package cli

import (
	"fmt"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/spf13/cobra"
)

type syncOpts struct {
	update []string
}

func newSyncCmd() *cobra.Command {
	var opts syncOpts

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

			if cmd.Flags().Changed("update") {
				if err := packages.UpdatePackages(dir, opts.update); err != nil {
					return fmt.Errorf("update packages failed: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated packages in %s/\n", dir)
			}

			if err := scaffold.Sync(dir); err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced types in %s/\n", dir)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&opts.update, "update", nil, "update package(s) to latest ref (e.g. --update github.com/owner/repo)")

	return cmd
}

func init() {
	rootCmd.AddCommand(newSyncCmd())
}
