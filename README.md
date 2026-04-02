# ct — Kubernetes manifests from code

`ct` generates Kubernetes YAML/JSON manifests from `.ct` definitions. Write real code (loops, conditionals, cross-references) instead of templating languages.

## Features

- **TypeScript syntax** — full language power: variables, loops, conditionals, type safety
- **Registration model** — each helper call (`deployment()`, `service()`, …) registers a resource and returns it for cross-references
- **URL imports** — packages resolved directly from URLs, no local config needed
- **Values** — `values.json` or `values.yaml` with `--set` overrides, like Helm
- **Multi-env** — separate values files per environment (ArgoCD-style)
- **Low-level API** — `resource()` / `resourceClusterScope()` for CRDs and any K8s object
- **High-level helpers** — `deployment`, `service`, `configMap`, `secret`, `ingress`, `statefulSet`, `daemonSet`, `namespace`
- **Zero dependencies for users** — single Go binary, no Node.js required
- **Zero config** — no `tsconfig.json`, no generated directories, just your code and values

## Install

One-line install (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/cloudticon/ctts/master/install.sh | sudo sh
```

Via `go install`:

```bash
go install github.com/cloudticon/ctts/cmd/ct@latest
```

Or build from source:

```bash
git clone https://github.com/cloudticon/ctts.git
cd ctts
go build -o ct ./cmd/ct
```

## Quick start

```bash
# Initialize a project in the current directory
ct init

# Edit main.ct and values.json, then render:
ct template . --namespace production

# JSON output
ct template . --namespace production --output json

# Override values
ct template . --namespace production --set replicas=5

# Explicit values file (useful for multi-env)
ct template . --namespace staging --values values-staging.json
```

## Project structure

After `ct init`, you get:

```
myproject/
  main.ct         # manifest definitions (TypeScript syntax)
  values.json     # configurable values
```

That's it — no generated directories, no config files.

**Multi-env project (ArgoCD):**

```
myproject/
  main.ct              # code
  values.json          # default values
  values-prod.json     # production overrides
  values-staging.yaml  # staging (YAML also supported)
```

## Example: main.ct

```typescript
import { deployment } from "https://github.com/cloudticon/k8s@master";
import { service } from "https://github.com/cloudticon/k8s@master";

const app = deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});

// Cross-reference — use app.metadata.name, zero typos
service({
  name: "web-app-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 8080 }],
});
```

## Example: values.json

```json
{
  "image": "nginx:1.25",
  "replicas": 3
}
```

Values can also be YAML:

```yaml
# values.yaml
image: nginx:1.25
replicas: 3
domain: app.example.com
```

## Output

```bash
$ ct template . --namespace production
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
    labels:
        app: web-app
    name: web-app
    namespace: production
spec:
    replicas: 3
    selector:
        matchLabels:
            app: web-app
    template:
        metadata:
            labels:
                app: web-app
        spec:
            containers:
                - image: nginx:1.25
                  name: web-app
                  ports:
                    - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
    name: web-app-svc
    namespace: production
spec:
    ports:
        - port: 80
          targetPort: 8080
    selector:
        app: web-app
```

## Conditional resources

```typescript
import { deployment } from "https://github.com/cloudticon/k8s@master";
import { ingress } from "https://github.com/cloudticon/k8s@master";

deployment({ name: "api", image: Values.image });

if (Values.enableIngress) {
  ingress({
    name: "api-ingress",
    host: Values.domain,
    serviceName: "api-svc",
  });
}
```

## Loops

```typescript
import { deployment } from "https://github.com/cloudticon/k8s@master";

for (const worker of Values.workers) {
  deployment({
    name: `worker-${worker.name}`,
    image: worker.image,
    replicas: worker.replicas,
  });
}
```

## Low-level API (CRDs)

```typescript
import { resource } from "https://github.com/cloudticon/k8s@master";

resource({
  apiVersion: "redis.redis.opstreelabs.in/v1beta2",
  kind: "Redis",
  metadata: { name: "my-redis" },
  spec: {
    kubernetesConfig: { image: "redis:7.2" },
    redisExporter: { enabled: true },
  },
});
```

## Cluster-scoped resources

```typescript
import { resourceClusterScope, namespace } from "https://github.com/cloudticon/k8s@master";

namespace({ name: "production" });

resourceClusterScope({
  apiVersion: "kyverno.io/v1",
  kind: "ClusterPolicy",
  metadata: { name: "require-labels" },
  spec: { validationFailureAction: "Enforce" },
});
```

## Available helpers

| Function | Description |
|----------|-------------|
| `deployment` | Deployment (apps/v1) |
| `statefulSet` | StatefulSet (apps/v1) |
| `daemonSet` | DaemonSet (apps/v1) |
| `service` | Service (core/v1) |
| `configMap` | ConfigMap (core/v1) |
| `secret` | Secret (core/v1) |
| `namespace` | Namespace (core/v1) |
| `ingress` | Ingress (networking/v1) |
| `resource` | Any namespaced K8s resource (CRDs) |
| `resourceClusterScope` | Any cluster-scoped K8s resource |

All helpers are imported from `https://github.com/cloudticon/k8s@<version>`.

## URL imports

Packages are referenced directly via URL in import statements:

```typescript
import { deployment } from "https://github.com/cloudticon/k8s@master";
```

URL format: `https://github.com/{owner}/{repo}@{version}`

- `{version}` is a git tag or branch name
- Packages are downloaded on first use and cached in `~/.ct/cache/`
- Subsequent runs work offline from cache

## CLI reference

```
ct init [flags]
  -d, --dir string   project directory (default ".")

ct template <dir> [flags]
  -n, --namespace string   default namespace for resources
  -f, --values string      path to values file (JSON or YAML, overrides auto-detect)
  -o, --output string      output format: yaml or json (default "yaml")
      --set stringArray    override values (e.g. --set replicas=5)
```

`ct template` auto-detects values files in order: `values.json`, `values.yaml`, `values.yml`. Use `--values` to override.

## How it works

1. **esbuild** bundles `main.ct` + URL-imported packages into a single IIFE JS file
2. URL imports are resolved from `~/.ct/cache/` (downloaded on first use via `git clone`)
3. **Goja** (pure Go JS engine) executes the bundle — each helper call pushes a resource to a global registry
4. `Values` object is injected as a global from the loaded JSON/YAML values file
5. **Post-processing** applies default namespace (from `--namespace`), skips cluster-scoped resources, removes nil fields
6. **Serializer** outputs YAML or JSON

No Node.js runtime needed — everything runs inside the Go binary.

## Development

```bash
# Run all tests
go test ./...

# Build
go build -o ct ./cmd/ct
```

## License

Apache 2.0
