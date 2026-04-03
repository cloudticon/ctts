package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadValuesFile(path string, setOverrides []string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading values file %s: %w", path, err)
	}

	var values map[string]interface{}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("parsing JSON values from %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("parsing YAML values from %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported values file format: %s (expected .json, .yaml, or .yml)", ext)
	}

	normalizeNumbers(values)

	if err := applySetOverrides(values, setOverrides); err != nil {
		return nil, fmt.Errorf("applying --set overrides: %w", err)
	}

	return values, nil
}

// normalizeNumbers converts float64 whole numbers (from JSON) and int (from YAML)
// to int64 for consistent downstream behavior.
func normalizeNumbers(m map[string]interface{}) {
	for k, v := range m {
		m[k] = normalizeValue(v)
	}
}

func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case int:
		return int64(val)
	case map[string]interface{}:
		normalizeNumbers(val)
		return val
	case []interface{}:
		for i, item := range val {
			val[i] = normalizeValue(item)
		}
		return val
	default:
		return val
	}
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
