package cmd

import (
	"os/exec"
	"testing"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctor_AllToolsOK(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	binDir := t.TempDir()
	toolPath := createMockTool(t, binDir, "hery", nil)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "hery", Path: toolPath, Required: true},
	})

	// Mock exec to return version
	orig := doctorExecCommand
	defer func() { doctorExecCommand = orig }()
	doctorExecCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(toolPath, args...)
	}

	stdout, _, err := executeTestCmd("doctor", "--config", configPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "OK: hery")
	assert.Contains(t, stdout, "All required tools are available.")
}

func TestDoctor_RequiredToolMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "nonexistent", Required: true},
	})

	_, stderr, err := executeTestCmd("doctor", "--config", configPath)
	assert.Error(t, err)
	assert.Contains(t, stderr, "FAIL: nonexistent")
	assert.Contains(t, err.Error(), "1 required tool(s) missing")
}

func TestDoctor_OptionalToolMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "nonexistent", Required: false},
	})

	stdout, _, err := executeTestCmd("doctor", "--config", configPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "SKIP: nonexistent")
	assert.Contains(t, stdout, "All required tools are available.")
}

func TestDoctor_MixedToolStatus(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	binDir := t.TempDir()
	toolPath := createMockTool(t, binDir, "hery", nil)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "hery", Path: toolPath, Required: true},
		{Name: "optional-missing", Required: false},
	})

	orig := doctorExecCommand
	defer func() { doctorExecCommand = orig }()
	doctorExecCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(toolPath, args...)
	}

	stdout, _, err := executeTestCmd("doctor", "--config", configPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "OK: hery")
	assert.Contains(t, stdout, "SKIP: optional-missing")
	assert.Contains(t, stdout, "All required tools are available.")
}
