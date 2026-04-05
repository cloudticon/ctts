package dev

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudticon/ctts/pkg/engine"
	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertTargets_ParsesAllSupportedFields(t *testing.T) {
	replicas := int64(2)
	probes := true

	raw := []engine.RawDevTarget{
		{
			Name:      "remix",
			Selector:  map[string]string{"app": "remix"},
			Container: "web",
			Sync: []map[string]interface{}{
				{
					"from":    "./",
					"to":      "/app",
					"exclude": []interface{}{"/node_modules", "/.git"},
					"polling": true,
				},
			},
			Ports:    []interface{}{int64(3000), []interface{}{int64(15432), int64(5432)}},
			Terminal: "npm i && bash",
			Probes:   &probes,
			Replicas: &replicas,
			Env: []map[string]interface{}{
				{"name": "NODE_ENV", "value": "development"},
			},
			WorkingDir: "/workspace",
			Image:      "custom:dev",
			Command:    []string{"npm", "run", "dev"},
		},
	}

	targets, err := convertTargets(raw)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	target := targets[0]
	assert.Equal(t, "remix", target.Name)
	assert.Equal(t, map[string]string{"app": "remix"}, target.Selector)
	assert.Equal(t, "web", target.Container)
	assert.Equal(t, "npm i && bash", target.Terminal)
	require.NotNil(t, target.Probes)
	assert.True(t, *target.Probes)
	require.NotNil(t, target.Replicas)
	assert.Equal(t, 2, *target.Replicas)
	assert.Equal(t, "/workspace", target.WorkingDir)
	assert.Equal(t, "custom:dev", target.Image)
	assert.Equal(t, []string{"npm", "run", "dev"}, target.Command)

	require.Len(t, target.Ports, 2)
	assert.Equal(t, PortRule{Local: 3000, Remote: 3000}, target.Ports[0])
	assert.Equal(t, PortRule{Local: 15432, Remote: 5432}, target.Ports[1])

	require.Len(t, target.Sync, 1)
	assert.Equal(t, SyncRule{
		From:    "./",
		To:      "/app",
		Exclude: []string{"/node_modules", "/.git"},
		Polling: true,
	}, target.Sync[0])

	assert.Equal(t, []EnvVar{{Name: "NODE_ENV", Value: "development"}}, target.Env)
}

func TestConvertTargets_InvalidPortTypeReturnsError(t *testing.T) {
	raw := []engine.RawDevTarget{
		{
			Name:  "broken",
			Ports: []interface{}{"invalid"},
		},
	}

	_, err := convertTargets(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `target "broken": ports[0]`)
}

func TestLoadEnvVars_LoadsRelativeFileAndMerges(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.dev")
	require.NoError(t, os.WriteFile(envPath, []byte("RUNNER_ENV=value\n"), 0o644))

	env, err := loadEnvVars(dir, ".env.dev")
	require.NoError(t, err)
	assert.Equal(t, "value", env["RUNNER_ENV"])
}

func TestLoadEnvVars_MissingFileDoesNotFail(t *testing.T) {
	env, err := loadEnvVars(t.TempDir(), ".env.missing")
	require.NoError(t, err)
	assert.NotNil(t, env)
}

type fakeApplier struct{}

func (fakeApplier) Apply(_ context.Context, _ []engine.Resource) error {
	return nil
}

func saveDevFeatureSeams(t *testing.T) {
	t.Helper()
	origPortForward := runPortForwardFn
	origLogs := runLogsFn
	origSync := runSyncFn
	origTerminal := runTerminalFn
	origWaitForPod := runWaitForPodFn
	t.Cleanup(func() {
		runPortForwardFn = origPortForward
		runLogsFn = origLogs
		runSyncFn = origSync
		runTerminalFn = origTerminal
		runWaitForPodFn = origWaitForPod
	})
	runWaitForPodFn = func(_ context.Context, _ *k8s.Client, _ map[string]string) (string, error) {
		return "pod-stub", nil
	}
}

func TestStartDevFeatures_StartsAllFeaturesAndRunsTerminal(t *testing.T) {
	saveDevFeatureSeams(t)

	var mu sync.Mutex
	var portCalls []string
	var logCalls []string
	var syncCalls []string
	var terminalSelector map[string]string
	var terminalCommand string

	runPortForwardFn = func(ctx context.Context, _ *k8s.Client, selector map[string]string, _ []PortRule) error {
		mu.Lock()
		portCalls = append(portCalls, selector["app"])
		mu.Unlock()
		<-ctx.Done()
		return nil
	}
	runLogsFn = func(ctx context.Context, _ *k8s.Client, targetName string, _ map[string]string, _ io.Writer) error {
		mu.Lock()
		logCalls = append(logCalls, targetName)
		mu.Unlock()
		<-ctx.Done()
		return nil
	}
	runSyncFn = func(ctx context.Context, _ *k8s.Client, selector map[string]string, rule SyncRule) error {
		mu.Lock()
		syncCalls = append(syncCalls, selector["app"]+":"+rule.From+"->"+rule.To)
		mu.Unlock()
		<-ctx.Done()
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, selector map[string]string, command string) error {
		terminalSelector = selector
		terminalCommand = command
		return nil
	}

	stdout := &bytes.Buffer{}
	targets := []Target{
		{
			Name:     "api",
			Selector: map[string]string{"app": "api"},
			Ports:    []PortRule{{Local: 8080, Remote: 8080}},
		},
		{
			Name:     "redis",
			Selector: map[string]string{"app": "redis"},
			Ports:    []PortRule{{Local: 6379, Remote: 6379}},
			Sync: []SyncRule{
				{From: "./", To: "/app"},
			},
			Terminal: "bash",
		},
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, targets, stdout)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"api", "redis"}, portCalls)
	assert.Empty(t, logCalls, "logs should be suppressed when any target has a terminal")
	assert.Equal(t, []string{"redis:./->/app"}, syncCalls)
	assert.Equal(t, map[string]string{"app": "redis"}, terminalSelector)
	assert.Equal(t, "bash", terminalCommand)
	assert.Contains(t, stdout.String(), "starting terminal for target redis")
}

func TestStartDevFeatures_RunsTerminalInWorkingDir(t *testing.T) {
	saveDevFeatureSeams(t)

	runPortForwardFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		return nil
	}
	runLogsFn = func(_ context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		return nil
	}
	runSyncFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		return nil
	}

	var terminalCmd string
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, command string) error {
		terminalCmd = command
		return nil
	}

	targets := []Target{
		{
			Name:       "web",
			Selector:   map[string]string{"app": "web"},
			Terminal:   "npm run dev",
			WorkingDir: "/workspace/app dir",
		},
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, targets, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, `cd "/workspace/app dir" && npm run dev`, terminalCmd)
}

func TestStartDevFeatures_SilencesGlobalLoggerDuringTerminalAndRestores(t *testing.T) {
	saveDevFeatureSeams(t)

	originalLogOutput := log.Writer()
	logBuffer := &bytes.Buffer{}
	log.SetOutput(logBuffer)
	t.Cleanup(func() {
		log.SetOutput(originalLogOutput)
	})

	runPortForwardFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		return nil
	}
	runLogsFn = func(_ context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		return nil
	}
	runSyncFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		log.Printf("[during-terminal] should be silenced")
		return nil
	}

	targets := []Target{
		{
			Name:     "redis",
			Selector: map[string]string{"app": "redis"},
			Sync:     []SyncRule{{From: "./", To: "/app"}},
			Terminal: "bash",
		},
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, targets, &bytes.Buffer{})
	require.NoError(t, err)
	assert.NotContains(t, logBuffer.String(), "[during-terminal] should be silenced",
		"global logger output should be silenced while terminal is active")
	assert.Same(t, logBuffer, log.Writer(),
		"global logger output should be restored after terminal exits")
}

func TestStartDevFeatures_UnsupportedClientType(t *testing.T) {
	err := startDevFeatures(context.Background(), fakeApplier{}, []Target{{Name: "redis", Terminal: "bash"}}, &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kubernetes client type")
}

func TestStartDevFeatures_ReturnsTerminalError(t *testing.T) {
	saveDevFeatureSeams(t)

	runPortForwardFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		return nil
	}
	runLogsFn = func(ctx context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		<-ctx.Done()
		return nil
	}
	runSyncFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		return errors.New("exec failed")
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, []Target{{Name: "redis", Terminal: "bash"}}, &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec failed")
}

func TestStartDevFeatures_IgnoresTerminalExitCode130(t *testing.T) {
	saveDevFeatureSeams(t)

	runPortForwardFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		return nil
	}
	runLogsFn = func(ctx context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		<-ctx.Done()
		return nil
	}
	runSyncFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		return errors.New("executing command in pod web-123: command terminated with exit code 130")
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, []Target{{Name: "redis", Terminal: "bash"}}, &bytes.Buffer{})
	require.NoError(t, err)
}

func TestStartDevFeatures_ReturnsBackgroundFeatureError(t *testing.T) {
	saveDevFeatureSeams(t)

	runPortForwardFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		return nil
	}
	runSyncFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		return nil
	}
	runLogsFn = func(_ context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		return errors.New("logs failed")
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		return nil
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, []Target{{Name: "redis"}}, &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logs failed")
}

func TestStartDevFeatures_SuppressesLogsWhenTerminalActive(t *testing.T) {
	saveDevFeatureSeams(t)

	logsCalled := false
	runPortForwardFn = func(ctx context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		<-ctx.Done()
		return nil
	}
	runLogsFn = func(ctx context.Context, _ *k8s.Client, _ string, _ map[string]string, _ io.Writer) error {
		logsCalled = true
		<-ctx.Done()
		return nil
	}
	runSyncFn = func(ctx context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		<-ctx.Done()
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		return nil
	}

	targets := []Target{
		{Name: "api", Selector: map[string]string{"app": "api"}},
		{Name: "web", Selector: map[string]string{"app": "web"}, Terminal: "bash"},
	}

	err := startDevFeatures(context.Background(), &k8s.Client{}, targets, &bytes.Buffer{})
	require.NoError(t, err)
	assert.False(t, logsCalled, "logs should not start when any target has a terminal")
}

func TestStartDevFeatures_StartsLogsWhenNoTerminal(t *testing.T) {
	saveDevFeatureSeams(t)

	var mu sync.Mutex
	var logCalls []string
	runPortForwardFn = func(ctx context.Context, _ *k8s.Client, _ map[string]string, _ []PortRule) error {
		<-ctx.Done()
		return nil
	}
	runLogsFn = func(ctx context.Context, _ *k8s.Client, name string, _ map[string]string, _ io.Writer) error {
		mu.Lock()
		logCalls = append(logCalls, name)
		mu.Unlock()
		<-ctx.Done()
		return nil
	}
	runSyncFn = func(ctx context.Context, _ *k8s.Client, _ map[string]string, _ SyncRule) error {
		<-ctx.Done()
		return nil
	}
	runTerminalFn = func(_ context.Context, _ *k8s.Client, _ map[string]string, _ string) error {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	targets := []Target{
		{Name: "api", Selector: map[string]string{"app": "api"}},
		{Name: "web", Selector: map[string]string{"app": "web"}},
	}

	err := startDevFeatures(ctx, &k8s.Client{}, targets, &bytes.Buffer{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"api", "web"}, logCalls, "logs should start for all targets when no terminal")
}

func TestNormalizeRunOpts_DefaultsReleaseNameToDev(t *testing.T) {
	result, err := normalizeRunOpts(RunOpts{Dir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, "dev", result.ReleaseName)
}

func TestNormalizeRunOpts_PreservesExplicitReleaseName(t *testing.T) {
	result, err := normalizeRunOpts(RunOpts{Dir: t.TempDir(), ReleaseName: "my-dev"})
	require.NoError(t, err)
	assert.Equal(t, "my-dev", result.ReleaseName)
}

func TestEnsureRunNamespace_SkipsWhenDisabledOrNamespaceEmpty(t *testing.T) {
	origEnsureNamespace := ensureNamespaceFn
	t.Cleanup(func() {
		ensureNamespaceFn = origEnsureNamespace
	})

	var calls int
	ensureNamespaceFn = func(ctx context.Context, client kubeApplier, namespace string) error {
		calls++
		return nil
	}

	err := ensureRunNamespace(context.Background(), &k8s.Client{}, "dev", false)
	require.NoError(t, err)

	err = ensureRunNamespace(context.Background(), &k8s.Client{}, "", true)
	require.NoError(t, err)

	assert.Equal(t, 0, calls)
}

func TestEnsureRunNamespace_CallsEnsureWhenEnabled(t *testing.T) {
	origEnsureNamespace := ensureNamespaceFn
	t.Cleanup(func() {
		ensureNamespaceFn = origEnsureNamespace
	})

	expectedClient := &k8s.Client{}
	var capturedNamespace string
	ensureNamespaceFn = func(ctx context.Context, client kubeApplier, namespace string) error {
		assert.Same(t, expectedClient, client)
		capturedNamespace = namespace
		return nil
	}

	err := ensureRunNamespace(context.Background(), expectedClient, "dev", true)
	require.NoError(t, err)
	assert.Equal(t, "dev", capturedNamespace)
}

func TestEnsureRunNamespace_ReturnsWrappedError(t *testing.T) {
	origEnsureNamespace := ensureNamespaceFn
	t.Cleanup(func() {
		ensureNamespaceFn = origEnsureNamespace
	})

	ensureNamespaceFn = func(ctx context.Context, client kubeApplier, namespace string) error {
		return errors.New("boom")
	}

	err := ensureRunNamespace(context.Background(), &k8s.Client{}, "dev", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `ensuring namespace "dev"`)
}

func saveDevDeleteSeams(t *testing.T) {
	t.Helper()
	origNewK8s := newK8sClient
	origLoad := loadInventoryFn
	origDeleteRes := deleteResourcesFn
	origDeleteInv := deleteInventoryFn
	t.Cleanup(func() {
		newK8sClient = origNewK8s
		loadInventoryFn = origLoad
		deleteResourcesFn = origDeleteRes
		deleteInventoryFn = origDeleteInv
	})
}

func TestRunDevDelete_DeletesResourcesAndInventory(t *testing.T) {
	saveDevDeleteSeams(t)

	expectedClient := &k8s.Client{}
	expectedResources := []k8s.ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "test-ns", Name: "web"},
		{APIVersion: "v1", Kind: "Service", Namespace: "test-ns", Name: "web-svc"},
	}
	var flow []string

	newK8sClient = func(kubeCtx, namespace string) (kubeApplier, error) {
		assert.Equal(t, "staging", kubeCtx)
		assert.Equal(t, "test-ns", namespace)
		flow = append(flow, "new-client")
		return expectedClient, nil
	}
	loadInventoryFn = func(_ context.Context, client kubeApplier, namespace, releaseName string) ([]k8s.ResourceRef, error) {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, "test-ns", namespace)
		assert.Equal(t, "dev", releaseName)
		flow = append(flow, "load-inventory")
		return expectedResources, nil
	}
	deleteResourcesFn = func(_ context.Context, client kubeApplier, resources []k8s.ResourceRef) error {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, expectedResources, resources)
		flow = append(flow, "delete-resources")
		return nil
	}
	deleteInventoryFn = func(_ context.Context, client kubeApplier, namespace, releaseName string) error {
		assert.Same(t, expectedClient, client)
		assert.Equal(t, "test-ns", namespace)
		assert.Equal(t, "dev", releaseName)
		flow = append(flow, "delete-inventory")
		return nil
	}

	stdout := &bytes.Buffer{}
	err := runDevDelete(context.Background(), RunOpts{
		KubeCtx:     "staging",
		ReleaseName: "dev",
		Stdout:      stdout,
	}, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, []string{"new-client", "load-inventory", "delete-resources", "delete-inventory"}, flow)
	assert.Contains(t, stdout.String(), "deleted dev environment dev (2 resources)")
}

func TestRunDevDelete_ReturnsErrorWhenClientCreationFails(t *testing.T) {
	saveDevDeleteSeams(t)

	newK8sClient = func(_, _ string) (kubeApplier, error) {
		return nil, errors.New("client boom")
	}

	err := runDevDelete(context.Background(), RunOpts{
		ReleaseName: "dev",
		Stdout:      &bytes.Buffer{},
	}, "ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating k8s client")
}

func TestRunDevDelete_ReturnsErrorWhenLoadInventoryFails(t *testing.T) {
	saveDevDeleteSeams(t)

	newK8sClient = func(_, _ string) (kubeApplier, error) {
		return &k8s.Client{}, nil
	}
	loadInventoryFn = func(_ context.Context, _ kubeApplier, _, _ string) ([]k8s.ResourceRef, error) {
		return nil, errors.New("inventory boom")
	}

	err := runDevDelete(context.Background(), RunOpts{
		ReleaseName: "dev",
		Stdout:      &bytes.Buffer{},
	}, "ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading inventory for release")
}

func TestRunDevDelete_ReturnsErrorWhenDeleteResourcesFails(t *testing.T) {
	saveDevDeleteSeams(t)

	newK8sClient = func(_, _ string) (kubeApplier, error) {
		return &k8s.Client{}, nil
	}
	loadInventoryFn = func(_ context.Context, _ kubeApplier, _, _ string) ([]k8s.ResourceRef, error) {
		return []k8s.ResourceRef{{APIVersion: "v1", Kind: "Service", Namespace: "ns", Name: "svc"}}, nil
	}
	deleteResourcesFn = func(_ context.Context, _ kubeApplier, _ []k8s.ResourceRef) error {
		return errors.New("delete boom")
	}

	err := runDevDelete(context.Background(), RunOpts{
		ReleaseName: "dev",
		Stdout:      &bytes.Buffer{},
	}, "ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting dev resources")
}

func TestRunDevDelete_ReturnsErrorWhenDeleteInventoryFails(t *testing.T) {
	saveDevDeleteSeams(t)

	newK8sClient = func(_, _ string) (kubeApplier, error) {
		return &k8s.Client{}, nil
	}
	loadInventoryFn = func(_ context.Context, _ kubeApplier, _, _ string) ([]k8s.ResourceRef, error) {
		return []k8s.ResourceRef{}, nil
	}
	deleteResourcesFn = func(_ context.Context, _ kubeApplier, _ []k8s.ResourceRef) error {
		return nil
	}
	deleteInventoryFn = func(_ context.Context, _ kubeApplier, _, _ string) error {
		return errors.New("cleanup boom")
	}

	err := runDevDelete(context.Background(), RunOpts{
		ReleaseName: "dev",
		Stdout:      &bytes.Buffer{},
	}, "ns")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting dev inventory")
}

func TestRunDevDelete_UsesCustomReleaseName(t *testing.T) {
	saveDevDeleteSeams(t)

	var capturedReleaseName string
	newK8sClient = func(_, _ string) (kubeApplier, error) {
		return &k8s.Client{}, nil
	}
	loadInventoryFn = func(_ context.Context, _ kubeApplier, _, releaseName string) ([]k8s.ResourceRef, error) {
		capturedReleaseName = releaseName
		return []k8s.ResourceRef{}, nil
	}
	deleteResourcesFn = func(_ context.Context, _ kubeApplier, _ []k8s.ResourceRef) error {
		return nil
	}
	deleteInventoryFn = func(_ context.Context, _ kubeApplier, _, _ string) error {
		return nil
	}

	stdout := &bytes.Buffer{}
	err := runDevDelete(context.Background(), RunOpts{
		ReleaseName: "dev-alice",
		Stdout:      stdout,
	}, "ns")
	require.NoError(t, err)
	assert.Equal(t, "dev-alice", capturedReleaseName)
	assert.Contains(t, stdout.String(), "deleted dev environment dev-alice")
}
