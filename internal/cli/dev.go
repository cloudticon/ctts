package cli

import (
	"fmt"
	"os"

	"github.com/cloudticon/ctts/internal/dev"
	"github.com/spf13/cobra"
)

type devOpts struct {
	envFile string
	context string
}

var runDevMode = dev.Run

func newDevCmd() *cobra.Command {
	var opts devOpts

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Start development mode from dev.ct configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDev(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.envFile, "env-file", ".env", "path to .env file (empty to skip)")
	cmd.Flags().StringVar(&opts.context, "context", "", "kubeconfig context")

	return cmd
}

func init() {
	rootCmd.AddCommand(newDevCmd())
}

func runDev(cmd *cobra.Command, opts devOpts) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}

	if err := runDevMode(cmd.Context(), dev.RunOpts{
		Dir:     dir,
		EnvFile: opts.envFile,
		KubeCtx: opts.context,
		Stdin:   cmd.InOrStdin(),
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
	}); err != nil {
		return fmt.Errorf("running dev mode: %w", err)
	}

	return nil
}
