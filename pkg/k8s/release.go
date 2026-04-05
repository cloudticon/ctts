package k8s

import (
	"context"
	"fmt"
	"log"

	"github.com/fatih/color"
)

// ApplyRelease performs a full release cycle: loads existing inventory,
// applies resources, prunes orphaned resources, and saves updated inventory.
func (c *Client) ApplyRelease(ctx context.Context, namespace, releaseName string, resources []Resource) error {
	oldRefs, err := LoadInventory(ctx, c, namespace, releaseName)
	if err != nil {
		return fmt.Errorf("loading inventory: %w", err)
	}

	newRefs, err := ResourcesToRefs(resources)
	if err != nil {
		return fmt.Errorf("building resource refs: %w", err)
	}

	orphaned := ComputeOrphaned(oldRefs, newRefs)

	if err := c.Apply(ctx, resources); err != nil {
		return fmt.Errorf("applying resources: %w", err)
	}

	if len(orphaned) > 0 {
		log.Printf("%s %d orphaned resource(s)", color.HiRedString("pruning"), len(orphaned))
		if err := c.Delete(ctx, orphaned); err != nil {
			return fmt.Errorf("pruning orphaned resources: %w", err)
		}
	}

	if err := SaveInventory(ctx, c, namespace, releaseName, resources); err != nil {
		return fmt.Errorf("saving inventory: %w", err)
	}

	return nil
}
