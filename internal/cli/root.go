package cli

import "github.com/spf13/cobra"

var version = "dev"

var rootCmd = &cobra.Command{
	Use:          "ct",
	Short:        "ct - Kubernetes manifest generator",
	Long:         "ct generates Kubernetes manifests from .ct definitions using a registration model.",
	SilenceUsage: true,
}

func init() {
	rootCmd.Version = version
}

func Execute() error {
	return rootCmd.Execute()
}
