package k8s

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnsureNamespace(ctx context.Context, client *Client, namespace string) error {
	if client == nil || client.CoreV1 == nil {
		return errors.New("k8s client is required")
	}
	if namespace == "" {
		return errors.New("namespace is required")
	}

	_, err := client.CoreV1.Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting namespace %q: %w", namespace, err)
	}

	_, err = client.CoreV1.Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				inventoryManagedByLabelKey: "ct",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating namespace %q: %w", namespace, err)
	}

	return nil
}
