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

func TestBundle_SimpleTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`console.log(42 as number);`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)
	assert.Contains(t, js, "42")
	assert.NotContains(t, js, "as number")
}

func TestBundle_WithCttsImport(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	code := `
import { deployment } from "ctts/k8s/apps/v1";
const app = deployment({
  name: "test-app",
  image: "nginx:latest",
});
`
	err := os.WriteFile(entry, []byte(code), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "__ct_resources")
	assert.Contains(t, js, "test-app")
	assert.Contains(t, js, "nginx:latest")
	assert.Contains(t, js, "Deployment")
}

func TestBundle_WithResourceImport(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	code := `
import { resource } from "ctts/k8s/resource";
resource({
  apiVersion: "v1",
  kind: "ConfigMap",
  metadata: { name: "test-cm" },
  data: { key: "value" },
});
`
	err := os.WriteFile(entry, []byte(code), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "__ct_resources")
	assert.Contains(t, js, "test-cm")
	assert.Contains(t, js, "namespaced")
}

func TestBundle_ClusterScopedResource(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	code := `
import { resourceClusterScope } from "ctts/k8s/resource";
import { namespace } from "ctts/k8s/core/v1";
namespace({ name: "prod" });
resourceClusterScope({
  apiVersion: "v1",
  kind: "ClusterRole",
  metadata: { name: "admin" },
});
`
	err := os.WriteFile(entry, []byte(code), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "cluster")
	assert.Contains(t, js, "prod")
}

func TestBundle_InvalidTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`this is not valid { typescript`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	_, err = tr.Bundle(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "esbuild")
}

func TestBundle_IIFEFormat(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`const x = 1;`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "()")
}

func TestBundle_MultipleCttsImports(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	code := `
import { deployment } from "ctts/k8s/apps/v1";
import { service } from "ctts/k8s/core/v1";
import { ingress } from "ctts/k8s/networking/v1";

const app = deployment({ name: "web", image: "nginx" });
service({
  name: "web-svc",
  selector: { app: "web" },
  ports: [{ port: 80 }],
});
ingress({
  name: "web-ing",
  host: "example.com",
  serviceName: "web-svc",
});
`
	err := os.WriteFile(entry, []byte(code), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "Deployment")
	assert.Contains(t, js, "Service")
	assert.Contains(t, js, "Ingress")
}

func TestBundleValues_ExportsGlobalName(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "values.ts")
	err := os.WriteFile(entry, []byte(`export default { key: "val" };`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib)
	js, err := tr.BundleValues(entry)
	require.NoError(t, err)
	assert.Contains(t, js, "__values_export")
	assert.Contains(t, js, "val")
}
