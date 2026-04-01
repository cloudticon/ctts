package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudticon/ctts/internal/engine"
	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/cloudticon/ctts/internal/output"
	"github.com/cloudticon/ctts/internal/packages"
	"github.com/spf13/cobra"
)

type templateOpts struct {
	namespace  string
	valuesFile string
	outputFmt  string
	setValues  []string
}

func newTemplateCmd() *cobra.Command {
	var opts templateOpts

	cmd := &cobra.Command{
		Use:   "template <dir>",
		Short: "Render Kubernetes manifests from a ct project",
		Long:  "Bundles and executes ct.ts from the given directory, producing Kubernetes manifests on stdout.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTemplate(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "default namespace for resources")
	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "path to values.ts (overrides auto-detect)")
	cmd.Flags().StringVarP(&opts.outputFmt, "output", "o", "yaml", "output format: yaml or json")
	cmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "override values (e.g. --set replicas=5)")

	return cmd
}

func init() {
	rootCmd.AddCommand(newTemplateCmd())
}

func runTemplate(cmd *cobra.Command, dir string, opts templateOpts) error {
	entryPoint := filepath.Join(dir, "ct.ts")
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		return fmt.Errorf("entry point not found: %s", entryPoint)
	}

	cttsDir := filepath.Join(dir, ".ctts")
	if _, err := os.Stat(cttsDir); os.IsNotExist(err) {
		return fmt.Errorf(".ctts/ directory not found in %s — run 'ct init' first", dir)
	}

	if err := packages.SyncPackages(dir); err != nil {
		return fmt.Errorf("auto-installing packages: %w", err)
	}

	tr := engine.NewTranspiler(k8s.Stdlib, dir)

	valuesPath := resolveValuesPath(dir, opts.valuesFile)
	values, err := loadValuesIfPresent(tr, valuesPath, opts.setValues)
	if err != nil {
		return err
	}

	jsCode, err := tr.Bundle(entryPoint)
	if err != nil {
		return fmt.Errorf("bundle failed: %w", err)
	}

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    jsCode,
		Values:    values,
		Namespace: opts.namespace,
	})
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	out, err := output.Serialize(toOutputResources(resources), opts.outputFmt)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

func resolveValuesPath(dir, explicit string) string {
	if explicit != "" {
		return explicit
	}
	candidate := filepath.Join(dir, "values.ts")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func loadValuesIfPresent(tr *engine.Transpiler, valuesPath string, setOverrides []string) (map[string]interface{}, error) {
	if valuesPath == "" && len(setOverrides) == 0 {
		return nil, nil
	}
	if valuesPath == "" {
		return nil, fmt.Errorf("--set provided but no values file found")
	}
	values, err := engine.LoadValues(tr, valuesPath, setOverrides)
	if err != nil {
		return nil, fmt.Errorf("loading values from %s: %w", valuesPath, err)
	}
	return values, nil
}

func toOutputResources(resources []engine.Resource) []output.Resource {
	result := make([]output.Resource, len(resources))
	for i, r := range resources {
		result[i] = output.Resource(r)
	}
	return result
}
