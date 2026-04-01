# ct — Kubernetes manifests from TypeScript

`ct` generates Kubernetes YAML/JSON manifests from TypeScript definitions. Write real code (loops, conditionals, cross-references) instead of templating languages.

## Features

- **TypeScript** — full language power: variables, loops, conditionals, type safety
- **Registration model** — each helper call (`deployment()`, `service()`, …) registers a resource and returns it for cross-references
- **IDE autocomplete** — generated `tsconfig.json` + `.d.ts` types work in any IDE with TypeScript support
- **Values** — typed `values.ts` with `--set` overrides, like Helm
- **Low-level API** — `resource()` / `resourceClusterScope()` for CRDs and any K8s object
- **High-level helpers** — `deployment`, `service`, `configMap`, `secret`, `ingress`, `statefulSet`, `daemonSet`, `namespace`
- **Zero dependencies for users** — single Go binary, no Node.js required

## Install

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
# Initialize a project
ct init

# Edit ct/ct.ts and ct/values.ts, then render:
ct template ct/ --namespace production

# JSON output
ct template ct/ --namespace production --output json

# Override values
ct template ct/ --namespace production --set replicas=5

# Custom values file
ct template ct/ --namespace staging --values other-values.ts

# After editing values.ts, regenerate types for IDE autocomplete:
ct sync ct/
```

## Project structure

After `ct init`, you get:

```
ct/
  ct.ts              # your manifest definitions
  values.ts          # configurable values
  tsconfig.json      # IDE paths mapping (generated)
  .ctts/
    types/
      k8s/           # stdlib type definitions
      values.d.ts    # typed Values (generated from values.ts)
```

## Example: ct.ts

```typescript
import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";

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

## Example: values.ts

```typescript
export default {
  image: "nginx:1.25",
  replicas: 3,
};
```

## Output

```bash
$ ct template ct/ --namespace production
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
import { deployment } from "ctts/k8s/apps/v1";
import { ingress } from "ctts/k8s/networking/v1";

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
import { deployment } from "ctts/k8s/apps/v1";

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
import { resource } from "ctts/k8s/resource";

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
import { resourceClusterScope } from "ctts/k8s/resource";
import { namespace } from "ctts/k8s/core/v1";

namespace({ name: "production" });

resourceClusterScope({
  apiVersion: "kyverno.io/v1",
  kind: "ClusterPolicy",
  metadata: { name: "require-labels" },
  spec: { validationFailureAction: "Enforce" },
});
```

## Available helpers

| Import | Functions |
|--------|-----------|
| `ctts/k8s/apps/v1` | `deployment`, `statefulSet`, `daemonSet` |
| `ctts/k8s/core/v1` | `service`, `configMap`, `secret`, `namespace` |
| `ctts/k8s/networking/v1` | `ingress` |
| `ctts/k8s/resource` | `resource`, `resourceClusterScope` |

## CLI reference

```
ct init [flags]
  -d, --dir string   project directory name (default "ct")

ct sync [dir]
  Regenerate stdlib types and values.d.ts (default dir: "ct")

ct template <dir> [flags]
  -n, --namespace string   default namespace for resources
  -f, --values string      path to values.ts (overrides auto-detect)
  -o, --output string      output format: yaml or json (default "yaml")
      --set stringArray    override values (e.g. --set replicas=5)
```

## How it works

1. **esbuild** bundles `ct.ts` + stdlib into a single IIFE JS file
2. **Goja** (pure Go JS engine) executes the bundle — each helper call pushes a resource to a global registry
3. **Post-processing** applies default namespace (from `--namespace`), skips cluster-scoped resources, removes nil fields
4. **Serializer** outputs YAML or JSON

No Node.js runtime needed — everything runs inside the Go binary.

## Development

```bash
# Run all tests
go test ./...

# Update golden files
go test -run TestGolden -update

# Build
go build -o ct ./cmd/ct
```

## License

Apache 2.0
