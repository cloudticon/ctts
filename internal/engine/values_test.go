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

func TestLoadValues_BasicExport(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  image: "nginx:1.25",
  replicas: 3,
  debug: false,
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, nil)
	require.NoError(t, err)

	assert.Equal(t, "nginx:1.25", values["image"])
	assert.Equal(t, int64(3), values["replicas"])
	assert.Equal(t, false, values["debug"])
}

func TestLoadValues_NestedObject(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  app: {
    name: "web",
    port: 8080,
  },
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, nil)
	require.NoError(t, err)

	app := values["app"].(map[string]interface{})
	assert.Equal(t, "web", app["name"])
	assert.Equal(t, int64(8080), app["port"])
}

func TestLoadValues_WithSetOverrides(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  image: "nginx:1.25",
  replicas: 3,
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, []string{
		"replicas=5",
		"image=nginx:1.26",
	})
	require.NoError(t, err)

	assert.Equal(t, int64(5), values["replicas"])
	assert.Equal(t, "nginx:1.26", values["image"])
}

func TestLoadValues_SetOverrideNested(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  app: {
    name: "web",
    port: 8080,
  },
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, []string{
		"app.port=9090",
	})
	require.NoError(t, err)

	app := values["app"].(map[string]interface{})
	assert.Equal(t, int64(9090), app["port"])
	assert.Equal(t, "web", app["name"])
}

func TestLoadValues_SetOverrideBooleans(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  debug: false,
  verbose: true,
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, []string{
		"debug=true",
		"verbose=false",
	})
	require.NoError(t, err)

	assert.Equal(t, true, values["debug"])
	assert.Equal(t, false, values["verbose"])
}

func TestLoadValues_SetOverrideCreatesNestedPath(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`export default {};`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, []string{
		"a.b.c=hello",
	})
	require.NoError(t, err)

	a := values["a"].(map[string]interface{})
	b := a["b"].(map[string]interface{})
	assert.Equal(t, "hello", b["c"])
}

func TestLoadValues_InvalidSetFormat(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`export default {};`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	_, err = engine.LoadValues(tr, valuesFile, []string{"invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --set format")
}

func TestLoadValues_WithArray(t *testing.T) {
	dir := t.TempDir()
	valuesFile := filepath.Join(dir, "values.ts")
	err := os.WriteFile(valuesFile, []byte(`
export default {
  workers: [
    { name: "email", replicas: 2 },
    { name: "pdf", replicas: 1 },
  ],
};
`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler(k8s.Stdlib, "")
	values, err := engine.LoadValues(tr, valuesFile, nil)
	require.NoError(t, err)

	workers := values["workers"].([]interface{})
	assert.Len(t, workers, 2)

	w0 := workers[0].(map[string]interface{})
	assert.Equal(t, "email", w0["name"])
	assert.Equal(t, int64(2), w0["replicas"])
}

func TestLoadValues_InvalidFile(t *testing.T) {
	tr := engine.NewTranspiler(k8s.Stdlib, "")
	_, err := engine.LoadValues(tr, "/nonexistent/values.ts", nil)
	assert.Error(t, err)
}
