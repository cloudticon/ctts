package engine

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloudticon/ctts/internal/packages"
	"github.com/evanw/esbuild/pkg/api"
)

type Transpiler struct {
	stdlib     fs.FS
	projectDir string
}

func NewTranspiler(stdlib fs.FS, projectDir string) *Transpiler {
	if projectDir != "" {
		if abs, err := filepath.Abs(projectDir); err == nil {
			projectDir = abs
		}
	}
	return &Transpiler{stdlib: stdlib, projectDir: projectDir}
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
		},
		Plugins: []api.Plugin{t.cttsResolverPlugin()},
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

func (t *Transpiler) BundleValues(entryPoint string) (string, error) {
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entryPoint},
		Bundle:      true,
		Format:      api.FormatIIFE,
		GlobalName:  "__values_export",
		Platform:    api.PlatformNeutral,
		Write:       false,
		Loader: map[string]api.Loader{
			".ts": api.LoaderTS,
		},
		Plugins: []api.Plugin{t.cttsResolverPlugin()},
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

func (t *Transpiler) cttsResolverPlugin() api.Plugin {
	return api.Plugin{
		Name: "ctts-resolver",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^ctts/`},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					p := strings.TrimPrefix(args.Path, "ctts/") + ".ts"
					return api.OnResolveResult{
						Path:      p,
						Namespace: "ctts",
					}, nil
				})

			build.OnResolve(api.OnResolveOptions{Filter: `^\.`, Namespace: "ctts"},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					resolved := path.Join(path.Dir(args.Importer), args.Path)
					if !strings.HasSuffix(resolved, ".ts") {
						resolved += ".ts"
					}
					return api.OnResolveResult{
						Path:      resolved,
						Namespace: "ctts",
					}, nil
				})

		build.OnResolve(api.OnResolveOptions{Filter: `^[a-zA-Z0-9]`},
			func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				if strings.HasPrefix(args.Path, "ctts/") {
					return api.OnResolveResult{}, nil
				}
				if !packages.IsGitPackage(args.Path) {
					return api.OnResolveResult{}, nil
				}
				pkgName, subPath := packages.SplitPackagePath(args.Path)
				pkgDir := filepath.Join(t.projectDir, ".ctts", "packages", pkgName)
				if subPath != "" {
					return api.OnResolveResult{
						Path: filepath.Join(pkgDir, subPath+".ts"),
					}, nil
				}
				return api.OnResolveResult{
					Path: filepath.Join(pkgDir, "index.ts"),
				}, nil
			})

		build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "ctts"},
			func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				fsPath := path.Join("stdlib", args.Path)
				data, err := fs.ReadFile(t.stdlib, fsPath)
				if err != nil {
					return api.OnLoadResult{}, fmt.Errorf("stdlib file not found: %s", fsPath)
				}
				contents := string(data)
				return api.OnLoadResult{
					Contents:   &contents,
					Loader:     api.LoaderTS,
					ResolveDir: path.Dir(args.Path),
				}, nil
			})
	},
}
}
