package cli

import (
	"context"
	"fmt"

	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
)

type deleteOpts struct {
	namespace string
	context   string
}

var newK8sClientForDelete = k8s.NewClient
var loadInventoryForDelete = k8s.LoadInventory
var deleteInventoryForDelete = k8s.DeleteInventory
var deleteResourcesForDelete = func(ctx context.Context, client *k8s.Client, resources []k8s.ResourceRef) error {
	return client.Delete(ctx, resources)
}

func newDeleteCmd() *cobra.Command {
	var opts deleteOpts

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a release from cluster using inventory",
		Long:  "Loads release inventory from Kubernetes ConfigMap, deletes tracked resources, then removes the inventory ConfigMap.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "namespace that contains release inventory")
	cmd.Flags().StringVar(&opts.context, "context", "", "kubeconfig context to use")

	return cmd
}

func init() {
	rootCmd.AddCommand(newDeleteCmd())
}

func runDelete(cmd *cobra.Command, releaseName string, opts deleteOpts) error {
	client, err := newK8sClientForDelete(opts.context, opts.namespace)
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	resources, err := loadInventoryForDelete(cmd.Context(), client, opts.namespace, releaseName)
	if err != nil {
		return fmt.Errorf("loading inventory for release %q: %w", releaseName, err)
	}

	if err := deleteResourcesForDelete(cmd.Context(), client, resources); err != nil {
		return fmt.Errorf("deleting release resources: %w", err)
	}

	if err := deleteInventoryForDelete(cmd.Context(), client, opts.namespace, releaseName); err != nil {
		return fmt.Errorf("deleting release inventory: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "deleted release %s (%d resources)\n", releaseName, len(resources))
	return nil
}
