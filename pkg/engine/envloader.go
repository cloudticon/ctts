package engine

import (
	"bufio"
	"os"
	"strings"
)

// LoadEnvFile parses a .env file into a map of key-value pairs.
// Supports KEY=VALUE, KEY="VALUE", KEY='VALUE' formats.
// Lines starting with # and empty lines are skipped.
func LoadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = unquoteEnvValue(value)
		env[key] = value
	}

	return env, scanner.Err()
}

func unquoteEnvValue(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// MergeEnvWithSystem merges file env vars on top of system env.
// File env vars take precedence over system env.
func MergeEnvWithSystem(fileEnv map[string]string) map[string]string {
	result := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		result[k] = v
	}
	for k, v := range fileEnv {
		result[k] = v
	}
	return result
}
