package cmd

import (
	"testing"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_ShowsToolStatus(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	binDir := t.TempDir()
	toolPath := createMockTool(t, binDir, "hery", []string{"amadla.org/entity/application@v1.0.0"})

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "hery", Path: toolPath, Supports: []string{"amadla.org/entity/application@v1.0.0"}},
		{Name: "nonexistent", Supports: []string{"amadla.org/entity/foo@v1.0.0"}},
	})

	stdout, _, err := executeTestCmd("list", "--config", configPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "TOOL")
	assert.Contains(t, stdout, "STATUS")
	assert.Contains(t, stdout, "hery")
	assert.Contains(t, stdout, "found")
	assert.Contains(t, stdout, "nonexistent")
	assert.Contains(t, stdout, "missing")
}

func TestList_RequiredToolMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "nonexistent", Required: true},
	})

	stdout, stderr, err := executeTestCmd("list", "--config", configPath)
	require.NoError(t, err) // list doesn't return error for missing tools

	assert.Contains(t, stdout, "MISSING!")
	assert.Contains(t, stderr, "error: required tool nonexistent not found")
}

func TestList_EmptyTools(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{})

	stdout, _, err := executeTestCmd("list", "--config", configPath)
	require.NoError(t, err)

	// Should still print header
	assert.Contains(t, stdout, "TOOL")
	assert.Contains(t, stdout, "STATUS")
}
