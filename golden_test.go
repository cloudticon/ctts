package ctts_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/engine"
	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/cloudticon/ctts/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func runPipeline(t *testing.T, dir, namespace string) string {
	t.Helper()

	entryPoint := filepath.Join(dir, "ct.ts")
	valuesPath := filepath.Join(dir, "values.ts")

	tr := engine.NewTranspiler(k8s.Stdlib)

	var values map[string]interface{}
	if _, err := os.Stat(valuesPath); err == nil {
		var loadErr error
		values, loadErr = engine.LoadValues(tr, valuesPath, nil)
		require.NoError(t, loadErr)
	}

	js, err := tr.Bundle(entryPoint)
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

func TestGolden(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
	}{
		{name: "simple", namespace: "default"},
		{name: "no_values", namespace: "default"},
		{name: "multi_resource", namespace: "production"},
		{name: "low_level_resource", namespace: "redis-ns"},
		{name: "conditional", namespace: "production"},
		{name: "loop", namespace: "default"},
		{name: "cross_ref", namespace: "default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)
			expectedPath := filepath.Join(dir, "expected.yaml")

			got := runPipeline(t, dir, tc.namespace)

			if *update {
				err := os.WriteFile(expectedPath, []byte(got), 0644)
				require.NoError(t, err)
				t.Logf("updated golden file: %s", expectedPath)
				return
			}

			expected, err := os.ReadFile(expectedPath)
			require.NoError(t, err, "golden file %s not found; run with -update to create", expectedPath)

			assert.Equal(t, string(expected), got)
		})
	}
}
