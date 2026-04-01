package cli

import (
	"fmt"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new ct project directory",
		Long:  "Creates a project folder with ct.ts, values.ts, tsconfig.json and stdlib type definitions for IDE autocomplete.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := scaffold.Init(dir); err != nil {
				return fmt.Errorf("init failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized ct project in %s/\n", dir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "ct", "project directory name")
	return cmd
}

func init() {
	rootCmd.AddCommand(newInitCmd())
}
