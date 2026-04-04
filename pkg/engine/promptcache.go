package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PromptCache stores prompt answers in a JSON file keyed by project directory hash.
type PromptCache struct {
	path string
	data map[string]string
}

// NewPromptCache creates a cache file at ~/.ct/prompt_cache/<project-hash>.json.
func NewPromptCache(projectDir string) (*PromptCache, error) {
	hash := sha256.Sum256([]byte(projectDir))
	hexHash := hex.EncodeToString(hash[:8])

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".ct", "prompt_cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	cachePath := filepath.Join(cacheDir, hexHash+".json")
	c := &PromptCache{
		path: cachePath,
		data: make(map[string]string),
	}

	if raw, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(raw, &c.data)
	}

	return c, nil
}

// NewPromptCacheFromPath creates a cache using an explicit file path (for testing).
func NewPromptCacheFromPath(cachePath string) *PromptCache {
	c := &PromptCache{
		path: cachePath,
		data: make(map[string]string),
	}
	if raw, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(raw, &c.data)
	}
	return c
}

func (c *PromptCache) Get(question string) (string, bool) {
	val, ok := c.data[question]
	return val, ok
}

// Set stores the answer and persists the cache to disk.
func (c *PromptCache) Set(question, answer string) error {
	c.data[question] = answer
	raw, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, raw, 0o644)
}

// MakePromptFn returns a prompt function that reads stdin with cache support.
func MakePromptFn(cache *PromptCache, reader io.Reader, writer io.Writer) func(string) (string, error) {
	return func(question string) (string, error) {
		if cached, ok := cache.Get(question); ok {
			fmt.Fprintf(writer, "%s [cached: %s]\n", question, cached)
			return cached, nil
		}
		fmt.Fprintf(writer, "%s: ", question)
		scanner := bufio.NewScanner(reader)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if err := cache.Set(question, answer); err != nil {
			return "", fmt.Errorf("saving prompt cache: %w", err)
		}
		return answer, nil
	}
}
