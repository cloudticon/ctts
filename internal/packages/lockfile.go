package packages

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type LockEntry struct {
	Ref string `yaml:"ref"`
	SHA string `yaml:"sha"`
}

type LockFile struct {
	Version  int                  `yaml:"version"`
	Packages map[string]LockEntry `yaml:"packages,omitempty"`
}

func NewLockFile() *LockFile {
	return &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}
}

func ReadLock(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewLockFile(), nil
		}
		return nil, fmt.Errorf("reading lock file %s: %w", path, err)
	}

	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lock file %s: %w", path, err)
	}
	if lf.Packages == nil {
		lf.Packages = make(map[string]LockEntry)
	}
	return &lf, nil
}

func WriteLock(path string, lf *LockFile) error {
	ordered := &orderedLockFile{
		Version:  lf.Version,
		Packages: sortedEntries(lf.Packages),
	}
	data, err := yaml.Marshal(ordered)
	if err != nil {
		return fmt.Errorf("marshalling lock file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing lock file %s: %w", path, err)
	}
	return nil
}

type orderedLockFile struct {
	Version  int                          `yaml:"version"`
	Packages yaml.Node                    `yaml:"packages,omitempty"`
}

func sortedEntries(m map[string]LockEntry) yaml.Node {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	node := yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		e := m[k]
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "ref"},
					{Kind: yaml.ScalarNode, Value: e.Ref},
					{Kind: yaml.ScalarNode, Value: "sha"},
					{Kind: yaml.ScalarNode, Value: e.SHA},
				},
			},
		)
	}
	return node
}
