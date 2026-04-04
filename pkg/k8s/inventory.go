package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	inventoryManagedByLabelKey = "app.kubernetes.io/managed-by"
	inventoryInstanceLabelKey  = "ct.cloudticon.com/instance"
	inventoryResourcesDataKey  = "resources"
)

func SaveInventory(ctx context.Context, client *Client, namespace, releaseName string, resources []Resource) error {
	if client == nil || client.CoreV1 == nil {
		return errors.New("k8s client is required")
	}
	if releaseName == "" {
		return errors.New("release name is required")
	}

	targetNamespace, err := resolveInventoryNamespace(client, namespace)
	if err != nil {
		return err
	}

	refs, err := ResourcesToRefs(resources)
	if err != nil {
		return err
	}

	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return fmt.Errorf("marshaling inventory refs: %w", err)
	}

	patchObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      inventoryConfigMapName(releaseName),
			"namespace": targetNamespace,
			"labels": map[string]interface{}{
				inventoryManagedByLabelKey: "ct",
				inventoryInstanceLabelKey:  releaseName,
			},
		},
		"data": map[string]interface{}{
			inventoryResourcesDataKey: string(refsJSON),
		},
	}

	patchData, err := json.Marshal(patchObj)
	if err != nil {
		return fmt.Errorf("marshaling inventory configmap patch: %w", err)
	}

	force := true
	if _, err := client.CoreV1.ConfigMaps(targetNamespace).Patch(
		ctx,
		inventoryConfigMapName(releaseName),
		types.ApplyPatchType,
		patchData,
		metav1.PatchOptions{
			FieldManager: "ct",
			Force:        &force,
		},
	); err != nil {
		return fmt.Errorf("saving inventory configmap: %w", err)
	}

	return nil
}

func LoadInventory(ctx context.Context, client *Client, namespace, releaseName string) ([]ResourceRef, error) {
	if client == nil || client.CoreV1 == nil {
		return nil, errors.New("k8s client is required")
	}
	if releaseName == "" {
		return nil, errors.New("release name is required")
	}

	targetNamespace, err := resolveInventoryNamespace(client, namespace)
	if err != nil {
		return nil, err
	}

	cm, err := client.CoreV1.ConfigMaps(targetNamespace).Get(ctx, inventoryConfigMapName(releaseName), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []ResourceRef{}, nil
		}
		return nil, fmt.Errorf("loading inventory configmap: %w", err)
	}

	raw := cm.Data[inventoryResourcesDataKey]
	if raw == "" {
		return []ResourceRef{}, nil
	}

	var refs []ResourceRef
	if err := json.Unmarshal([]byte(raw), &refs); err != nil {
		return nil, fmt.Errorf("unmarshaling inventory resources: %w", err)
	}

	return refs, nil
}

func DeleteInventory(ctx context.Context, client *Client, namespace, releaseName string) error {
	if client == nil || client.CoreV1 == nil {
		return errors.New("k8s client is required")
	}
	if releaseName == "" {
		return errors.New("release name is required")
	}

	targetNamespace, err := resolveInventoryNamespace(client, namespace)
	if err != nil {
		return err
	}

	err = client.CoreV1.ConfigMaps(targetNamespace).Delete(ctx, inventoryConfigMapName(releaseName), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting inventory configmap: %w", err)
	}

	return nil
}

func inventoryConfigMapName(releaseName string) string {
	return inventoryConfigMapPrefix + releaseName
}

func resolveInventoryNamespace(client *Client, namespace string) (string, error) {
	if namespace != "" {
		return namespace, nil
	}
	if client.Namespace != "" {
		return client.Namespace, nil
	}
	return "", errors.New("namespace is required")
}

func ResourcesToRefs(resources []Resource) ([]ResourceRef, error) {
	refs := make([]ResourceRef, 0, len(resources))
	for i, resource := range resources {
		apiVersion, ok := resource["apiVersion"].(string)
		if !ok || apiVersion == "" {
			return nil, fmt.Errorf("resource %d has invalid apiVersion", i)
		}
		kind, ok := resource["kind"].(string)
		if !ok || kind == "" {
			return nil, fmt.Errorf("resource %d has invalid kind", i)
		}

		metadata, _ := resource["metadata"].(map[string]interface{})
		name, _ := metadata["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("resource %d has invalid metadata.name", i)
		}

		namespace, _ := metadata["namespace"].(string)
		refs = append(refs, ResourceRef{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			Namespace:  namespace,
		})
	}

	return refs, nil
}
