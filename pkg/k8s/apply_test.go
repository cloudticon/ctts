package k8s

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newTestClient(t *testing.T, apiResources []*metav1.APIResourceList) (*Client, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)

	dynClient.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(k8stesting.PatchAction)
		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(patchAction.GetPatch(), &obj.Object); err != nil {
			return true, nil, err
		}
		return true, obj, nil
	})

	fakeClient := fake.NewSimpleClientset()
	fakeClient.Fake.Resources = apiResources

	c := &Client{
		Clientset: fakeClient,
		Dynamic:   dynClient,
		Namespace: "default",
		gvrCache:  make(map[string]*resourceInfo),
	}

	return c, dynClient
}

func TestToUnstructured(t *testing.T) {
	res := Resource{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "web", "namespace": "test"},
		"spec":       map[string]interface{}{"replicas": int64(3)},
	}

	obj := toUnstructured(res)

	assert.Equal(t, "apps/v1", obj.GetAPIVersion())
	assert.Equal(t, "Deployment", obj.GetKind())
	assert.Equal(t, "web", obj.GetName())
	assert.Equal(t, "test", obj.GetNamespace())
}

func TestToUnstructured_MinimalResource(t *testing.T) {
	res := Resource{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "cfg"},
	}

	obj := toUnstructured(res)

	assert.Equal(t, "v1", obj.GetAPIVersion())
	assert.Equal(t, "ConfigMap", obj.GetKind())
	assert.Equal(t, "cfg", obj.GetName())
	assert.Equal(t, "", obj.GetNamespace())
}

func TestResolveResourceInfo_Cached(t *testing.T) {
	c := &Client{
		gvrCache: map[string]*resourceInfo{
			"apps/v1/Deployment": {
				GVR:        schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespaced: true,
			},
		},
	}

	info, err := c.resolveResourceInfo("apps/v1", "Deployment")
	require.NoError(t, err)
	assert.Equal(t, "deployments", info.GVR.Resource)
	assert.True(t, info.Namespaced)
}

func TestResolveResourceInfo_Discovery(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.Fake.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
				{Name: "statefulsets", Kind: "StatefulSet", Namespaced: true},
				{Name: "daemonsets", Kind: "DaemonSet", Namespaced: true},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "services", Kind: "Service", Namespaced: true},
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	}

	c := &Client{
		Clientset: fakeClient,
		gvrCache:  make(map[string]*resourceInfo),
	}

	info, err := c.resolveResourceInfo("apps/v1", "Deployment")
	require.NoError(t, err)
	assert.Equal(t, schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, info.GVR)
	assert.True(t, info.Namespaced)

	info2, err := c.resolveResourceInfo("v1", "Namespace")
	require.NoError(t, err)
	assert.Equal(t, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, info2.GVR)
	assert.False(t, info2.Namespaced)

	info3, err := c.resolveResourceInfo("apps/v1", "Deployment")
	require.NoError(t, err)
	assert.Equal(t, info, info3)
}

func TestResolveResourceInfo_KindNotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.Fake.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
	}

	c := &Client{
		Clientset: fakeClient,
		gvrCache:  make(map[string]*resourceInfo),
	}

	_, err := c.resolveResourceInfo("apps/v1", "NonExistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApply_NamespacedResource(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	})
	c.Namespace = "test-ns"

	resources := []Resource{
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "test-ns",
			},
			"data": map[string]interface{}{"key": "value"},
		},
	}

	err := c.Apply(context.Background(), resources)
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 1)
	assert.Equal(t, "patch", actions[0].GetVerb())
	assert.Equal(t, "test-ns", actions[0].GetNamespace())
}

func TestApply_FallsBackToClientNamespace(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	})
	c.Namespace = "fallback-ns"

	resources := []Resource{
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "no-ns-config"},
		},
	}

	err := c.Apply(context.Background(), resources)
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 1)
	assert.Equal(t, "fallback-ns", actions[0].GetNamespace())
}

func TestApply_ClusterScopedResource(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
			},
		},
	})

	resources := []Resource{
		{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "new-ns"},
		},
	}

	err := c.Apply(context.Background(), resources)
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 1)
	assert.Equal(t, "patch", actions[0].GetVerb())
	assert.Equal(t, "", actions[0].GetNamespace())
}

func TestApply_MultipleResources(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
				{Name: "services", Kind: "Service", Namespaced: true},
			},
		},
	})
	c.Namespace = "multi-ns"

	resources := []Resource{
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "cfg1", "namespace": "multi-ns"},
		},
		{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": "svc1", "namespace": "multi-ns"},
		},
	}

	err := c.Apply(context.Background(), resources)
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 2)
}

func TestApply_EmptyResources(t *testing.T) {
	c := &Client{
		gvrCache: make(map[string]*resourceInfo),
	}

	err := c.Apply(context.Background(), []Resource{})
	require.NoError(t, err)
}

func TestApply_MixedScopes(t *testing.T) {
	c, dynClient := newTestClient(t, []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	})
	c.Namespace = "my-ns"

	resources := []Resource{
		{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "my-ns"},
		},
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "cfg", "namespace": "my-ns"},
		},
	}

	err := c.Apply(context.Background(), resources)
	require.NoError(t, err)

	actions := dynClient.Actions()
	require.Len(t, actions, 2)
	assert.Equal(t, "", actions[0].GetNamespace())
	assert.Equal(t, "my-ns", actions[1].GetNamespace())
}
