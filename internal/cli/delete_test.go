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

func TestDeleteCmd_RequiresReleaseName(t *testing.T) {
	cmd := newDeleteCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestDeleteCmd_DefaultFlags(t *testing.T) {
	cmd := newDeleteCmd()

	ns, err := cmd.Flags().GetString("namespace")
	require.NoError(t, err)
	assert.Equal(t, "", ns)

	kubeContext, err := cmd.Flags().GetString("context")
	require.NoError(t, err)
	assert.Equal(t, "", kubeContext)
}

func TestRunDelete_DeletesResourcesAndInventory(t *testing.T) {
	origNewClient := newK8sClientForDelete
	origLoadInventory := loadInventoryForDelete
	origDeleteResources := deleteResourcesForDelete
	origDeleteInventory := deleteInventoryForDelete
	t.Cleanup(func() {
		newK8sClientForDelete = origNewClient
		loadInventoryForDelete = origLoadInventory
		deleteResourcesForDelete = origDeleteResources
		deleteInventoryForDelete = origDeleteInventory
	})

	expectedClient := &k8s.Client{}
	expectedResources := []k8s.ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
		{APIVersion: "v1", Kind: "Service", Namespace: "prod", Name: "web-svc"},
	}
	var flow []string

	newK8sClientForDelete = func(kubeContext, namespace string) (*k8s.Client, error) {
		assert.Equal(t, "staging", kubeContext)
		assert.Equal(t, "prod", namespace)
		flow = append(flow, "new-client")
		return expectedClient, nil
	}
	loadInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) ([]k8s.ResourceRef, error) {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, "prod", namespace)
		assert.Equal(t, "my-release", releaseName)
		flow = append(flow, "load-inventory")
		return expectedResources, nil
	}
	deleteResourcesForDelete = func(ctx context.Context, client *k8s.Client, resources []k8s.ResourceRef) error {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, expectedResources, resources)
		flow = append(flow, "delete-resources")
		return nil
	}
	deleteInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) error {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, "prod", namespace)
		assert.Equal(t, "my-release", releaseName)
		flow = append(flow, "delete-inventory")
		return nil
	}

	stderr := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetErr(stderr)

	err := runDelete(cmd, "my-release", deleteOpts{
		namespace: "prod",
		context:   "staging",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"new-client", "load-inventory", "delete-resources", "delete-inventory"}, flow)
	assert.Contains(t, stderr.String(), "deleted release my-release (2 resources)")
}

func TestRunDelete_ReturnsErrorWhenClientCreationFails(t *testing.T) {
	origNewClient := newK8sClientForDelete
	t.Cleanup(func() {
		newK8sClientForDelete = origNewClient
	})

	newK8sClientForDelete = func(kubeContext, namespace string) (*k8s.Client, error) {
		return nil, errors.New("boom")
	}

	err := runDelete(&cobra.Command{}, "my-release", deleteOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating k8s client")
}

func TestRunDelete_ReturnsErrorWhenLoadInventoryFails(t *testing.T) {
	origNewClient := newK8sClientForDelete
	origLoadInventory := loadInventoryForDelete
	t.Cleanup(func() {
		newK8sClientForDelete = origNewClient
		loadInventoryForDelete = origLoadInventory
	})

	expectedClient := &k8s.Client{}
	newK8sClientForDelete = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	loadInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) ([]k8s.ResourceRef, error) {
		return nil, errors.New("inventory failure")
	}

	err := runDelete(&cobra.Command{}, "my-release", deleteOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading inventory for release")
}

func TestRunDelete_ReturnsErrorWhenDeleteResourcesFails(t *testing.T) {
	origNewClient := newK8sClientForDelete
	origLoadInventory := loadInventoryForDelete
	origDeleteResources := deleteResourcesForDelete
	t.Cleanup(func() {
		newK8sClientForDelete = origNewClient
		loadInventoryForDelete = origLoadInventory
		deleteResourcesForDelete = origDeleteResources
	})

	expectedClient := &k8s.Client{}
	newK8sClientForDelete = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	loadInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) ([]k8s.ResourceRef, error) {
		return []k8s.ResourceRef{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"}}, nil
	}
	deleteResourcesForDelete = func(ctx context.Context, client *k8s.Client, resources []k8s.ResourceRef) error {
		return errors.New("delete failed")
	}

	err := runDelete(&cobra.Command{}, "my-release", deleteOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting release resources")
}

func TestRunDelete_ReturnsErrorWhenDeleteInventoryFails(t *testing.T) {
	origNewClient := newK8sClientForDelete
	origLoadInventory := loadInventoryForDelete
	origDeleteResources := deleteResourcesForDelete
	origDeleteInventory := deleteInventoryForDelete
	t.Cleanup(func() {
		newK8sClientForDelete = origNewClient
		loadInventoryForDelete = origLoadInventory
		deleteResourcesForDelete = origDeleteResources
		deleteInventoryForDelete = origDeleteInventory
	})

	expectedClient := &k8s.Client{}
	newK8sClientForDelete = func(kubeContext, namespace string) (*k8s.Client, error) {
		return expectedClient, nil
	}
	loadInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) ([]k8s.ResourceRef, error) {
		return []k8s.ResourceRef{}, nil
	}
	deleteResourcesForDelete = func(ctx context.Context, client *k8s.Client, resources []k8s.ResourceRef) error {
		return nil
	}
	deleteInventoryForDelete = func(ctx context.Context, client *k8s.Client, namespace, releaseName string) error {
		return errors.New("cleanup failed")
	}

	err := runDelete(&cobra.Command{}, "my-release", deleteOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting release inventory")
}
