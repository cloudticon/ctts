package packages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Syncer struct {
	git GitClient
}

func NewSyncer() *Syncer {
	return &Syncer{git: NewGitClient()}
}

func NewSyncerWithGit(git GitClient) *Syncer {
	return &Syncer{git: git}
}

func SyncPackages(projectDir string) error {
	return NewSyncer().Sync(projectDir)
}

func UpdatePackages(projectDir string, pkgNames []string) error {
	return NewSyncer().Update(projectDir, pkgNames)
}

func (s *Syncer) Sync(projectDir string) error {
	lockPath := filepath.Join(projectDir, "ct.lock")
	packagesDir := filepath.Join(projectDir, ".ctts", "packages")

	lf, err := ReadLock(lockPath)
	if err != nil {
		return fmt.Errorf("reading lock file: %w", err)
	}

	installed, err := s.resolveAndInstall(projectDir, packagesDir, lf)
	if err != nil {
		return err
	}

	if !installed {
		return nil
	}

	if err := WriteLock(lockPath, lf); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

func (s *Syncer) Update(projectDir string, pkgNames []string) error {
	lockPath := filepath.Join(projectDir, "ct.lock")
	packagesDir := filepath.Join(projectDir, ".ctts", "packages")

	lf, err := ReadLock(lockPath)
	if err != nil {
		return fmt.Errorf("reading lock file: %w", err)
	}

	toUpdate := pkgNames
	if len(toUpdate) == 0 {
		toUpdate = make([]string, 0, len(lf.Packages))
		for name := range lf.Packages {
			toUpdate = append(toUpdate, name)
		}
	}

	for _, pkgName := range toUpdate {
		entry, exists := lf.Packages[pkgName]
		if !exists {
			return fmt.Errorf("package %s not found in ct.lock", pkgName)
		}

		url := PackageToGitURL(pkgName)
		latestSHA, err := s.git.FetchSHA(url, entry.Ref)
		if err != nil {
			return fmt.Errorf("fetching latest SHA for %s: %w", pkgName, err)
		}

		if latestSHA == entry.SHA {
			continue
		}

		pkgDir := filepath.Join(packagesDir, pkgName)
		sha, err := s.installOne(pkgName, entry.Ref, pkgDir)
		if err != nil {
			return fmt.Errorf("updating %s: %w", pkgName, err)
		}

		lf.Packages[pkgName] = LockEntry{Ref: entry.Ref, SHA: sha}
	}

	return WriteLock(lockPath, lf)
}

func (s *Syncer) resolveAndInstall(projectDir, packagesDir string, lf *LockFile) (bool, error) {
	entryPoint := filepath.Join(projectDir, "ct.ts")
	known := make(map[string]bool)
	visitedFiles := make(map[string]bool)
	fileQueue := []string{entryPoint}
	anyInstalled := false

	for len(fileQueue) > 0 {
		file := fileQueue[0]
		fileQueue = fileQueue[1:]

		if visitedFiles[file] {
			continue
		}
		visitedFiles[file] = true

		imports, err := ParseImports(file)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("parsing imports from %s: %w", file, err)
		}

		for _, imp := range imports {
			if !IsGitPackage(imp.Path) {
				continue
			}
			pkgName, _ := SplitPackagePath(imp.Path)
			if known[pkgName] {
				continue
			}
			known[pkgName] = true

			pkgDir := filepath.Join(packagesDir, pkgName)

			entry, inLock := lf.Packages[pkgName]
			if inLock && dirHasFiles(pkgDir) {
				tsFiles, _ := collectTSFiles(pkgDir)
				fileQueue = append(fileQueue, tsFiles...)
				continue
			}

			if err := os.MkdirAll(packagesDir, 0o755); err != nil {
				return false, fmt.Errorf("creating packages directory: %w", err)
			}

			sha, err := s.installOne(pkgName, entry.Ref, pkgDir)
			if err != nil {
				return false, err
			}

			ref := entry.Ref
			if ref == "" {
				ref = "main"
			}
			lf.Packages[pkgName] = LockEntry{Ref: ref, SHA: sha}
			anyInstalled = true

			tsFiles, _ := collectTSFiles(pkgDir)
			fileQueue = append(fileQueue, tsFiles...)
		}
	}

	return anyInstalled || len(known) > 0, nil
}

func (s *Syncer) installOne(pkgName, ref, pkgDir string) (string, error) {
	url := PackageToGitURL(pkgName)

	tmpDir, err := os.MkdirTemp("", "ctts-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir for %s: %w", pkgName, err)
	}
	defer os.RemoveAll(tmpDir)

	sha, err := s.git.Clone(url, ref, tmpDir)
	if err != nil {
		return "", fmt.Errorf("cloning %s: %w", pkgName, err)
	}

	if err := os.RemoveAll(pkgDir); err != nil {
		return "", fmt.Errorf("removing old package dir %s: %w", pkgDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(pkgDir), 0o755); err != nil {
		return "", fmt.Errorf("creating parent for %s: %w", pkgDir, err)
	}
	if err := copyPackageFiles(tmpDir, pkgDir); err != nil {
		return "", fmt.Errorf("copying package %s: %w", pkgName, err)
	}

	return sha, nil
}

func copyPackageFiles(src, dst string) error {
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

func collectTSFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".ts") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func detectDefaultBranch(repoDir string) string {
	headRef, err := os.ReadFile(filepath.Join(repoDir, ".git", "HEAD"))
	if err != nil {
		return "main"
	}
	s := strings.TrimSpace(string(headRef))
	if strings.HasPrefix(s, "ref: refs/heads/") {
		return strings.TrimPrefix(s, "ref: refs/heads/")
	}
	return "main"
}
