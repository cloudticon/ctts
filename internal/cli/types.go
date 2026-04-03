package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudticon/ctts/pkg/cache"
	"github.com/cloudticon/ctts/pkg/engine"
	"github.com/cloudticon/ctts/pkg/packages"
	"github.com/spf13/cobra"
)

type typesOpts struct {
	output   string
	operator bool
}

func newTypesCmd() *cobra.Command {
	var opts typesOpts

	cmd := &cobra.Command{
		Use:   "types [dir]",
		Short: "Generate TypeScript type definitions for a ct project",
		Long:  "Generates globals.d.ts and values.d.ts for IDE support. Outputs the directory path containing the generated files.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return runTypes(cmd, dir, opts)
		},
	}

	cmd.Flags().StringVar(&opts.output, "output", "", "output directory (default ~/.ct/types/<project-hash>)")
	cmd.Flags().BoolVar(&opts.operator, "operator", false, "include operator globals (getStatus, setStatus, fetch, log, Env)")

	return cmd
}

func init() {
	rootCmd.AddCommand(newTypesCmd())
}

func runTypes(cmd *cobra.Command, dir string, opts typesOpts) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}

	if entryPoint := findEntryPoint(absDir); entryPoint != "" {
		if err := resolveURLImports(entryPoint); err != nil {
			return fmt.Errorf("resolving imports: %w", err)
		}
	}

	outDir := opts.output
	if outDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		outDir = filepath.Join(home, ".ct", "types", projectHash(absDir))
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	valuesPath := resolveValuesPath(absDir, "")
	var values map[string]interface{}
	if valuesPath != "" {
		values, err = engine.LoadValuesFile(valuesPath, nil)
		if err != nil {
			return fmt.Errorf("loading values: %w", err)
		}
	}

	if err := os.WriteFile(filepath.Join(outDir, "values.d.ts"), []byte(generateValuesDts(values)), 0o644); err != nil {
		return fmt.Errorf("writing values.d.ts: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "globals.d.ts"), []byte(generateGlobalsDts(opts.operator)), 0o644); err != nil {
		return fmt.Errorf("writing globals.d.ts: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), outDir)
	return nil
}

func findEntryPoint(dir string) string {
	for _, name := range []string{"main.ct", "operator.ct"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func resolveURLImports(entryPath string) error {
	imports, err := packages.ParseImports(entryPath)
	if err != nil {
		return err
	}

	for _, imp := range imports {
		rawURL, ok := importToURL(imp.Path)
		if !ok {
			continue
		}
		if _, err := cache.Resolve(rawURL); err != nil {
			return fmt.Errorf("resolving %s: %w", imp.Path, err)
		}
	}
	return nil
}

func importToURL(importPath string) (string, bool) {
	if packages.IsURLImport(importPath) {
		return importPath, true
	}
	if !packages.IsGitPackage(importPath) {
		return "", false
	}
	pkgWithVersion, _ := packages.SplitPackagePath(importPath)
	pkg, version := packages.SplitPackageVersion(pkgWithVersion)
	url := "https://" + pkg
	if version != "" {
		url += "@" + version
	}
	return url, true
}

func projectHash(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(h[:])[:12]
}

func generateValuesDts(values map[string]interface{}) string {
	if len(values) == 0 {
		return "interface CtValues {}\n"
	}
	var buf strings.Builder
	buf.WriteString("interface CtValues {\n")
	writeObjectFields(&buf, values, "  ")
	buf.WriteString("}\n")
	return buf.String()
}

func writeObjectFields(buf *strings.Builder, obj map[string]interface{}, indent string) {
	for _, k := range sortedKeys(obj) {
		fmt.Fprintf(buf, "%s%s: %s;\n", indent, k, inferTSType(obj[k], indent))
	}
}

func inferTSType(v interface{}, indent string) string {
	switch val := v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int64, float64, int:
		return "number"
	case map[string]interface{}:
		var buf strings.Builder
		buf.WriteString("{\n")
		writeObjectFields(&buf, val, indent+"  ")
		buf.WriteString(indent + "}")
		return buf.String()
	case []interface{}:
		if len(val) == 0 {
			return "any[]"
		}
		elemType := inferTSType(val[0], indent)
		return elemType + "[]"
	default:
		return "any"
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func generateGlobalsDts(operator bool) string {
	var buf strings.Builder
	buf.WriteString("/// <reference path=\"./values.d.ts\" />\n\n")
	buf.WriteString("declare const Values: CtValues;\n")

	if operator {
		buf.WriteString("\ndeclare function getStatus<T>(resource: T): any;\n")
		buf.WriteString("declare function setStatus(cr: any, status: any): void;\n")
		buf.WriteString("declare function fetch(url: string, opts?: {\n")
		buf.WriteString("  method?: string;\n")
		buf.WriteString("  headers?: Record<string, string>;\n")
		buf.WriteString("  body?: string;\n")
		buf.WriteString("  timeout?: string;\n")
		buf.WriteString("}): { status: number; body: string; headers: Record<string, string> };\n")
		buf.WriteString("declare const log: {\n")
		buf.WriteString("  info(...args: any[]): void;\n")
		buf.WriteString("  warn(...args: any[]): void;\n")
		buf.WriteString("  error(...args: any[]): void;\n")
		buf.WriteString("};\n")
		buf.WriteString("declare const Env: Record<string, string>;\n")
	}

	return buf.String()
}
