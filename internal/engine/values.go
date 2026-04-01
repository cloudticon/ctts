package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

func LoadValues(transpiler *Transpiler, valuesPath string, setOverrides []string) (map[string]interface{}, error) {
	jsCode, err := transpiler.BundleValues(valuesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to transpile values: %w", err)
	}

	values, err := executeValuesJS(jsCode)
	if err != nil {
		return nil, fmt.Errorf("failed to execute values: %w", err)
	}

	if err := applySetOverrides(values, setOverrides); err != nil {
		return nil, fmt.Errorf("failed to apply --set overrides: %w", err)
	}

	return values, nil
}

func executeValuesJS(jsCode string) (map[string]interface{}, error) {
	vm := goja.New()

	if _, err := vm.RunString(jsCode); err != nil {
		return nil, fmt.Errorf("values JS execution error: %w", err)
	}

	val := vm.GlobalObject().Get("__values_export")
	if val == nil || goja.IsUndefined(val) {
		return nil, fmt.Errorf("values export not found")
	}

	obj := val.ToObject(vm)
	defaultExport := obj.Get("default")
	if defaultExport != nil && !goja.IsUndefined(defaultExport) {
		exported := defaultExport.Export()
		if m, ok := exported.(map[string]interface{}); ok {
			return m, nil
		}
		return nil, fmt.Errorf("default export is not an object, got %T", defaultExport.Export())
	}

	return nil, fmt.Errorf("values file has no default export")
}

func applySetOverrides(values map[string]interface{}, overrides []string) error {
	for _, override := range overrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --set format: %q (expected key=value)", override)
		}
		key, rawValue := parts[0], parts[1]
		setNestedValue(values, strings.Split(key, "."), parseValue(rawValue))
	}
	return nil
}

func setNestedValue(obj map[string]interface{}, keys []string, value interface{}) {
	for i, k := range keys {
		if i == len(keys)-1 {
			obj[k] = value
			return
		}
		next, ok := obj[k].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			obj[k] = next
		}
		obj = next
	}
}

func parseValue(s string) interface{} {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if s == "null" {
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
