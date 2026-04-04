package dev

// Config represents the fully parsed dev.ct configuration.
type Config struct {
	Namespace string
	Values    map[string]interface{}
	Targets   []Target
}

// Target describes a single dev target with its features and workload patches.
type Target struct {
	Name      string
	Selector  map[string]string // nil = auto-resolve from resources
	Container string            // which container to patch (empty = first)

	// Dev features
	Sync     []SyncRule
	Ports    []PortRule
	Terminal string // empty = no terminal

	// Workload patches (applied between render and apply)
	Probes     *bool    // nil = default false (remove probes); true = keep
	Replicas   *int     // nil = no change
	Env        []EnvVar // add/override env vars on container
	WorkingDir string   // override container workingDir
	Image      string   // override container image
	Command    []string // override container command
}

type EnvVar struct {
	Name  string
	Value string
}

type SyncRule struct {
	From    string
	To      string
	Exclude []string
	Polling bool
}

type PortRule struct {
	Local  int
	Remote int
}

// DeepMergeValues merges overlay on top of base (from values.json).
// Overlay keys override base; nested maps are merged recursively.
// Neither base nor overlay are mutated.
func DeepMergeValues(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(overlay))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if baseMap, ok := result[k].(map[string]interface{}); ok {
			if overlayMap, ok := v.(map[string]interface{}); ok {
				result[k] = DeepMergeValues(baseMap, overlayMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}
