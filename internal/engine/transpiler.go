package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudticon/ctts/internal/cache"
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
						Path: filepath.Join(pkgDir, "index.ts"),
					}, nil
				})
		},
	}
}
