package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cloudticon/ctts/internal/k8s"
)

// Sync regenerates stdlib types and values.d.ts for an existing ct project.
// It expects dir to contain ct.ts and a .ctts/ directory structure.
func Sync(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "ct.ts")); os.IsNotExist(err) {
		return fmt.Errorf("ct.ts not found in %s — is this a ct project?", dir)
	}

	typesDir := filepath.Join(dir, ".ctts", "types")
	for _, sub := range []string{
		filepath.Join(typesDir, "k8s", "apps"),
		filepath.Join(typesDir, "k8s", "core"),
		filepath.Join(typesDir, "k8s", "networking"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", sub, err)
		}
	}

	if err := copyStdlibTypes(dir); err != nil {
		return fmt.Errorf("copying stdlib types: %w", err)
	}

	valuesPath := filepath.Join(dir, "values.ts")
	valuesDtsPath := filepath.Join(typesDir, "values.d.ts")
	if err := GenerateValuesDts(valuesPath, valuesDtsPath); err != nil {
		return fmt.Errorf("generating values.d.ts: %w", err)
	}

	return nil
}

func copyStdlibTypes(dir string) error {
	typesDir := filepath.Join(dir, ".ctts", "types")
	return fs.WalkDir(k8s.Stdlib, "stdlib", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel("stdlib", path)
		targetPath := filepath.Join(typesDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, readErr := fs.ReadFile(k8s.Stdlib, path)
		if readErr != nil {
			return fmt.Errorf("reading embedded %s: %w", path, readErr)
		}
		return os.WriteFile(targetPath, content, 0o644)
	})
}
