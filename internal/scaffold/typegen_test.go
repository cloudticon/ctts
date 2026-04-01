package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateValuesDts_SimpleTypes(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  image: "nginx:1.25",
  replicas: 3,
  debug: true,
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "declare const Values: {")
	assert.Contains(t, s, "debug: boolean")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
}

func TestGenerateValuesDts_NestedObject(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  app: {
    name: "web",
    port: 8080,
  },
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "app: {")
	assert.Contains(t, s, "name: string")
	assert.Contains(t, s, "port: number")
}

func TestGenerateValuesDts_ArrayOfObjects(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  workers: [
    { name: "email", replicas: 2 },
    { name: "pdf", replicas: 1 },
  ],
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "workers: {")
	assert.Contains(t, s, "name: string")
	assert.Contains(t, s, "replicas: number")
	assert.Contains(t, s, "}[]")
}

func TestGenerateValuesDts_ArrayOfPrimitives(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  tags: ["web", "api"],
  ports: [80, 443],
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "ports: number[]")
	assert.Contains(t, s, "tags: string[]")
}

func TestGenerateValuesDts_EmptyObject(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "declare const Values: {};")
}

func TestGenerateValuesDts_EmptyArray(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  items: [],
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "items: never[]")
}

func TestGenerateValuesDts_MixedComplex(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export default {
  image: "nginx:1.25",
  replicas: 3,
  enableIngress: true,
  domain: "app.example.com",
  workers: [
    { name: "email", image: "worker:1.0", replicas: 2 },
    { name: "pdf", image: "worker:1.0", replicas: 1 },
  ],
};`), 0o644))

	require.NoError(t, scaffold.GenerateValuesDts(valuesPath, outputPath))

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "declare const Values: {")
	assert.Contains(t, s, "domain: string")
	assert.Contains(t, s, "enableIngress: boolean")
	assert.Contains(t, s, "image: string")
	assert.Contains(t, s, "replicas: number")
	assert.Contains(t, s, "workers: {")
}

func TestGenerateValuesDts_ErrorOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "nonexistent.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	err := scaffold.GenerateValuesDts(valuesPath, outputPath)
	assert.Error(t, err)
}

func TestGenerateValuesDts_ErrorOnInvalidTS(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`this is not valid typescript @@@ {{{`), 0o644))

	err := scaffold.GenerateValuesDts(valuesPath, outputPath)
	assert.Error(t, err)
}

func TestGenerateValuesDts_ErrorOnNoDefaultExport(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.ts")
	outputPath := filepath.Join(dir, "values.d.ts")

	require.NoError(t, os.WriteFile(valuesPath, []byte(`export const foo = 42;`), 0o644))

	err := scaffold.GenerateValuesDts(valuesPath, outputPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default export")
}
