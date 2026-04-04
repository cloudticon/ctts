package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func TestOrderForDelete_SafetyOrder(t *testing.T) {
	input := []ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: "prod"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "ct-inventory-prod"},
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
		{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole", Name: "web-role"},
	}

	ordered := orderForDelete(input)
	assert.Equal(t, []ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
		{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole", Name: "web-role"},
		{APIVersion: "v1", Kind: "Namespace", Name: "prod"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "ct-inventory-prod"},
	}, ordered)
}

func TestDelete_ContinuesOnNotFound(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	})

	dynClient.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteAction := action.(k8stesting.DeleteAction)
		if deleteAction.GetName() == "missing" {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "missing")
		}
		return false, nil, nil
	})

	err := c.Delete(context.Background(), []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "missing"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "existing"},
	})
	require.NoError(t, err)
}

func TestDelete_UsesClusterScopedClient(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
			},
		},
	})

	err := c.Delete(context.Background(), []ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: "prod"},
	})
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 1)
	assert.Equal(t, "delete", actions[0].GetVerb())
	assert.Equal(t, "", actions[0].GetNamespace())
}

func TestDelete_ReturnsJoinedErrors(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	})

	dynClient.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	err := c.Delete(context.Background(), []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cfg1"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cfg2"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting resources")
	assert.Contains(t, err.Error(), "boom")
}
