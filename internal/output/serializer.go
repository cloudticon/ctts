package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Resource = map[string]interface{}

func Serialize(resources []Resource, format string) (string, error) {
	cleaned := make([]Resource, len(resources))
	for i, r := range resources {
		cleaned[i] = cleanNilFields(r).(Resource)
	}

	switch format {
	case "yaml", "":
		return serializeYAML(cleaned)
	case "json":
		return serializeJSON(cleaned)
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

func serializeYAML(resources []Resource) (string, error) {
	var docs []string
	for _, r := range resources {
		data, err := yaml.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("yaml marshal error: %w", err)
		}
		docs = append(docs, strings.TrimRight(string(data), "\n"))
	}
	return strings.Join(docs, "\n---\n") + "\n", nil
}

func serializeJSON(resources []Resource) (string, error) {
	data, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal error: %w", err)
	}
	return string(data) + "\n", nil
}

func cleanNilFields(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{}, len(val))
		for k, v := range val {
			cv := cleanNilFields(v)
			if cv != nil {
				cleaned[k] = cv
			}
		}
		if len(cleaned) == 0 {
			return nil
		}
		return cleaned
	case []interface{}:
		cleaned := make([]interface{}, 0, len(val))
		for _, item := range val {
			cv := cleanNilFields(item)
			if cv != nil {
				cleaned = append(cleaned, cv)
			}
		}
		if len(cleaned) == 0 {
			return nil
		}
		return cleaned
	default:
		return v
	}
}
