package dev

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudticon/ctts/pkg/k8s"
	"github.com/cloudticon/ctts/pkg/engine"
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
			Command: []string{"npm", "run", "dev"},
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

func TestStartDevFeatures_StartsAllFeaturesAndRunsTerminal(t *testing.T) {
	origRunPortForward := runPortForwardFn
	origRunLogs := runLogsFn
	origRunSync := runSyncFn
	origRunTerminal := runTerminalFn
	t.Cleanup(func() {
		runPortForwardFn = origRunPortForward
		runLogsFn = origRunLogs
		runSyncFn = origRunSync
		runTerminalFn = origRunTerminal
	})

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
	assert.ElementsMatch(t, []string{"api", "redis"}, logCalls)
	assert.Equal(t, []string{"redis:./->/app"}, syncCalls)
	assert.Equal(t, map[string]string{"app": "redis"}, terminalSelector)
	assert.Equal(t, "bash", terminalCommand)
	assert.Contains(t, stdout.String(), "starting terminal for target redis")
}

func TestStartDevFeatures_UnsupportedClientType(t *testing.T) {
	err := startDevFeatures(context.Background(), fakeApplier{}, []Target{{Name: "redis", Terminal: "bash"}}, &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kubernetes client type")
}

func TestStartDevFeatures_ReturnsTerminalError(t *testing.T) {
	origRunPortForward := runPortForwardFn
	origRunLogs := runLogsFn
	origRunSync := runSyncFn
	origRunTerminal := runTerminalFn
	t.Cleanup(func() {
		runPortForwardFn = origRunPortForward
		runLogsFn = origRunLogs
		runSyncFn = origRunSync
		runTerminalFn = origRunTerminal
	})

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

func TestStartDevFeatures_ReturnsBackgroundFeatureError(t *testing.T) {
	origRunPortForward := runPortForwardFn
	origRunLogs := runLogsFn
	origRunSync := runSyncFn
	origRunTerminal := runTerminalFn
	t.Cleanup(func() {
		runPortForwardFn = origRunPortForward
		runLogsFn = origRunLogs
		runSyncFn = origRunSync
		runTerminalFn = origRunTerminal
	})

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
