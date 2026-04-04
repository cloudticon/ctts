package cli

import (
	"fmt"

	"github.com/cloudticon/ctts/internal/output"
	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
)

type applyOpts struct {
	templateOpts
	context string
}

func newApplyCmd() *cobra.Command {
	var opts applyOpts

	cmd := &cobra.Command{
		Use:   "apply <dir>",
		Short: "Render and apply Kubernetes manifests to a cluster",
		Long:  "Bundles and executes main.ct from the given directory, then applies the resulting manifests via server-side apply.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "target namespace for resources")
	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "path to values file (JSON or YAML, overrides auto-detect)")
	cmd.Flags().StringVarP(&opts.outputFmt, "output", "o", "", "output format: yaml or json (default: no output)")
	cmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "override values (e.g. --set replicas=5)")
	cmd.Flags().StringVar(&opts.context, "context", "", "kubeconfig context to use")

	return cmd
}

func init() {
	rootCmd.AddCommand(newApplyCmd())
}

func runApply(cmd *cobra.Command, dir string, opts applyOpts) error {
	resources, err := renderResources(dir, opts.templateOpts)
	if err != nil {
		return err
	}

	client, err := k8s.NewClient(opts.context, opts.namespace)
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	if err := client.Apply(cmd.Context(), resources); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	if opts.outputFmt != "" {
		out, err := output.Serialize(toOutputResources(resources), opts.outputFmt)
		if err != nil {
			return fmt.Errorf("serialization failed: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
	}

	return nil
}
