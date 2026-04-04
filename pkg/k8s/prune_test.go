package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeOrphaned_EmptyOldRefs(t *testing.T) {
	orphaned := ComputeOrphaned(nil, []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	})

	assert.Empty(t, orphaned)
}

func TestComputeOrphaned_NoOrphans(t *testing.T) {
	oldRefs := []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
	}
	newRefs := []ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}

	orphaned := ComputeOrphaned(oldRefs, newRefs)
	assert.Empty(t, orphaned)
}

func TestComputeOrphaned_ReturnsRemovedResources(t *testing.T) {
	oldRefs := []ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
		{APIVersion: "v1", Kind: "Service", Namespace: "prod", Name: "web-svc"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}
	newRefs := []ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "prod", Name: "web"},
	}

	orphaned := ComputeOrphaned(oldRefs, newRefs)
	assert.Equal(t, []ResourceRef{
		{APIVersion: "v1", Kind: "Service", Namespace: "prod", Name: "web-svc"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}, orphaned)
}

func TestComputeOrphaned_SupportsClusterScopedResources(t *testing.T) {
	oldRefs := []ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: "prod"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}
	newRefs := []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}

	orphaned := ComputeOrphaned(oldRefs, newRefs)
	assert.Equal(t, []ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: "prod"},
	}, orphaned)
}

func TestComputeOrphaned_DeduplicatesOldRefs(t *testing.T) {
	oldRefs := []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}

	orphaned := ComputeOrphaned(oldRefs, nil)
	assert.Equal(t, []ResourceRef{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "prod", Name: "cfg"},
	}, orphaned)
}
