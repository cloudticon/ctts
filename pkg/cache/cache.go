package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type PackageRef struct {
	Host    string
	Owner   string
	Repo    string
	Version string
}

var packageURLRegex = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/@]+)(?:@(.+))?$`)

func ParsePackageURL(rawURL string) (*PackageRef, error) {
	m := packageURLRegex.FindStringSubmatch(rawURL)
	if m == nil {
		return nil, fmt.Errorf("invalid package URL: %s (expected https://host/owner/repo[@version])", rawURL)
	}
	return &PackageRef{Host: m[1], Owner: m[2], Repo: m[3], Version: m[4]}, nil
}

func (r *PackageRef) CacheKey() string {
	version := r.Version
	if version == "" {
		version = "_default"
	}
	return filepath.Join(r.Host, r.Owner, r.Repo+"@"+version)
}

func (r *PackageRef) GitURL() string {
	return fmt.Sprintf("https://%s/%s/%s.git", r.Host, r.Owner, r.Repo)
}

func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".ct", "cache"), nil
}

// Resolve returns the local directory for a cached package.
// If the package is not cached, it is downloaded first.
func Resolve(rawURL string) (string, error) {
	ref, err := ParsePackageURL(rawURL)
	if err != nil {
		return "", err
	}

	cacheBase, err := CacheDir()
	if err != nil {
		return "", err
	}

	pkgDir := filepath.Join(cacheBase, ref.CacheKey())

	if dirHasFiles(pkgDir) {
		return pkgDir, nil
	}

	if err := download(ref, pkgDir); err != nil {
		return "", fmt.Errorf("downloading %s: %w", rawURL, err)
	}

	return pkgDir, nil
}

func download(ref *PackageRef, destDir string) error {
	tmpDir, err := os.MkdirTemp("", "ct-download-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gitURL := ref.GitURL()

	args := []string{"clone", "--depth", "1"}
	if ref.Version != "" {
		args = append(args, "--branch", ref.Version)
	}
	args = append(args, gitURL, tmpDir)

	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s: %s: %w", gitURL, strings.TrimSpace(string(out)), err)
	}

	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	return copyDir(tmpDir, destDir)
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		if strings.HasPrefix(rel, ".git") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		return os.WriteFile(target, data, 0o644)
	})
}
