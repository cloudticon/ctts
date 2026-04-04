package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/cloudticon/ctts/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyCmd_MissingMainCt(t *testing.T) {
	dir := t.TempDir()

	cmd := newApplyCmd()
	cmd.SetArgs([]string{"my-release", dir})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}

func TestApplyCmd_RequiresExactlyTwoArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"zero args", []string{}},
		{"one arg", []string{"my-release"}},
		{"three args", []string{"my-release", ".", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newApplyCmd()
			cmd.SetArgs(tt.args)
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "accepts 2 arg(s)")
		})
	}
}

func TestApplyCmd_DefaultFlags(t *testing.T) {
	cmd := newApplyCmd()

	ns, _ := cmd.Flags().GetString("namespace")
	assert.Equal(t, "", ns)

	ctx, _ := cmd.Flags().GetString("context")
	assert.Equal(t, "", ctx)

	outputFmt, _ := cmd.Flags().GetString("output")
	assert.Equal(t, "", outputFmt)

	noCache, _ := cmd.Flags().GetBool("no-cache")
	assert.False(t, noCache)
}

func TestApplyCmd_UsageShowsTwoArgs(t *testing.T) {
	cmd := newApplyCmd()
	assert.Contains(t, cmd.Use, "<name>")
	assert.Contains(t, cmd.Use, "<dir|repo>")
}

func TestApplyCmd_MissingMainCt_WithReleaseName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, scaffold.Init(dir))

	cmd := newApplyCmd()
	cmd.SetArgs([]string{"prod-release", t.TempDir()})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}
