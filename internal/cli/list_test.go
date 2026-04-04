package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCmd_RequiresNoArgs(t *testing.T) {
	cmd := newListCmd()
	cmd.SetArgs([]string{"unexpected"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command "unexpected" for "list"`)
}

func TestListCmd_DefaultFlags(t *testing.T) {
	cmd := newListCmd()

	ns, err := cmd.Flags().GetString("namespace")
	require.NoError(t, err)
	assert.Equal(t, "", ns)

	allNamespaces, err := cmd.Flags().GetBool("all-namespaces")
	require.NoError(t, err)
	assert.False(t, allNamespaces)

	kubeContext, err := cmd.Flags().GetString("context")
	require.NoError(t, err)
	assert.Equal(t, "", kubeContext)

	outputFmt, err := cmd.Flags().GetString("output")
	require.NoError(t, err)
	assert.Equal(t, "", outputFmt)
}

func TestRunList_TableOutput(t *testing.T) {
	origNewClient := newK8sClientForList
	origListReleases := listReleasesForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
		listReleasesForList = origListReleases
	})

	expectedClient := &k8s.Client{}
	expectedReleases := []k8s.ReleaseInfo{
		{Name: "api", Namespace: "prod", Resources: 3},
		{Name: "backend", Namespace: "staging", Resources: 1},
	}

	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		assert.Equal(t, "dev-cluster", kubeContext)
		assert.Equal(t, "prod", namespace)
		return expectedClient, nil
	}
	listReleasesForList = func(ctx context.Context, client *k8s.Client, namespace string, allNamespaces bool) ([]k8s.ReleaseInfo, error) {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, "prod", namespace)
		assert.False(t, allNamespaces)
		return expectedReleases, nil
	}

	stdout := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)

	err := runList(cmd, listOpts{
		namespace: "prod",
		context:   "dev-cluster",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "NAME")
	assert.Contains(t, stdout.String(), "api")
	assert.Contains(t, stdout.String(), "staging")
}

func TestRunList_JSONOutput(t *testing.T) {
	origNewClient := newK8sClientForList
	origListReleases := listReleasesForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
		listReleasesForList = origListReleases
	})

	expectedClient := &k8s.Client{}
	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	listReleasesForList = func(ctx context.Context, client *k8s.Client, namespace string, allNamespaces bool) ([]k8s.ReleaseInfo, error) {
		return []k8s.ReleaseInfo{{Name: "api", Namespace: "prod", Resources: 2}}, nil
	}

	stdout := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)

	err := runList(cmd, listOpts{outputFmt: "json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "api"`)
	assert.Contains(t, stdout.String(), `"resources": 2`)
}

func TestRunList_YAMLOutput(t *testing.T) {
	origNewClient := newK8sClientForList
	origListReleases := listReleasesForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
		listReleasesForList = origListReleases
	})

	expectedClient := &k8s.Client{}
	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	listReleasesForList = func(ctx context.Context, client *k8s.Client, namespace string, allNamespaces bool) ([]k8s.ReleaseInfo, error) {
		assert.True(t, allNamespaces)
		return []k8s.ReleaseInfo{{Name: "backend", Namespace: "staging", Resources: 1}}, nil
	}

	stdout := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)

	err := runList(cmd, listOpts{outputFmt: "yaml", allNamespaces: true})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "name: backend")
	assert.Contains(t, stdout.String(), "namespace: staging")
}

func TestRunList_ReturnsErrorWhenClientCreationFails(t *testing.T) {
	origNewClient := newK8sClientForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
	})

	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		return nil, errors.New("boom")
	}

	err := runList(&cobra.Command{}, listOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating k8s client")
}

func TestRunList_ReturnsErrorWhenListReleasesFails(t *testing.T) {
	origNewClient := newK8sClientForList
	origListReleases := listReleasesForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
		listReleasesForList = origListReleases
	})

	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		return &k8s.Client{}, nil
	}
	listReleasesForList = func(ctx context.Context, client *k8s.Client, namespace string, allNamespaces bool) ([]k8s.ReleaseInfo, error) {
		return nil, errors.New("list failure")
	}

	err := runList(&cobra.Command{}, listOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing releases")
}

func TestRunList_ReturnsErrorOnUnsupportedOutputFormat(t *testing.T) {
	origNewClient := newK8sClientForList
	origListReleases := listReleasesForList
	t.Cleanup(func() {
		newK8sClientForList = origNewClient
		listReleasesForList = origListReleases
	})

	newK8sClientForList = func(kubeContext, namespace string) (*k8s.Client, error) {
		return &k8s.Client{}, nil
	}
	listReleasesForList = func(ctx context.Context, client *k8s.Client, namespace string, allNamespaces bool) ([]k8s.ReleaseInfo, error) {
		return []k8s.ReleaseInfo{}, nil
	}

	err := runList(&cobra.Command{}, listOpts{outputFmt: "xml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}
