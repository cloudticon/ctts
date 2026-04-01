package ctts_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/engine"
	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/cloudticon/ctts/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGitPackage(t *testing.T, projectDir string) {
	t.Helper()
	pkgDir := filepath.Join(projectDir, ".ctts", "packages", "github.com", "cloudticon", "k8s")
	for _, sub := range []string{
		filepath.Join(pkgDir, "apps"),
		filepath.Join(pkgDir, "core"),
		filepath.Join(pkgDir, "networking"),
	} {
		require.NoError(t, os.MkdirAll(sub, 0o755))
	}
	srcFiles := map[string]string{
		"resource.ts":       filepath.Join(pkgDir, "resource.ts"),
		"index.ts":          filepath.Join(pkgDir, "index.ts"),
		"apps/v1.ts":        filepath.Join(pkgDir, "apps", "v1.ts"),
		"core/v1.ts":        filepath.Join(pkgDir, "core", "v1.ts"),
		"networking/v1.ts":  filepath.Join(pkgDir, "networking", "v1.ts"),
	}
	for rel, dst := range srcFiles {
		data, err := os.ReadFile(filepath.Join("packages", "k8s", rel))
		require.NoError(t, err, "reading package source %s", rel)
		require.NoError(t, os.WriteFile(dst, data, 0o644))
	}
}

func bundleAndSerialize(t *testing.T, tr *engine.Transpiler, entryPath, namespace string, values map[string]interface{}) string {
	t.Helper()
	js, err := tr.Bundle(entryPath)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Values:    values,
		Namespace: namespace,
	})
	require.NoError(t, err)

	yaml, err := output.Serialize(resources, "yaml")
	require.NoError(t, err)
	return yaml
}

func TestGitPackageK8s_MatchesEmbeddedStdlib(t *testing.T) {
	cases := []struct {
		name       string
		namespace  string
		values     map[string]interface{}
		embeddedTS string
		gitPkgTS   string
	}{
		{
			name:      "simple_deployment",
			namespace: "default",
			values:    map[string]interface{}{"image": "nginx:1.25", "replicas": 3},
			embeddedTS: `
import { deployment } from "ctts/k8s/apps/v1";
deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});`,
			gitPkgTS: `
import { deployment } from "github.com/cloudticon/k8s/apps/v1";
deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});`,
		},
		{
			name:      "multi_resource",
			namespace: "production",
			embeddedTS: `
import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
import { ingress } from "ctts/k8s/networking/v1";
const app = deployment({ name: "api", image: "api:latest", ports: [{ containerPort: 3000 }] });
service({ name: "api-svc", selector: { app: app.metadata.name }, ports: [{ port: 80, targetPort: 3000 }] });
ingress({ name: "api-ing", host: "api.example.com", serviceName: "api-svc" });`,
			gitPkgTS: `
import { deployment } from "github.com/cloudticon/k8s/apps/v1";
import { service } from "github.com/cloudticon/k8s/core/v1";
import { ingress } from "github.com/cloudticon/k8s/networking/v1";
const app = deployment({ name: "api", image: "api:latest", ports: [{ containerPort: 3000 }] });
service({ name: "api-svc", selector: { app: app.metadata.name }, ports: [{ port: 80, targetPort: 3000 }] });
ingress({ name: "api-ing", host: "api.example.com", serviceName: "api-svc" });`,
		},
		{
			name:      "low_level_resource",
			namespace: "redis-ns",
			embeddedTS: `
import { resource } from "ctts/k8s/resource";
resource({
  apiVersion: "redis.redis.opstreelabs.in/v1beta2",
  kind: "Redis",
  metadata: { name: "my-redis" },
  spec: { kubernetesConfig: { image: "redis:7.2" } },
});`,
			gitPkgTS: `
import { resource } from "github.com/cloudticon/k8s/resource";
resource({
  apiVersion: "redis.redis.opstreelabs.in/v1beta2",
  kind: "Redis",
  metadata: { name: "my-redis" },
  spec: { kubernetesConfig: { image: "redis:7.2" } },
});`,
		},
		{
			name:      "cluster_scoped_mixed",
			namespace: "production",
			embeddedTS: `
import { namespace, configMap } from "ctts/k8s/core/v1";
namespace({ name: "production" });
configMap({ name: "app-config", data: { env: "production" } });`,
			gitPkgTS: `
import { namespace, configMap } from "github.com/cloudticon/k8s/core/v1";
namespace({ name: "production" });
configMap({ name: "app-config", data: { env: "production" } });`,
		},
		{
			name:      "index_import",
			namespace: "default",
			embeddedTS: `
import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
deployment({ name: "app", image: "nginx" });
service({ name: "app-svc", selector: { app: "app" }, ports: [{ port: 80 }] });`,
			gitPkgTS: `
import { deployment, service } from "github.com/cloudticon/k8s";
deployment({ name: "app", image: "nginx" });
service({ name: "app-svc", selector: { app: "app" }, ports: [{ port: 80 }] });`,
		},
		{
			name:      "statefulset",
			namespace: "default",
			embeddedTS: `
import { statefulSet } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
service({ name: "db-headless", selector: { app: "db" }, ports: [{ port: 5432 }], clusterIP: "None" });
statefulSet({
  name: "db",
  image: "postgres:15",
  serviceName: "db-headless",
  replicas: 3,
  ports: [{ containerPort: 5432 }],
  volumeClaimTemplates: [{ name: "data", accessModes: ["ReadWriteOnce"], storage: "10Gi" }],
});`,
			gitPkgTS: `
import { statefulSet } from "github.com/cloudticon/k8s/apps/v1";
import { service } from "github.com/cloudticon/k8s/core/v1";
service({ name: "db-headless", selector: { app: "db" }, ports: [{ port: 5432 }], clusterIP: "None" });
statefulSet({
  name: "db",
  image: "postgres:15",
  serviceName: "db-headless",
  replicas: 3,
  ports: [{ containerPort: 5432 }],
  volumeClaimTemplates: [{ name: "data", accessModes: ["ReadWriteOnce"], storage: "10Gi" }],
});`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			embeddedDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(embeddedDir, "ct.ts"), []byte(tc.embeddedTS), 0o644))
			embeddedTr := engine.NewTranspiler(k8s.Stdlib, "")
			embeddedYAML := bundleAndSerialize(t, embeddedTr, filepath.Join(embeddedDir, "ct.ts"), tc.namespace, tc.values)

			gitDir := t.TempDir()
			setupGitPackage(t, gitDir)
			require.NoError(t, os.WriteFile(filepath.Join(gitDir, "ct.ts"), []byte(tc.gitPkgTS), 0o644))
			gitTr := engine.NewTranspiler(k8s.Stdlib, gitDir)
			gitYAML := bundleAndSerialize(t, gitTr, filepath.Join(gitDir, "ct.ts"), tc.namespace, tc.values)

			assert.Equal(t, embeddedYAML, gitYAML, "git package output should match embedded stdlib output")
		})
	}
}

func TestGitPackageK8s_SubpathImports(t *testing.T) {
	dir := t.TempDir()
	setupGitPackage(t, dir)

	entry := filepath.Join(dir, "ct.ts")
	require.NoError(t, os.WriteFile(entry, []byte(`
import { deployment, daemonSet } from "github.com/cloudticon/k8s/apps/v1";
import { service, configMap, secret } from "github.com/cloudticon/k8s/core/v1";
import { ingress } from "github.com/cloudticon/k8s/networking/v1";

deployment({ name: "web", image: "nginx", ports: [{ containerPort: 80 }] });
daemonSet({ name: "log-agent", image: "fluentd:latest" });
service({ name: "web-svc", selector: { app: "web" }, ports: [{ port: 80 }] });
configMap({ name: "cfg", data: { key: "value" } });
secret({ name: "tls", stringData: { cert: "abc" } });
ingress({ name: "web-ing", host: "web.example.com", serviceName: "web-svc" });
`), 0o644))

	tr := engine.NewTranspiler(k8s.Stdlib, dir)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Namespace: "default",
	})
	require.NoError(t, err)
	require.Len(t, resources, 6)

	kinds := make([]string, len(resources))
	for i, r := range resources {
		kinds[i] = r["kind"].(string)
	}
	assert.Equal(t, []string{"Deployment", "DaemonSet", "Service", "ConfigMap", "Secret", "Ingress"}, kinds)
}

func TestGitPackageK8s_IndexImport(t *testing.T) {
	dir := t.TempDir()
	setupGitPackage(t, dir)

	entry := filepath.Join(dir, "ct.ts")
	require.NoError(t, os.WriteFile(entry, []byte(`
import { deployment, service, ingress, resource } from "github.com/cloudticon/k8s";

deployment({ name: "app", image: "nginx" });
service({ name: "app-svc", selector: { app: "app" }, ports: [{ port: 80 }] });
ingress({ name: "app-ing", host: "app.example.com", serviceName: "app-svc" });
resource({
  apiVersion: "cert-manager.io/v1",
  kind: "Certificate",
  metadata: { name: "app-cert" },
  spec: { secretName: "app-tls", issuerRef: { name: "letsencrypt" } },
});
`), 0o644))

	tr := engine.NewTranspiler(k8s.Stdlib, dir)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Namespace: "default",
	})
	require.NoError(t, err)
	require.Len(t, resources, 4)

	assert.Equal(t, "Deployment", resources[0]["kind"])
	assert.Equal(t, "Service", resources[1]["kind"])
	assert.Equal(t, "Ingress", resources[2]["kind"])
	assert.Equal(t, "Certificate", resources[3]["kind"])
}
