package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type listOpts struct {
	namespace     string
	allNamespaces bool
	context       string
	outputFmt     string
}

var newK8sClientForList = k8s.NewClient
var listReleasesForList = k8s.ListReleases

func newListCmd() *cobra.Command {
	var opts listOpts

	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: "List releases tracked by ct inventory",
		Long:  "Lists release inventory ConfigMaps managed by ct and shows release name, namespace, and tracked resources count.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "namespace to search")
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	cmd.Flags().StringVar(&opts.context, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVarP(&opts.outputFmt, "output", "o", "", "output format: json or yaml (default: table)")

	return cmd
}

func init() {
	rootCmd.AddCommand(newListCmd())
}

func runList(cmd *cobra.Command, opts listOpts) error {
	client, err := newK8sClientForList(opts.context, opts.namespace)
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	releases, err := listReleasesForList(cmd.Context(), client, opts.namespace, opts.allNamespaces)
	if err != nil {
		return fmt.Errorf("listing releases: %w", err)
	}

	switch strings.ToLower(opts.outputFmt) {
	case "":
		return writeReleaseTable(cmd.OutOrStdout(), releases)
	case "json":
		return writeReleaseJSON(cmd.OutOrStdout(), releases)
	case "yaml":
		return writeReleaseYAML(cmd.OutOrStdout(), releases)
	default:
		return fmt.Errorf("unsupported output format: %s", opts.outputFmt)
	}
}

func writeReleaseTable(w io.Writer, releases []k8s.ReleaseInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tNAMESPACE\tRESOURCES"); err != nil {
		return err
	}
	for _, release := range releases {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\n", release.Name, release.Namespace, release.Resources); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeReleaseJSON(w io.Writer, releases []k8s.ReleaseInfo) error {
	data, err := json.MarshalIndent(releases, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return err
	}
	return nil
}

func writeReleaseYAML(w io.Writer, releases []k8s.ReleaseInfo) error {
	data, err := yaml.Marshal(releases)
	if err != nil {
		return fmt.Errorf("yaml marshal error: %w", err)
	}
	if _, err := fmt.Fprint(w, string(data)); err != nil {
		return err
	}
	return nil
}
