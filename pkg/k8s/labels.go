package k8s

const (
	managedByLabelKey = "app.kubernetes.io/managed-by"
	managedByLabelVal = "ct"
	instanceLabelKey  = "ct.cloudticon.com/instance"
)

// InjectReleaseLabels adds ct release labels to each resource metadata.labels
// without overwriting any existing label values.
func InjectReleaseLabels(resources []Resource, releaseName string) []Resource {
	if len(resources) == 0 {
		return []Resource{}
	}

	labeled := make([]Resource, 0, len(resources))
	for _, resource := range resources {
		resourceCopy := cloneMap(resource)

		metadata := map[string]interface{}{}
		if existingMetadata, ok := resourceCopy["metadata"].(map[string]interface{}); ok {
			metadata = cloneMap(existingMetadata)
		}

		labels := map[string]interface{}{}
		if existingLabels, ok := metadata["labels"].(map[string]interface{}); ok {
			labels = cloneMap(existingLabels)
		}

		if _, exists := labels[managedByLabelKey]; !exists {
			labels[managedByLabelKey] = managedByLabelVal
		}
		if _, exists := labels[instanceLabelKey]; !exists {
			labels[instanceLabelKey] = releaseName
		}

		metadata["labels"] = labels
		resourceCopy["metadata"] = metadata
		labeled = append(labeled, resourceCopy)
	}

	return labeled
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}

	cloned := make(map[string]interface{}, len(src))
	for k, v := range src {
		cloned[k] = v
	}

	return cloned
}
