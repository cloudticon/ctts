package scaffold

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/cloudticon/ctts/internal/packages"
)

// PackageSyncer abstracts git-based package installation so that callers
// (and tests) can substitute a no-op or mock implementation.
type PackageSyncer interface {
	SyncPackages(dir string) error
}

type realPackageSyncer struct{}

func (realPackageSyncer) SyncPackages(dir string) error {
	return packages.SyncPackages(dir)
}

type nopSyncer struct{}

func (nopSyncer) SyncPackages(string) error { return nil }

// NopPackageSyncer returns a PackageSyncer that does nothing.
func NopPackageSyncer() PackageSyncer { return nopSyncer{} }

// Sync regenerates stdlib types and values.d.ts for an existing ct project.
// It expects dir to contain ct.ts and a .ctts/ directory structure.
func Sync(dir string) error {
	return SyncWith(dir, realPackageSyncer{})
}

// SyncWith is like Sync but accepts a custom PackageSyncer.
func SyncWith(dir string, pkgSyncer PackageSyncer) error {
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

	if err := pkgSyncer.SyncPackages(dir); err != nil {
		return fmt.Errorf("syncing packages: %w", err)
	}

	valuesPath := filepath.Join(dir, "values.ts")
	valuesDtsPath := filepath.Join(typesDir, "values.d.ts")
	if err := GenerateValuesDts(valuesPath, valuesDtsPath); err != nil {
		return fmt.Errorf("generating values.d.ts: %w", err)
	}

	if err := generateTsconfig(dir); err != nil {
		return fmt.Errorf("generating tsconfig.json: %w", err)
	}

	return nil
}

func generateTsconfig(dir string) error {
	lockPath := filepath.Join(dir, "ct.lock")
	lf, err := packages.ReadLock(lockPath)
	if err != nil {
		return err
	}

	paths := map[string][]string{
		"ctts/*": {".ctts/types/*"},
	}
	include := []string{
		"*.ts",
		".ctts/types/**/*.ts",
		".ctts/types/**/*.d.ts",
	}

	pkgNames := sortedPackageNames(lf.Packages)
	if len(pkgNames) > 0 {
		include = append(include, ".ctts/packages/**/*.ts")
	}
	for _, name := range pkgNames {
		paths[name] = []string{".ctts/packages/" + name + "/index.ts"}
		paths[name+"/*"] = []string{".ctts/packages/" + name + "/*"}
	}

	tsconfig := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"target":           "ES2020",
			"module":           "ES2020",
			"moduleResolution": "node",
			"strict":           true,
			"noEmit":           true,
			"baseUrl":          ".",
			"paths":            paths,
		},
		"include": include,
	}

	data, err := json.MarshalIndent(tsconfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling tsconfig: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "tsconfig.json"), data, 0o644)
}

func sortedPackageNames[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
