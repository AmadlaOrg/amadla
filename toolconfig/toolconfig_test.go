package toolconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".config", "amadla", "tools.hery"), path)
}

func TestDefaultConfigPath_HomeDirError(t *testing.T) {
	orig := osUserHomeDir
	defer func() { osUserHomeDir = orig }()
	osUserHomeDir = func() (string, error) {
		return "", fmt.Errorf("no home")
	}

	_, err := DefaultConfigPath()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine home directory")
}

func TestLoad_ValidConfig(t *testing.T) {
	content := `_type: amadla.org/entity/tools@v1.0.0
_body:
  tools:
    - name: hery
      required: true
      supports:
        - amadla.org/entity/application@v1.0.0
    - name: weaver
      description: Template generator
`
	tmpFile := filepath.Join(t.TempDir(), "tools.hery")
	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

	cfg, err := Load(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "amadla.org/entity/tools@v1.0.0", cfg.Type)
	assert.Len(t, cfg.Body.Tools, 2)
	assert.Equal(t, "hery", cfg.Body.Tools[0].Name)
	assert.True(t, cfg.Body.Tools[0].Required)
	assert.Equal(t, []string{"amadla.org/entity/application@v1.0.0"}, cfg.Body.Tools[0].Supports)
	assert.Equal(t, "weaver", cfg.Body.Tools[1].Name)
	assert.Equal(t, "Template generator", cfg.Body.Tools[1].Description)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/tools.hery")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.hery")
	require.NoError(t, os.WriteFile(tmpFile, []byte(":\n  :\n    - [invalid"), 0644))

	_, err := Load(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse config")
}

func TestResolvePath_ExplicitPath(t *testing.T) {
	// Create a temp file to act as the tool binary
	tmpFile := filepath.Join(t.TempDir(), "mytool")
	require.NoError(t, os.WriteFile(tmpFile, []byte("#!/bin/sh\n"), 0755))

	tool := Tool{Name: "mytool", Path: tmpFile}
	path, err := ResolvePath(tool)
	require.NoError(t, err)
	assert.Equal(t, tmpFile, path)
}

func TestResolvePath_ExplicitPathNotFound(t *testing.T) {
	tool := Tool{Name: "mytool", Path: "/nonexistent/mytool"}
	_, err := ResolvePath(tool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "explicit path")
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePath_LookPath(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()
	execLookPath = func(file string) (string, error) {
		if file == "hery" {
			return "/usr/local/bin/hery", nil
		}
		return "", fmt.Errorf("not found")
	}

	tool := Tool{Name: "hery"}
	path, err := ResolvePath(tool)
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/hery", path)
}

func TestResolvePath_NotInPATH(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()
	execLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	tool := Tool{Name: "nonexistent"}
	_, err := ResolvePath(tool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in PATH")
}

func TestStandardToolNames(t *testing.T) {
	names := StandardToolNames()
	assert.Len(t, names, 13)
	assert.Contains(t, names, "hery")
	assert.Contains(t, names, "doorman")
	assert.Contains(t, names, "weaver")
	assert.Contains(t, names, "lay")
	assert.Contains(t, names, "enjoin")
	assert.Contains(t, names, "waiter")
	assert.Contains(t, names, "raise")
	assert.Contains(t, names, "unravel")
	assert.Contains(t, names, "judge")
	assert.Contains(t, names, "conduct")
	assert.Contains(t, names, "lighthouse")
	assert.Contains(t, names, "garbage")
	assert.Contains(t, names, "dryrun")
}

func TestInfoCache_NewCreatesEmpty(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	cache := NewInfoCache(cachePath)
	assert.NotNil(t, cache)
	assert.Empty(t, cache.entries)
}

func TestInfoCache_GetCallsInfoAndCaches(t *testing.T) {
	// Mock execLookPath
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	tmpBin := filepath.Join(t.TempDir(), "mytool")
	require.NoError(t, os.WriteFile(tmpBin, []byte("#!/bin/sh\n"), 0755))

	execLookPath = func(file string) (string, error) {
		if file == "mytool" {
			return tmpBin, nil
		}
		return "", fmt.Errorf("not found")
	}

	// Mock execCommand to return tool info JSON
	origCommand := execCommand
	defer func() { execCommand = origCommand }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"name":"mytool","version":"1.0.0","supports":["amadla.org/entity/application@v1.0.0"]}`)
	}

	cachePath := filepath.Join(t.TempDir(), "cache.json")
	cache := NewInfoCache(cachePath)

	info, err := cache.Get("mytool")
	require.NoError(t, err)
	assert.Equal(t, "mytool", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, []string{"amadla.org/entity/application@v1.0.0"}, info.Supports)
	assert.Equal(t, tmpBin, info.BinaryPath)

	// Verify cache file was written
	_, err = os.Stat(cachePath)
	assert.NoError(t, err)

	// Second call should use cache (not call execCommand again)
	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("echo", `{"name":"mytool","version":"1.0.0"}`)
	}

	info2, err := cache.Get("mytool")
	require.NoError(t, err)
	assert.Equal(t, "mytool", info2.Name)
	assert.Equal(t, 0, callCount, "should not call tool info again — cached")
}

func TestInfoCache_InvalidatesOnMtimeChange(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	tmpBin := filepath.Join(t.TempDir(), "mytool")
	require.NoError(t, os.WriteFile(tmpBin, []byte("v1"), 0755))

	execLookPath = func(file string) (string, error) {
		return tmpBin, nil
	}

	origCommand := execCommand
	defer func() { execCommand = origCommand }()

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("echo", `{"name":"mytool","version":"2.0.0"}`)
	}

	cachePath := filepath.Join(t.TempDir(), "cache.json")
	cache := NewInfoCache(cachePath)

	// First call
	_, err := cache.Get("mytool")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Change binary mtime explicitly (rewrite + set mtime to future)
	require.NoError(t, os.WriteFile(tmpBin, []byte("v2"), 0755))
	futureTime := time.Now().Add(10 * time.Second)
	require.NoError(t, os.Chtimes(tmpBin, futureTime, futureTime))

	// Second call should re-fetch because mtime changed
	info, err := cache.Get("mytool")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "should call tool info again — mtime changed")
	assert.Equal(t, "2.0.0", info.Version)
}

func TestInfoCache_ToolNotInPATH(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()
	execLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	cache := NewInfoCache(filepath.Join(t.TempDir(), "cache.json"))
	_, err := cache.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in PATH")
}

func TestDefaultCachePath(t *testing.T) {
	path, err := DefaultCachePath()
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".cache", "amadla", "tool-info.json"), path)
}

func TestEnrichWithInfo_FillsMissingSupports(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	tmpBin := filepath.Join(t.TempDir(), "weaver")
	require.NoError(t, os.WriteFile(tmpBin, []byte("#!/bin/sh\n"), 0755))

	execLookPath = func(file string) (string, error) {
		if file == "weaver" {
			return tmpBin, nil
		}
		return "", fmt.Errorf("not found")
	}

	origCommand := execCommand
	defer func() { execCommand = origCommand }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"name":"weaver","version":"1.0.0","supports":["amadla.org/entity/template@v1.0.0"],"description":"Template generator"}`)
	}

	cfg := &Config{}
	cfg.Body.Tools = []Tool{
		{Name: "hery", Supports: []string{"amadla.org/entity/application@v1.0.0"}}, // already set
		{Name: "weaver"}, // missing supports — should be filled
	}

	cache := NewInfoCache(filepath.Join(t.TempDir(), "cache.json"))
	EnrichWithInfo(cfg, cache)

	// hery should be unchanged
	assert.Equal(t, []string{"amadla.org/entity/application@v1.0.0"}, cfg.Body.Tools[0].Supports)
	// weaver should be filled from info
	assert.Equal(t, []string{"amadla.org/entity/template@v1.0.0"}, cfg.Body.Tools[1].Supports)
	assert.Equal(t, "Template generator", cfg.Body.Tools[1].Description)
}

func TestEnrichWithInfo_SkipsUnavailableTools(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()
	execLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	cfg := &Config{}
	cfg.Body.Tools = []Tool{
		{Name: "nonexistent"}, // not on PATH
	}

	cache := NewInfoCache(filepath.Join(t.TempDir(), "cache.json"))
	EnrichWithInfo(cfg, cache)

	// Should remain empty
	assert.Empty(t, cfg.Body.Tools[0].Supports)
}

func TestEnrichWithInfo_PreservesExistingDescription(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	tmpBin := filepath.Join(t.TempDir(), "mytool")
	require.NoError(t, os.WriteFile(tmpBin, []byte("#!/bin/sh\n"), 0755))

	execLookPath = func(file string) (string, error) {
		return tmpBin, nil
	}

	origCommand := execCommand
	defer func() { execCommand = origCommand }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"name":"mytool","version":"1.0.0","supports":["type-a"],"description":"from info"}`)
	}

	cfg := &Config{}
	cfg.Body.Tools = []Tool{
		{Name: "mytool", Description: "manual description"},
	}

	cache := NewInfoCache(filepath.Join(t.TempDir(), "cache.json"))
	EnrichWithInfo(cfg, cache)

	// Description should not be overwritten
	assert.Equal(t, "manual description", cfg.Body.Tools[0].Description)
	// But entity types should be filled
	assert.Equal(t, []string{"type-a"}, cfg.Body.Tools[0].Supports)
}

func TestLoad_EmptyTools(t *testing.T) {
	content := `_type: amadla.org/entity/tools@v1.0.0
_body:
  tools: []
`
	tmpFile := filepath.Join(t.TempDir(), "tools.hery")
	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

	cfg, err := Load(tmpFile)
	require.NoError(t, err)
	assert.Empty(t, cfg.Body.Tools)
}
