package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/engine"
	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_DeploymentFromTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
import { deployment } from "ctts/k8s/apps/v1";

deployment({
  name: "web-app",
  image: Values.image,
  replicas: Values.replicas,
  ports: [{ containerPort: 8080 }],
});
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode: js,
		Values: map[string]interface{}{
			"image":    "nginx:1.25",
			"replicas": 3,
		},
		Namespace: "production",
	})
	require.NoError(t, err)
	require.Len(t, resources, 1)

	res := resources[0]
	assert.Equal(t, "apps/v1", res["apiVersion"])
	assert.Equal(t, "Deployment", res["kind"])

	meta := res["metadata"].(map[string]interface{})
	assert.Equal(t, "web-app", meta["name"])
	assert.Equal(t, "production", meta["namespace"])

	spec := res["spec"].(map[string]interface{})
	assert.Equal(t, int64(3), spec["replicas"])

	tmpl := spec["template"].(map[string]interface{})
	tmplSpec := tmpl["spec"].(map[string]interface{})
	containers := tmplSpec["containers"].([]interface{})
	container := containers[0].(map[string]interface{})
	assert.Equal(t, "nginx:1.25", container["image"])
}

func TestIntegration_MultipleResourcesWithCrossRef(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";

const app = deployment({
  name: "api",
  image: "api:latest",
  ports: [{ containerPort: 3000 }],
});

service({
  name: "api-svc",
  selector: { app: app.metadata.name },
  ports: [{ port: 80, targetPort: 3000 }],
});
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Namespace: "default",
	})
	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "Deployment", resources[0]["kind"])
	assert.Equal(t, "Service", resources[1]["kind"])

	svcSpec := resources[1]["spec"].(map[string]interface{})
	selector := svcSpec["selector"].(map[string]interface{})
	assert.Equal(t, "api", selector["app"])
}

func TestIntegration_ConditionalResources(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
import { deployment } from "ctts/k8s/apps/v1";
import { ingress } from "ctts/k8s/networking/v1";

deployment({ name: "app", image: "nginx" });

if (Values.enableIngress) {
  ingress({
    name: "app-ingress",
    host: Values.domain,
    serviceName: "app-svc",
  });
}
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)

	js, err := tr.Bundle(entry)
	require.NoError(t, err)
	enabledRes, err := engine.Execute(engine.ExecuteOpts{
		JSCode: js,
		Values: map[string]interface{}{
			"enableIngress": true,
			"domain":        "app.example.com",
		},
		Namespace: "prod",
	})
	require.NoError(t, err)
	assert.Len(t, enabledRes, 2)

	disabledRes, err := engine.Execute(engine.ExecuteOpts{
		JSCode: js,
		Values: map[string]interface{}{
			"enableIngress": false,
		},
		Namespace: "prod",
	})
	require.NoError(t, err)
	assert.Len(t, disabledRes, 1)
}

func TestIntegration_LoopResources(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
import { deployment } from "ctts/k8s/apps/v1";

for (const w of Values.workers) {
  deployment({
    name: "worker-" + w.name,
    image: w.image,
    replicas: w.replicas,
  });
}
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode: js,
		Values: map[string]interface{}{
			"workers": []interface{}{
				map[string]interface{}{"name": "email", "image": "worker:1.0", "replicas": 2},
				map[string]interface{}{"name": "pdf", "image": "worker:1.0", "replicas": 1},
			},
		},
		Namespace: "default",
	})
	require.NoError(t, err)
	require.Len(t, resources, 2)

	meta0 := resources[0]["metadata"].(map[string]interface{})
	meta1 := resources[1]["metadata"].(map[string]interface{})
	assert.Equal(t, "worker-email", meta0["name"])
	assert.Equal(t, "worker-pdf", meta1["name"])
}

func TestIntegration_LowLevelResource(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
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
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Namespace: "redis-ns",
	})
	require.NoError(t, err)
	require.Len(t, resources, 1)

	res := resources[0]
	assert.Equal(t, "redis.redis.opstreelabs.in/v1beta2", res["apiVersion"])
	assert.Equal(t, "Redis", res["kind"])

	meta := res["metadata"].(map[string]interface{})
	assert.Equal(t, "redis-ns", meta["namespace"])
}

func TestIntegration_ClusterScopedMixed(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "ct.ts")
	err := os.WriteFile(entry, []byte(`
import { namespace } from "ctts/k8s/core/v1";
import { configMap } from "ctts/k8s/core/v1";

namespace({ name: "production" });
configMap({
  name: "app-config",
  data: { env: "production" },
});
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Namespace: "production",
	})
	require.NoError(t, err)
	require.Len(t, resources, 2)

	nsMeta := resources[0]["metadata"].(map[string]interface{})
	_, nsHasNamespace := nsMeta["namespace"]
	assert.False(t, nsHasNamespace, "Namespace resource should not get namespace")

	cmMeta := resources[1]["metadata"].(map[string]interface{})
	assert.Equal(t, "production", cmMeta["namespace"])
}

func TestIntegration_FullPipeline_WithValuesFile(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  image: "myapp:2.0",
  replicas: 5,
};
`), 0644)
	require.NoError(t, err)

	entryFile := filepath.Join(dir, "ct.ts")
	err = os.WriteFile(entryFile, []byte(`
import { deployment } from "ctts/k8s/apps/v1";

deployment({
  name: "myapp",
  image: Values.image,
  replicas: Values.replicas,
});
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)

	values, err := engine.LoadValues(tr, valuesFile, []string{"replicas=10"})
	require.NoError(t, err)
	assert.Equal(t, int64(10), values["replicas"])

	js, err := tr.Bundle(entryFile)
	require.NoError(t, err)

	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    js,
		Values:    values,
		Namespace: "staging",
	})
	require.NoError(t, err)
	require.Len(t, resources, 1)

	spec := resources[0]["spec"].(map[string]interface{})
	assert.Equal(t, int64(10), spec["replicas"])

	meta := resources[0]["metadata"].(map[string]interface{})
	assert.Equal(t, "staging", meta["namespace"])
}
