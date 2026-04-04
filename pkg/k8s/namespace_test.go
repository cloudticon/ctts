package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureNamespace_CreatesWhenMissing(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &Client{CoreV1: fakeClient.CoreV1()}

	err := EnsureNamespace(context.Background(), client, "dev")
	require.NoError(t, err)

	ns, err := client.CoreV1.Namespaces().Get(context.Background(), "dev", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "ct", ns.Labels[inventoryManagedByLabelKey])
}

func TestEnsureNamespace_NoOpWhenExists(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "dev"},
	})
	client := &Client{CoreV1: fakeClient.CoreV1()}

	err := EnsureNamespace(context.Background(), client, "dev")
	require.NoError(t, err)

	var createCount int
	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "namespaces" {
			createCount++
		}
	}
	assert.Equal(t, 0, createCount)
}

func TestEnsureNamespace_ReturnsErrorWhenGetFails(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	client := &Client{CoreV1: fakeClient.CoreV1()}

	err := EnsureNamespace(context.Background(), client, "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting namespace")
}

func TestEnsureNamespace_IgnoresAlreadyExistsOnCreate(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
		err := apierrors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "namespaces"}, "dev")
		return true, nil, err
	})
	client := &Client{CoreV1: fakeClient.CoreV1()}

	err := EnsureNamespace(context.Background(), client, "dev")
	require.NoError(t, err)
}

func TestEnsureNamespace_Validation(t *testing.T) {
	err := EnsureNamespace(context.Background(), nil, "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8s client is required")

	client := &Client{CoreV1: fake.NewSimpleClientset().CoreV1()}
	err = EnsureNamespace(context.Background(), client, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")
}
