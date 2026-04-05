package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyCmd_MissingMainCt(t *testing.T) {
	dir := t.TempDir()

	cmd := newApplyCmd()
	cmd.SetArgs([]string{"my-release", dir})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}

func TestApplyCmd_RequiresExactlyTwoArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"zero args", []string{}},
		{"one arg", []string{"my-release"}},
		{"three args", []string{"my-release", ".", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newApplyCmd()
			cmd.SetArgs(tt.args)
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "accepts 2 arg(s)")
		})
	}
}

func TestApplyCmd_DefaultFlags(t *testing.T) {
	cmd := newApplyCmd()

	ns, _ := cmd.Flags().GetString("namespace")
	assert.Equal(t, "", ns)

	ctx, _ := cmd.Flags().GetString("context")
	assert.Equal(t, "", ctx)

	outputFmt, _ := cmd.Flags().GetString("output")
	assert.Equal(t, "", outputFmt)

	noCache, _ := cmd.Flags().GetBool("no-cache")
	assert.False(t, noCache)

	createNamespace, _ := cmd.Flags().GetBool("create-namespace")
	assert.False(t, createNamespace)
}

func TestApplyCmd_UsageShowsTwoArgs(t *testing.T) {
	cmd := newApplyCmd()
	assert.Contains(t, cmd.Use, "<name>")
	assert.Contains(t, cmd.Use, "<dir|repo>")
}

func TestApplyCmd_MissingMainCt_WithReleaseName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, scaffold.Init(dir))

	cmd := newApplyCmd()
	cmd.SetArgs([]string{"prod-release", t.TempDir()})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}

func TestEnsureApplyNamespace_SkipsWhenDisabledOrNamespaceEmpty(t *testing.T) {
	origEnsure := ensureNamespaceForApply
	t.Cleanup(func() {
		ensureNamespaceForApply = origEnsure
	})

	var calls int
	ensureNamespaceForApply = func(ctx context.Context, client *k8s.Client, namespace string) error {
		calls++
		return nil
	}

	err := ensureApplyNamespace(context.Background(), &k8s.Client{}, "dev", false)
	require.NoError(t, err)

	err = ensureApplyNamespace(context.Background(), &k8s.Client{}, "", true)
	require.NoError(t, err)

	assert.Equal(t, 0, calls)
}

func TestRunApply_CreatesNamespaceWhenEnabled(t *testing.T) {
	origResolveSourceDir := resolveSourceDirForApply
	origRenderResources := renderResourcesForApply
	origInjectLabels := injectReleaseLabelsForApply
	origNewClient := newK8sClientForApply
	origEnsureNamespace := ensureNamespaceForApply
	origApplyRelease := applyReleaseForApply
	t.Cleanup(func() {
		resolveSourceDirForApply = origResolveSourceDir
		renderResourcesForApply = origRenderResources
		injectReleaseLabelsForApply = origInjectLabels
		newK8sClientForApply = origNewClient
		ensureNamespaceForApply = origEnsureNamespace
		applyReleaseForApply = origApplyRelease
	})

	expectedClient := &k8s.Client{}
	var ensuredNamespace string

	resolveSourceDirForApply = func(source string, noCache bool) (string, error) {
		return "/tmp/fake-source", nil
	}
	renderResourcesForApply = func(dir string, opts templateOpts) ([]k8s.Resource, error) {
		return []k8s.Resource{
			{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "cfg",
					"namespace": "dev",
				},
			},
		}, nil
	}
	injectReleaseLabelsForApply = func(resources []k8s.Resource, releaseName string) []k8s.Resource {
		return resources
	}
	newK8sClientForApply = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	ensureNamespaceForApply = func(ctx context.Context, client *k8s.Client, namespace string) error {
		assert.Same(t, expectedClient, client)
		ensuredNamespace = namespace
		return nil
	}
	applyReleaseForApply = func(ctx context.Context, client *k8s.Client, namespace, releaseName string, resources []k8s.Resource) error {
		return nil
	}

	err := runApply(&cobra.Command{}, "my-release", ".", applyOpts{
		templateOpts: templateOpts{
			namespace: "dev",
		},
		createNamespace: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "dev", ensuredNamespace)
}

func TestEnsureApplyNamespace_ReturnsWrappedError(t *testing.T) {
	origEnsure := ensureNamespaceForApply
	t.Cleanup(func() {
		ensureNamespaceForApply = origEnsure
	})

	ensureNamespaceForApply = func(ctx context.Context, client *k8s.Client, namespace string) error {
		return errors.New("boom")
	}

	err := ensureApplyNamespace(context.Background(), &k8s.Client{}, "dev", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `ensuring namespace "dev"`)
}
