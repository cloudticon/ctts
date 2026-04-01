package packages

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Import struct {
	Path string
}

var importRegex = regexp.MustCompile(`(?s)(?:import|export)\s.*?from\s*["']([^"']+)["']`)

func ParseImports(filePath string) ([]Import, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}
	return ParseImportsFromSource(string(data)), nil
}

func ParseImportsFromSource(source string) []Import {
	matches := importRegex.FindAllStringSubmatch(source, -1)
	imports := make([]Import, 0, len(matches))
	for _, m := range matches {
		imports = append(imports, Import{Path: m[1]})
	}
	return imports
}

func IsGitPackage(importPath string) bool {
	if strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "ctts/") {
		return false
	}
	parts := strings.SplitN(importPath, "/", 2)
	if len(parts) < 2 {
		return false
	}
	return strings.Contains(parts[0], ".")
}

func SplitPackagePath(importPath string) (pkgName, subPath string) {
	parts := strings.Split(importPath, "/")
	n := packageSegmentCount(parts[0])
	if len(parts) <= n {
		return importPath, ""
	}
	return strings.Join(parts[:n], "/"), strings.Join(parts[n:], "/")
}

func PackageToGitURL(pkgName string) string {
	return "https://" + pkgName + ".git"
}

func packageSegmentCount(domain string) int {
	wellKnown := [...]string{"github.com", "gitlab.com", "bitbucket.org"}
	for _, h := range wellKnown {
		if domain == h {
			return 3
		}
	}
	return 2
}
