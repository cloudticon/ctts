package dev

import (
	"os"
	"path/filepath"
	"testing"

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
