package k8s

import (
	"context"
	"fmt"
	"log"

	"github.com/fatih/color"
)

// ApplyRelease performs a full release cycle: loads existing inventory,
// applies resources, prunes orphaned resources, and saves updated inventory.
func (c *client) applyRelease(ctx context.Context, namespace, releaseName string, resources []Resource) error {
	oldRefs, err := loadInventory(ctx, c, namespace, releaseName)
	if err != nil {
		return fmt.Errorf("loading inventory: %w", err)
	}

	newRefs, err := resourcesToRefs(resources)
	if err != nil {
		return fmt.Errorf("building resource refs: %w", err)
	}

	orphaned := computeOrphaned(oldRefs, newRefs)

	if err := c.apply(ctx, resources); err != nil {
		return fmt.Errorf("applying resources: %w", err)
	}

	if len(orphaned) > 0 {
		log.Printf("%s %d orphaned resource(s)", color.HiRedString("pruning"), len(orphaned))
		if err := c.del(ctx, orphaned); err != nil {
			return fmt.Errorf("pruning orphaned resources: %w", err)
		}
	}

	if err := saveInventory(ctx, c, namespace, releaseName, resources); err != nil {
		return fmt.Errorf("saving inventory: %w", err)
	}

	return nil
}
