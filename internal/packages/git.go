package packages

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitClient interface {
	Clone(url, ref, destDir string) (sha string, err error)
	FetchSHA(url, ref string) (sha string, err error)
}

type gitCLI struct{}

func NewGitClient() GitClient {
	return gitCLI{}
}

func (gitCLI) Clone(url, ref, destDir string) (string, error) {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, destDir)

	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone %s: %s: %w", url, strings.TrimSpace(string(out)), err)
	}

	sha, err := revParseHEAD(destDir)
	if err != nil {
		return "", fmt.Errorf("resolving HEAD after clone: %w", err)
	}
	return sha, nil
}

func (gitCLI) FetchSHA(url, ref string) (string, error) {
	lsRef := ref
	if lsRef == "" {
		lsRef = "HEAD"
	}

	out, err := exec.Command("git", "ls-remote", url, lsRef).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git ls-remote %s %s: %s: %w", url, lsRef, strings.TrimSpace(string(out)), err)
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return "", fmt.Errorf("ref %q not found in %s", ref, url)
	}
	fields := strings.Fields(strings.SplitN(lines, "\n", 2)[0])
	if len(fields) == 0 {
		return "", fmt.Errorf("unexpected ls-remote output for %s", url)
	}
	return fields[0], nil
}

func revParseHEAD(repoDir string) (string, error) {
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD in %s: %s: %w", repoDir, strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
