package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type Resource = map[string]interface{}

func (c *Client) Apply(ctx context.Context, resources []Resource) error {
	for _, res := range resources {
		if err := c.applyOne(ctx, res); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyOne(ctx context.Context, res Resource) error {
	obj := toUnstructured(res)

	info, err := c.resolveResourceInfo(obj.GetAPIVersion(), obj.GetKind())
	if err != nil {
		return fmt.Errorf("resolving resource info for %s %s: %w", obj.GetAPIVersion(), obj.GetKind(), err)
	}

	var dynClient dynamic.ResourceInterface
	if info.Namespaced {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = c.Namespace
		}
		dynClient = c.Dynamic.Resource(info.GVR).Namespace(ns)
	} else {
		dynClient = c.Dynamic.Resource(info.GVR)
	}

	data, err := json.Marshal(obj.Object)
	if err != nil {
		return fmt.Errorf("marshaling %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	force := true
	_, err = dynClient.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "ct",
		Force:        &force,
	})
	if err != nil {
		return fmt.Errorf("applying %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	log.Printf("applied %s/%s", obj.GetKind(), obj.GetName())
	return nil
}

func toUnstructured(res Resource) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: res}
}

func (c *Client) resolveResourceInfo(apiVersion, kind string) (*resourceInfo, error) {
	key := apiVersion + "/" + kind
	if info, ok := c.gvrCache[key]; ok {
		return info, nil
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}

	resourceList, err := c.Discovery.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("discovering resources for %s: %w", apiVersion, err)
	}

	for _, r := range resourceList.APIResources {
		if r.Kind == kind {
			info := &resourceInfo{
				GVR: schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: r.Name,
				},
				Namespaced: r.Namespaced,
			}
			c.gvrCache[key] = info
			return info, nil
		}
	}

	return nil, fmt.Errorf("kind %q not found in apiVersion %q", kind, apiVersion)
}
