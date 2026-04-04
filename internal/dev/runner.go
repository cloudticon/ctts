package dev

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudticon/ctts/pkg/engine"
	"github.com/cloudticon/ctts/pkg/k8s"
)

type RunOpts struct {
	Dir     string
	EnvFile string
	KubeCtx string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type kubeApplier interface {
	Apply(ctx context.Context, resources []engine.Resource) error
}

var newK8sClient = func(kubeCtx, namespace string) (kubeApplier, error) {
	return k8s.NewClient(kubeCtx, namespace)
}

var startDevFeatures = func(ctx context.Context, client kubeApplier, targets []Target, stdout io.Writer) error {
	_ = ctx
	_ = client
	_ = targets
	_ = stdout
	// Port-forwarding, sync, logs and terminal are added in later phases.
	return nil
}

func Run(ctx context.Context, opts RunOpts) error {
	normalizedOpts, err := normalizeRunOpts(opts)
	if err != nil {
		return err
	}

	envVars, err := loadEnvVars(normalizedOpts.Dir, normalizedOpts.EnvFile)
	if err != nil {
		return err
	}

	devCode, err := bundleEntry(normalizedOpts.Dir, "dev.ct")
	if err != nil {
		return fmt.Errorf("bundling dev.ct: %w", err)
	}

	promptCache, err := engine.NewPromptCache(normalizedOpts.Dir)
	if err != nil {
		return fmt.Errorf("creating prompt cache: %w", err)
	}

	devResult, err := engine.ExecuteDev(engine.ExecuteDevOpts{
		JSCode:   devCode,
		EnvVars:  envVars,
		PromptFn: engine.MakePromptFn(promptCache, normalizedOpts.Stdin, normalizedOpts.Stdout),
	})
	if err != nil {
		return fmt.Errorf("executing dev.ct: %w", err)
	}

	targets, err := convertTargets(devResult.Targets)
	if err != nil {
		return err
	}

	resources, err := renderMainResources(normalizedOpts.Dir, devResult.Namespace, devResult.Values)
	if err != nil {
		return err
	}

	if err := ResolveSelectors(targets, resources); err != nil {
		return err
	}
	PatchResources(resources, targets)

	client, err := newK8sClient(normalizedOpts.KubeCtx, devResult.Namespace)
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	if err := client.Apply(ctx, resources); err != nil {
		return fmt.Errorf("applying resources: %w", err)
	}

	if err := startDevFeatures(ctx, client, targets, normalizedOpts.Stdout); err != nil {
		return fmt.Errorf("starting dev features: %w", err)
	}

	return nil
}

func normalizeRunOpts(opts RunOpts) (RunOpts, error) {
	result := opts
	if result.Dir == "" {
		result.Dir = "."
	}

	absDir, err := filepath.Abs(result.Dir)
	if err != nil {
		return RunOpts{}, fmt.Errorf("resolving directory: %w", err)
	}
	result.Dir = absDir

	if result.Stdin == nil {
		result.Stdin = os.Stdin
	}
	if result.Stdout == nil {
		result.Stdout = os.Stdout
	}
	if result.Stderr == nil {
		result.Stderr = os.Stderr
	}

	return result, nil
}

func loadEnvVars(dir, envFile string) (map[string]string, error) {
	if envFile == "" {
		return engine.MergeEnvWithSystem(nil), nil
	}

	envPath := envFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(dir, envPath)
	}

	fileEnv, err := engine.LoadEnvFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return engine.MergeEnvWithSystem(nil), nil
		}
		return nil, fmt.Errorf("loading env file %s: %w", envPath, err)
	}

	return engine.MergeEnvWithSystem(fileEnv), nil
}

func bundleEntry(dir, fileName string) (string, error) {
	entryPath := filepath.Join(dir, fileName)
	if _, err := os.Stat(entryPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("entry point not found: %s", entryPath)
		}
		return "", fmt.Errorf("checking %s: %w", fileName, err)
	}

	tr := engine.NewTranspiler(dir)
	jsCode, err := tr.Bundle(entryPath)
	if err != nil {
		return "", fmt.Errorf("bundle failed: %w", err)
	}
	return jsCode, nil
}

func renderMainResources(dir, namespace string, overlayValues map[string]interface{}) ([]engine.Resource, error) {
	mainCode, err := bundleEntry(dir, "main.ct")
	if err != nil {
		return nil, fmt.Errorf("bundling main.ct: %w", err)
	}

	baseValues, err := loadBaseValues(dir)
	if err != nil {
		return nil, err
	}

	mergedValues := DeepMergeValues(baseValues, overlayValues)
	resources, err := engine.Execute(engine.ExecuteOpts{
		JSCode:    mainCode,
		Values:    mergedValues,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("executing main.ct: %w", err)
	}

	return resources, nil
}

func loadBaseValues(dir string) (map[string]interface{}, error) {
	valuesPath := resolveValuesPath(dir)
	if valuesPath == "" {
		return nil, nil
	}

	values, err := engine.LoadValuesFile(valuesPath, nil)
	if err != nil {
		return nil, fmt.Errorf("loading values from %s: %w", valuesPath, err)
	}
	return values, nil
}

func resolveValuesPath(dir string) string {
	for _, name := range []string{"values.json", "values.yaml", "values.yml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func convertTargets(rawTargets []engine.RawDevTarget) ([]Target, error) {
	targets := make([]Target, 0, len(rawTargets))
	for _, raw := range rawTargets {
		syncRules, err := parseSyncRules(raw.Name, raw.Sync)
		if err != nil {
			return nil, err
		}

		portRules, err := parsePortRules(raw.Name, raw.Ports)
		if err != nil {
			return nil, err
		}

		envVars, err := parseEnvVars(raw.Name, raw.Env)
		if err != nil {
			return nil, err
		}

		target := Target{
			Name:      raw.Name,
			Selector:  raw.Selector,
			Container: raw.Container,
			Sync:      syncRules,
			Ports:     portRules,
			Terminal:  raw.Terminal,
			Probes:    raw.Probes,
			Env:       envVars,
			Command:   raw.Command,
		}

		if raw.Replicas != nil {
			replicas := int(*raw.Replicas)
			target.Replicas = &replicas
		}

		targets = append(targets, target)
	}
	return targets, nil
}

func parseSyncRules(targetName string, rawSync []map[string]interface{}) ([]SyncRule, error) {
	rules := make([]SyncRule, 0, len(rawSync))
	for i, rawRule := range rawSync {
		from, ok := rawRule["from"].(string)
		if !ok || strings.TrimSpace(from) == "" {
			return nil, fmt.Errorf("target %q: sync[%d].from must be a non-empty string", targetName, i)
		}
		to, ok := rawRule["to"].(string)
		if !ok || strings.TrimSpace(to) == "" {
			return nil, fmt.Errorf("target %q: sync[%d].to must be a non-empty string", targetName, i)
		}

		rule := SyncRule{
			From: from,
			To:   to,
		}

		if rawExclude, ok := rawRule["exclude"].([]interface{}); ok {
			rule.Exclude = make([]string, 0, len(rawExclude))
			for _, item := range rawExclude {
				rule.Exclude = append(rule.Exclude, fmt.Sprint(item))
			}
		}

		if polling, ok := rawRule["polling"].(bool); ok {
			rule.Polling = polling
		}

		rules = append(rules, rule)
	}
	return rules, nil
}

func parsePortRules(targetName string, rawPorts []interface{}) ([]PortRule, error) {
	rules := make([]PortRule, 0, len(rawPorts))
	for i, rawPort := range rawPorts {
		switch v := rawPort.(type) {
		case int:
			rules = append(rules, PortRule{Local: v, Remote: v})
		case int64:
			n := int(v)
			rules = append(rules, PortRule{Local: n, Remote: n})
		case float64:
			n, err := floatToInt(v)
			if err != nil {
				return nil, fmt.Errorf("target %q: ports[%d]: %w", targetName, i, err)
			}
			rules = append(rules, PortRule{Local: n, Remote: n})
		case []interface{}:
			if len(v) != 2 {
				return nil, fmt.Errorf("target %q: ports[%d] tuple must have 2 items", targetName, i)
			}
			local, err := numericToInt(v[0])
			if err != nil {
				return nil, fmt.Errorf("target %q: ports[%d][0]: %w", targetName, i, err)
			}
			remote, err := numericToInt(v[1])
			if err != nil {
				return nil, fmt.Errorf("target %q: ports[%d][1]: %w", targetName, i, err)
			}
			rules = append(rules, PortRule{Local: local, Remote: remote})
		default:
			return nil, fmt.Errorf("target %q: ports[%d] must be number or [local,remote]", targetName, i)
		}
	}
	return rules, nil
}

func parseEnvVars(targetName string, rawEnv []map[string]interface{}) ([]EnvVar, error) {
	result := make([]EnvVar, 0, len(rawEnv))
	for i, raw := range rawEnv {
		name, ok := raw["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("target %q: env[%d].name must be a non-empty string", targetName, i)
		}
		value := ""
		if rawValue, exists := raw["value"]; exists {
			value = fmt.Sprint(rawValue)
		}
		result = append(result, EnvVar{Name: name, Value: value})
	}
	return result, nil
}

func numericToInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return floatToInt(n)
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}

func floatToInt(v float64) (int, error) {
	n := int(v)
	if float64(n) != v {
		return 0, fmt.Errorf("value %v is not an integer", v)
	}
	return n, nil
}
