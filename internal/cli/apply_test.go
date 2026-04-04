package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyCmd_MissingMainCt(t *testing.T) {
	dir := t.TempDir()

	cmd := newApplyCmd()
	cmd.SetArgs([]string{dir})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry point not found")
}

func TestApplyCmd_RequiresArg(t *testing.T) {
	cmd := newApplyCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestApplyCmd_DefaultFlags(t *testing.T) {
	cmd := newApplyCmd()

	ns, _ := cmd.Flags().GetString("namespace")
	assert.Equal(t, "", ns)

	ctx, _ := cmd.Flags().GetString("context")
	assert.Equal(t, "", ctx)

	outputFmt, _ := cmd.Flags().GetString("output")
	assert.Equal(t, "", outputFmt)
}
