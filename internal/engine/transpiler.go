package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudticon/ctts/internal/cache"
	"github.com/cloudticon/ctts/internal/packages"
	"github.com/evanw/esbuild/pkg/api"
)

type Transpiler struct {
	projectDir string
}

func NewTranspiler(projectDir string) *Transpiler {
	if projectDir != "" {
		if abs, err := filepath.Abs(projectDir); err == nil {
			projectDir = abs
		}
	}
	return &Transpiler{projectDir: projectDir}
}

func (t *Transpiler) Bundle(entryPoint string) (string, error) {
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entryPoint},
		Bundle:      true,
		Format:      api.FormatIIFE,
		Platform:    api.PlatformNeutral,
		Write:       false,
		Loader: map[string]api.Loader{
			".ts": api.LoaderTS,
			".ct": api.LoaderTS,
		},
		Plugins: []api.Plugin{t.urlResolverPlugin()},
	})

	if len(result.Errors) > 0 {
		msgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			msgs[i] = e.Text
		}
		return "", fmt.Errorf("esbuild errors: %s", strings.Join(msgs, "; "))
	}

	if len(result.OutputFiles) == 0 {
		return "", fmt.Errorf("esbuild produced no output")
	}

	return string(result.OutputFiles[0].Contents), nil
}

func (t *Transpiler) urlResolverPlugin() api.Plugin {
	return api.Plugin{
		Name: "url-resolver",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^https://`},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					pkgDir, err := cache.Resolve(args.Path)
					if err != nil {
						return api.OnResolveResult{}, err
					}
					return api.OnResolveResult{
						Path: resolveFilePath(pkgDir, ""),
					}, nil
				})

			build.OnResolve(api.OnResolveOptions{Filter: `^[a-zA-Z0-9]`},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					if !packages.IsGitPackage(args.Path) {
						return api.OnResolveResult{}, nil
					}

					pkgWithVersion, subPath := packages.SplitPackagePath(args.Path)
					pkg, version := packages.SplitPackageVersion(pkgWithVersion)

					url := "https://" + pkg
					if version != "" {
						url += "@" + version
					}

					pkgDir, err := cache.Resolve(url)
					if err != nil {
						return api.OnResolveResult{}, err
					}

					return api.OnResolveResult{
						Path: resolveFilePath(pkgDir, subPath),
					}, nil
				})
		},
	}
}

var entryExtensions = [...]string{".ts", ".ct"}

func resolveFilePath(pkgDir, subPath string) string {
	if subPath == "" {
		return resolveIndex(pkgDir)
	}
	for _, ext := range entryExtensions {
		f := filepath.Join(pkgDir, subPath+ext)
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	return resolveIndex(filepath.Join(pkgDir, subPath))
}

func resolveIndex(dir string) string {
	for _, ext := range entryExtensions {
		f := filepath.Join(dir, "index"+ext)
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	return filepath.Join(dir, "index.ts")
}
