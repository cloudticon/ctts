package cli

import "github.com/spf13/cobra"

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ct",
	Short: "ct - Kubernetes manifest generator from TypeScript",
	Long:  "ct generates Kubernetes manifests from TypeScript definitions using a registration model.",
}

func init() {
	rootCmd.Version = version
}

func Execute() error {
	return rootCmd.Execute()
}
