package scaffold

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

// GenerateValuesDts transpiles a values.ts file, executes it to obtain runtime
// values, infers TypeScript types from them, and writes a declare const Values
// declaration to outputPath.
func GenerateValuesDts(valuesPath, outputPath string) error {
	valuesObj, err := loadValuesObject(valuesPath)
	if err != nil {
		return fmt.Errorf("loading values from %s: %w", valuesPath, err)
	}

	dts := generateDtsContent(valuesObj)
	return os.WriteFile(outputPath, []byte(dts), 0o644)
}

func loadValuesObject(valuesPath string) (map[string]interface{}, error) {
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{valuesPath},
		Bundle:      true,
		Write:       false,
		Format:      api.FormatIIFE,
		GlobalName:  "__values_export",
		Platform:    api.PlatformNeutral,
		Loader: map[string]api.Loader{
			".ts": api.LoaderTS,
		},
	})
	if len(result.Errors) > 0 {
		msgs := make([]string, 0, len(result.Errors))
		for _, e := range result.Errors {
			msgs = append(msgs, e.Text)
		}
		return nil, fmt.Errorf("esbuild: %s", strings.Join(msgs, "; "))
	}
	if len(result.OutputFiles) == 0 {
		return nil, fmt.Errorf("esbuild produced no output")
	}

	vm := goja.New()
	jsCode := string(result.OutputFiles[0].Contents)
	if _, err := vm.RunString(jsCode); err != nil {
		return nil, fmt.Errorf("executing values JS: %w", err)
	}

	val := vm.Get("__values_export")
	if val == nil || goja.IsUndefined(val) {
		return nil, fmt.Errorf("no exports found in values module")
	}

	exports, ok := val.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("values module exports is not an object")
	}

	defaultExport, ok := exports["default"]
	if !ok {
		return nil, fmt.Errorf("values.ts has no default export")
	}

	valuesMap, ok := defaultExport.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("default export is not an object (got %T)", defaultExport)
	}

	return valuesMap, nil
}

func generateDtsContent(obj map[string]interface{}) string {
	var b strings.Builder
	b.WriteString("declare const Values: ")
	writeType(&b, obj, 0)
	b.WriteString(";\n")
	return b.String()
}

func writeType(b *strings.Builder, val interface{}, indent int) {
	switch v := val.(type) {
	case map[string]interface{}:
		writeObjectType(b, v, indent)
	case []interface{}:
		writeArrayType(b, v, indent)
	case string:
		b.WriteString("string")
	case float64:
		b.WriteString("number")
	case int64:
		b.WriteString("number")
	case bool:
		b.WriteString("boolean")
	default:
		b.WriteString("unknown")
	}
}

func writeObjectType(b *strings.Builder, obj map[string]interface{}, indent int) {
	if len(obj) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	keys := sortedKeys(obj)
	for _, key := range keys {
		writeIndent(b, indent+1)
		b.WriteString(key)
		b.WriteString(": ")
		writeType(b, obj[key], indent+1)
		b.WriteString(";\n")
	}
	writeIndent(b, indent)
	b.WriteString("}")
}

func writeArrayType(b *strings.Builder, arr []interface{}, indent int) {
	if len(arr) == 0 {
		b.WriteString("never[]")
		return
	}
	first := arr[0]
	if _, ok := first.(map[string]interface{}); ok {
		writeType(b, first, indent)
		b.WriteString("[]")
		return
	}
	writeType(b, first, indent)
	b.WriteString("[]")
}

func writeIndent(b *strings.Builder, level int) {
	for range level {
		b.WriteString("  ")
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
