package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

const mainCtTemplate = `import { deployment } from "https://github.com/cloudticon/k8s@master";
import { service } from "https://github.com/cloudticon/k8s@master";

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

const valuesJsonTemplate = `{
  "image": "nginx:1.25",
  "replicas": 3
}
`

func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating project directory %s: %w", dir, err)
	}

	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(dir, "main.ct"), mainCtTemplate},
		{filepath.Join(dir, "values.json"), valuesJsonTemplate},
	}
	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", f.path, err)
		}
	}

	return nil
}
