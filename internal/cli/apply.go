package cli

import (
	"context"
	"fmt"

	"github.com/cloudticon/ctts/internal/output"
	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
)

type applyOpts struct {
	templateOpts
	context         string
	createNamespace bool
}

var resolveSourceDirForApply = resolveSourceDir
var renderResourcesForApply = renderResources
var injectReleaseLabelsForApply = k8s.InjectReleaseLabels
var newK8sClientForApply = k8s.NewClient
var ensureNamespaceForApply = k8s.EnsureNamespace
var applyReleaseForApply = func(ctx context.Context, client *k8s.Client, namespace, releaseName string, resources []k8s.Resource) error {
	return client.ApplyRelease(ctx, namespace, releaseName, resources)
}

func newApplyCmd() *cobra.Command {
	var opts applyOpts

	cmd := &cobra.Command{
		Use:   "apply <name> <dir|repo>",
		Short: "Render and apply Kubernetes manifests to a cluster",
		Long:  "Bundles and executes main.ct from the given directory, then applies the resulting manifests via server-side apply. Injects release labels, tracks inventory, and prunes orphaned resources.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, args[0], args[1], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "target namespace for resources")
	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "path to values file (JSON or YAML, overrides auto-detect)")
	cmd.Flags().StringVarP(&opts.outputFmt, "output", "o", "", "output format: yaml or json (default: no output)")
	cmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "override values (e.g. --set replicas=5)")
	cmd.Flags().BoolVar(&opts.noCache, "no-cache", false, "skip cache, re-download remote source")
	cmd.Flags().StringVar(&opts.context, "context", "", "kubeconfig context to use")
	cmd.Flags().BoolVar(&opts.createNamespace, "create-namespace", false, "create namespace if it does not exist")

	return cmd
}

func init() {
	rootCmd.AddCommand(newApplyCmd())
}

func runApply(cmd *cobra.Command, releaseName, source string, opts applyOpts) error {
	resolvedDir, err := resolveSourceDirForApply(source, opts.noCache)
	if err != nil {
		return err
	}

	opts.templateOpts.releaseName = releaseName
	resources, err := renderResourcesForApply(resolvedDir, opts.templateOpts)
	if err != nil {
		return err
	}

	resources = injectReleaseLabelsForApply(resources, releaseName)

	client, err := newK8sClientForApply(opts.context, opts.namespace)
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	if err := ensureApplyNamespace(cmd.Context(), client, opts.namespace, opts.createNamespace); err != nil {
		return err
	}

	if err := applyReleaseForApply(cmd.Context(), client, opts.namespace, releaseName, resources); err != nil {
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

func ensureApplyNamespace(ctx context.Context, client *k8s.Client, namespace string, createNamespace bool) error {
	if !createNamespace || namespace == "" {
		return nil
	}
	if err := ensureNamespaceForApply(ctx, client, namespace); err != nil {
		return fmt.Errorf("ensuring namespace %q: %w", namespace, err)
	}
	return nil
}
