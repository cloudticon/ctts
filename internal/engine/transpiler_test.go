package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundle_SimpleTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`console.log(42 as number);`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler("")
	js, err := tr.Bundle(entry)
	require.NoError(t, err)
	assert.Contains(t, js, "42")
	assert.NotContains(t, js, "as number")
}

func TestBundle_CtFileAsTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ct")
	err := os.WriteFile(entry, []byte(`const x: number = 42; console.log(x);`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler("")
	js, err := tr.Bundle(entry)
	require.NoError(t, err)
	assert.Contains(t, js, "42")
	assert.NotContains(t, js, ": number")
}

func TestBundle_InvalidTS(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`this is not valid { typescript`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler("")
	_, err = tr.Bundle(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "esbuild")
}

func TestBundle_IIFEFormat(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.ts")
	err := os.WriteFile(entry, []byte(`const x = 1;`), 0644)
	require.NoError(t, err)

	tr := engine.NewTranspiler("")
	js, err := tr.Bundle(entry)
	require.NoError(t, err)

	assert.Contains(t, js, "()")
}
