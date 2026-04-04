package k8s

import "fmt"

// ComputeOrphaned returns resources that existed before but are no longer present.
// Comparison key is apiVersion+kind+namespace+name.
func ComputeOrphaned(oldRefs, newRefs []ResourceRef) []ResourceRef {
	if len(oldRefs) == 0 {
		return []ResourceRef{}
	}

	newSet := make(map[string]struct{}, len(newRefs))
	for _, ref := range newRefs {
		newSet[resourceRefKey(ref)] = struct{}{}
	}

	orphaned := make([]ResourceRef, 0)
	seen := make(map[string]struct{}, len(oldRefs))
	for _, ref := range oldRefs {
		key := resourceRefKey(ref)
		if _, alreadyAdded := seen[key]; alreadyAdded {
			continue
		}
		seen[key] = struct{}{}

		if _, exists := newSet[key]; !exists {
			orphaned = append(orphaned, ref)
		}
	}

	return orphaned
}

func resourceRefKey(ref ResourceRef) string {
	return fmt.Sprintf("%s|%s|%s|%s", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
}
