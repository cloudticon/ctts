package k8s_test

import (
	"strings"
	"testing"

	"github.com/cloudticon/ctts/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdlibEmbed_ContainsAllFiles(t *testing.T) {
	files := []string{
		"stdlib/k8s/resource.ts",
		"stdlib/k8s/apps/v1.ts",
		"stdlib/k8s/core/v1.ts",
		"stdlib/k8s/networking/v1.ts",
	}
	for _, f := range files {
		data, err := k8s.Stdlib.ReadFile(f)
		require.NoError(t, err, "failed to read embedded file: %s", f)
		assert.NotEmpty(t, data, "embedded file should not be empty: %s", f)
	}
}

func TestStdlibEmbed_ResourceExports(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/resource.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, "export function resource("))
	assert.True(t, strings.Contains(content, "export function resourceClusterScope("))
	assert.True(t, strings.Contains(content, "__ct_resources"))
}

func TestStdlibEmbed_AppsV1Exports(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/apps/v1.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, "export function deployment("))
	assert.True(t, strings.Contains(content, "export function statefulSet("))
	assert.True(t, strings.Contains(content, "export function daemonSet("))
}

func TestStdlibEmbed_CoreV1Exports(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/core/v1.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, "export function service("))
	assert.True(t, strings.Contains(content, "export function configMap("))
	assert.True(t, strings.Contains(content, "export function secret("))
	assert.True(t, strings.Contains(content, "export function namespace("))
}

func TestStdlibEmbed_NetworkingV1Exports(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/networking/v1.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, "export function ingress("))
}

func TestStdlibEmbed_RegistryModel(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/resource.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, `__ctts_scope: "namespaced"`))
	assert.True(t, strings.Contains(content, `__ctts_scope: "cluster"`))
}

func TestStdlibEmbed_NamespaceIsClusterScoped(t *testing.T) {
	data, err := k8s.Stdlib.ReadFile("stdlib/k8s/core/v1.ts")
	require.NoError(t, err)
	content := string(data)
	assert.True(t, strings.Contains(content, "resourceClusterScope"))
}
