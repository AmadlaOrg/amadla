package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_CreatesConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	binDir := t.TempDir()
	createMockTool(t, binDir, "hery", nil)
	createMockTool(t, binDir, "weaver", nil)
	t.Setenv("PATH", binDir)

	stdout, _, err := executeTestCmd("init")
	require.NoError(t, err)

	configPath := filepath.Join(tmpHome, ".config", "amadla", "tools.hery")
	assert.FileExists(t, configPath)
	assert.Contains(t, stdout, "Created")
	assert.Contains(t, stdout, "2 tools")
	assert.Contains(t, stdout, "hery")
	assert.Contains(t, stdout, "weaver")

	// Verify file content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: hery")
	assert.Contains(t, string(data), "name: weaver")
}

func TestInit_FailsWhenConfigExists(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config", "amadla")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "tools.hery"), []byte("existing"), 0644))

	_, stderr, err := executeTestCmd("init")
	assert.Error(t, err)
	assert.Contains(t, stderr, "config already exists")
}

func TestInit_NoToolsFound(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Empty PATH — no tools will be found
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	// Override LookPath to ensure nothing is found
	orig := initExecLookPath
	defer func() { initExecLookPath = orig }()
	initExecLookPath = func(file string) (string, error) {
		return exec.LookPath(file)
	}

	_, stderr, err := executeTestCmd("init")
	require.NoError(t, err) // not an error, just a warning
	assert.Contains(t, stderr, "No Amadla tools found")

	// Config should NOT be created
	configPath := filepath.Join(tmpHome, ".config", "amadla", "tools.hery")
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))
}
