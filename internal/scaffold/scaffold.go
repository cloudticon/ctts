package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

const ctTsTemplate = `import { deployment } from "github.com/cloudticon/k8s/apps/v1";
import { service } from "github.com/cloudticon/k8s/core/v1";

const app = deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});

service({
  name: "web-app-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 8080 }],
});
`

const valuesTsTemplate = `export default {
  image: "nginx:1.25",
  replicas: 3,
};
`

const tsconfigTemplate = `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ES2020",
    "moduleResolution": "node",
    "strict": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": {
      "ctts/*": [".ctts/types/*"]
    }
  },
  "include": ["*.ts", ".ctts/types/**/*.ts", ".ctts/types/**/*.d.ts"]
}
`

const gitignoreTemplate = `.ctts/packages/
`

// Init creates the project folder structure with starter files, then delegates
// to Sync to copy stdlib types and generate values.d.ts.
func Init(dir string) error {
	return InitWith(dir, realPackageSyncer{})
}

// InitWith is like Init but accepts a custom PackageSyncer, allowing tests to
// skip real git operations.
func InitWith(dir string, pkgSyncer PackageSyncer) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating project directory %s: %w", dir, err)
	}

	starterFiles := []struct {
		path    string
		content string
	}{
		{filepath.Join(dir, "ct.ts"), ctTsTemplate},
		{filepath.Join(dir, "values.ts"), valuesTsTemplate},
		{filepath.Join(dir, "tsconfig.json"), tsconfigTemplate},
		{filepath.Join(dir, ".gitignore"), gitignoreTemplate},
	}
	for _, f := range starterFiles {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", f.path, err)
		}
	}

	return SyncWith(dir, pkgSyncer)
}
