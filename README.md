# ct — Kubernetes manifests from code

[![Build](https://github.com/cloudticon/ctts/actions/workflows/build.yml/badge.svg)](https://github.com/cloudticon/ctts/actions/workflows/build.yml)
[![Release](https://github.com/cloudticon/ctts/actions/workflows/release.yml/badge.svg)](https://github.com/cloudticon/ctts/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/cloudticon/ctts)](https://github.com/cloudticon/ctts)
[![License](https://img.shields.io/github/license/cloudticon/ctts)](https://github.com/cloudticon/ctts/blob/master/LICENSE)

> **⚠️ Beta** — `ct` is under active development. APIs, CLI flags, and file formats may change between releases. Feedback and bug reports are welcome!

`ct` generates Kubernetes YAML/JSON manifests from `.ct` definitions. Write real code (loops, conditionals, cross-references) instead of templating languages.

`ct dev` mode is inspired by [DevSpace](https://devspace.sh): fast inner-loop development directly against Kubernetes workloads.

**Documentation:** [cloudticon.com](https://cloudticon.com/)

## Features

- **TypeScript syntax** — full language power: variables, loops, conditionals, type safety
- **Registration model** — each helper call (`deployment()`, `service()`, …) registers a resource and returns it for cross-references
- **URL imports** — packages resolved directly from URLs, no local config needed
- **Values** — `values.json` or `values.yaml` with `--set` overrides, like Helm
- **Multi-env** — separate values files per environment (ArgoCD-style)
- **Custom resources** — `resource()` factory for CRDs and any K8s object, with optional schema
- **High-level helpers** — `deployment`, `service`, `configMap`, `secret`, `ingress`, `statefulSet`, `daemonSet`, `namespace`
- **IDE support** — `ct types` generates `.d.ts` files for autocomplete and type checking
- **Embeddable engine** — public Go API in `pkg/` for building tools on top of ct
- **Zero dependencies for users** — single Go binary, no Node.js required
- **Zero config** — no `tsconfig.json`, no generated directories, just your code and values

## Install

One-line install (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/cloudticon/ct/master/install.sh | sudo sh
```

Via `go install`:

```bash
go install github.com/cloudticon/ct/cmd/ct@latest
```

Or build from source:

```bash
git clone https://github.com/cloudticon/ct.git
cd ct
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

## User project layout

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
import { deployment } from "github.com/cloudticon/k8s@master";
import { service } from "github.com/cloudticon/k8s@master";

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
import { deployment } from "github.com/cloudticon/k8s@master";
import { ingress } from "github.com/cloudticon/k8s@master";

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
import { deployment } from "github.com/cloudticon/k8s@master";

for (const worker of Values.workers) {
  deployment({
    name: `worker-${worker.name}`,
    image: worker.image,
    replicas: worker.replicas,
  });
}
```

## Custom resources (CRDs)

Use `resource()` to define your own typed resource factory:

```typescript
import { resource, z } from "github.com/cloudticon/k8s@master";

const redis = resource("redis.redis.opstreelabs.in/v1beta2", "Redis", {
  spec: {
    image: z.string(),
    exporter: z.boolean().optional(),
  },
});

redis({
  name: "my-redis",
  image: "redis:7.2",
  exporter: true,
});
```

For cluster-scoped resources, set `scope`:

```typescript
import { resource, z } from "github.com/cloudticon/k8s@master";

const clusterPolicy = resource("kyverno.io/v1", "ClusterPolicy", {
  scope: "Cluster",
  spec: {
    validationFailureAction: z.enum(["Enforce", "Audit"]),
  },
});

clusterPolicy({
  name: "require-labels",
  validationFailureAction: "Enforce",
});
```

## Available helpers

| Function     | Description                                                   |
| ------------ | ------------------------------------------------------------- |
| `deployment` | Deployment (apps/v1)                                          |
| `statefulSet`| StatefulSet (apps/v1)                                         |
| `daemonSet`  | DaemonSet (apps/v1)                                           |
| `service`    | Service (core/v1)                                             |
| `configMap`  | ConfigMap (core/v1)                                           |
| `secret`     | Secret (core/v1)                                              |
| `namespace`  | Namespace (core/v1)                                           |
| `ingress`    | Ingress (networking/v1)                                       |
| `resource`   | Factory for custom/CRD resources (`scope: "Cluster"` option)  |
| `z`          | Schema builder for `resource()` spec definitions              |

All helpers are imported from `github.com/cloudticon/k8s@<version>`.

## URL imports

Packages are referenced directly via URL in import statements:

```typescript
import { deployment } from "github.com/cloudticon/k8s@master";
```

URL format: `github.com/{owner}/{repo}@{version}`

- `{version}` is a git tag or branch name
- Packages are downloaded on first use and cached in `~/.ct/cache/`
- Subsequent runs work offline from cache

## IDE support — `ct types`

Generate TypeScript type definitions for IDE autocomplete and type checking:

```bash
# Generate types for the current project
ct types .

# Custom output directory
ct types . --output ./types

# Include operator globals (getStatus, setStatus, fetch, log, Env)
ct types . --operator
```

Generated files:

| File           | Contents                                               |
| -------------- | ------------------------------------------------------ |
| `values.d.ts`  | `CtValues` interface inferred from `values.json`/YAML  |
| `globals.d.ts` | `declare const Values: CtValues` + operator globals    |

The command also resolves and caches URL imports so IDE resolution works offline.

Output directory defaults to `~/.ct/types/<project-hash>`. The path is printed to stdout so tools (e.g. VS Code extension) can consume it.

## Apply manifests to cluster — `ct apply`

Render and apply manifests in one step using Kubernetes server-side apply.

```bash
# Render + apply resources from main.ct
ct apply .

# Apply with explicit namespace and context
ct apply . --namespace development --context staging

# Override values while applying
ct apply . --values values-staging.yaml --set replicas=2

# Optionally print applied output
ct apply . --output yaml
```

`ct apply` is useful when you want one command for both generation and deployment without a separate `kubectl apply`.

## Development mode — `ct dev`

Run live development workflows directly on cluster workloads from `dev.ct` (DevSpace-inspired flow).

```bash
# Start dev mode from current directory
ct dev

# Use a custom env file
ct dev --env-file .env.dev

# Skip env file loading
ct dev --env-file ""

# Use a specific kubeconfig context
ct dev --context staging
```

`ct dev` executes `dev.ct`, applies rendered resources, then starts development features such as port forwarding, logs, and sync according to your dev targets.

## CLI reference

```
ct init [flags]
  -d, --dir string   project directory (default ".")

ct template <dir> [flags]
  -n, --namespace string   default namespace for resources
  -f, --values string      path to values file (JSON or YAML, overrides auto-detect)
  -o, --output string      output format: yaml or json (default "yaml")
      --set stringArray    override values (e.g. --set replicas=5)

ct apply <dir> [flags]
  -n, --namespace string   target namespace for resources
  -f, --values string      path to values file (JSON or YAML, overrides auto-detect)
  -o, --output string      output format: yaml or json (default: no output)
      --set stringArray    override values (e.g. --set replicas=5)
      --context string     kubeconfig context to use

ct dev [flags]
      --env-file string    path to .env file (empty to skip) (default ".env")
      --context string     kubeconfig context

ct types [dir] [flags]
      --output string      output directory (default ~/.ct/types/<project-hash>)
      --operator           include operator globals (getStatus, setStatus, fetch, log, Env)
      --dev                generate dev.d.ts for dev.ct IDE support
```

`ct template` auto-detects values files in order: `values.json`, `values.yaml`, `values.yml`. Use `--values` to override.

## How it works

1. **esbuild** bundles `main.ct` + URL-imported packages into a single IIFE JS file
2. URL imports are resolved from `~/.ct/cache/` (downloaded on first use via `git clone`)
3. An esbuild plugin rejects `async`/`await` at bundle time — the Goja runtime is synchronous
4. **Goja** (pure Go JS engine) executes the bundle — each helper call pushes a resource to a global registry
5. `Values` object is injected as a global from the loaded JSON/YAML values file
6. **Post-processing** applies default namespace (from `--namespace`), skips cluster-scoped resources, removes nil fields
7. **Serializer** outputs YAML or JSON

No Node.js runtime needed — everything runs inside the Go binary.

## Using ct as a Go library

The engine, cache, and package resolver are exported as public Go packages under `pkg/`:

```go
import (
    "github.com/cloudticon/ctts/pkg/engine"
    "github.com/cloudticon/ctts/pkg/cache"
    "github.com/cloudticon/ctts/pkg/packages"
)
```

### Engine — bundle and execute

```go
transpiler := engine.NewTranspiler("/path/to/project")
jsCode, err := transpiler.Bundle("main.ct")

values, err := engine.LoadValuesFile("values.json", []string{"replicas=5"})

resources, err := engine.Execute(engine.ExecuteOpts{
    JSCode:    jsCode,
    Values:    values,
    Namespace: "production",
})
```

### Cache — resolve URL imports to local paths

```go
localDir, err := cache.Resolve("https://github.com/cloudticon/k8s@master")
```

### Packages — parse imports and sync dependencies

```go
imports, err := packages.ParseImports("main.ct")
err = packages.SyncPackages("/path/to/project")
```

## Source layout

```
cmd/ct/              CLI entry point
internal/
  cli/               commands: init, template, types
  output/            YAML/JSON serializer
  scaffold/          ct init scaffolding
pkg/
  engine/            transpiler (esbuild) + runtime (Goja) + values loader
  cache/             URL import → local cache resolver
  packages/          git client, import parser, dependency sync
```

## Development

```bash
# Run all tests
go test ./...

# Build
go build -o ct ./cmd/ct
```

## License

Apache 2.0
