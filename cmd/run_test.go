package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildToolMap(t *testing.T) {
	cfg := &toolconfig.Config{}
	cfg.Body.Tools = []toolconfig.Tool{
		{
			Name:        "hery",
			Supports: []string{"amadla.org/entity/application@v1.0.0", "amadla.org/entity/system@v1.0.0"},
		},
		{
			Name:        "weaver",
			Supports: []string{"amadla.org/entity/template@v1.0.0"},
		},
		{
			Name: "doorman",
			// No entity types
		},
	}

	m := buildToolMap(cfg)
	assert.Len(t, m, 3)
	assert.Equal(t, "hery", m["amadla.org/entity/application@v1.0.0"].Name)
	assert.Equal(t, "hery", m["amadla.org/entity/system@v1.0.0"].Name)
	assert.Equal(t, "weaver", m["amadla.org/entity/template@v1.0.0"].Name)
}

func TestBuildToolMap_Empty(t *testing.T) {
	cfg := &toolconfig.Config{}
	m := buildToolMap(cfg)
	assert.Empty(t, m)
}

func TestBuildDAG_ValidEntities(t *testing.T) {
	dir := t.TempDir()

	// Entity with no requires
	writeHery(t, dir, "secrets.hery", `
_type: amadla.org/entity/secret@v1.0.0
_body:
  provider: vault
`)

	// Entity that requires secrets
	writeHery(t, dir, "postgres.hery", `
_type: amadla.org/entity/application@v1.0.0
_requires:
  - amadla.org/entity/secret@v1.0.0
_body:
  name: postgres
`)

	graph, entities, err := buildDAG(dir)
	require.NoError(t, err)

	assert.Len(t, entities, 2)
	assert.Contains(t, entities, "amadla.org/entity/secret@v1.0.0")
	assert.Contains(t, entities, "amadla.org/entity/application@v1.0.0")

	order, err := graph.Sort()
	require.NoError(t, err)
	assert.Len(t, order, 2)

	// Secrets must come before application
	idxSecrets := indexOf(order, "amadla.org/entity/secret@v1.0.0")
	idxApp := indexOf(order, "amadla.org/entity/application@v1.0.0")
	assert.True(t, idxSecrets < idxApp, "secrets must come before application")
}

func TestBuildDAG_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	graph, entities, err := buildDAG(dir)
	require.NoError(t, err)
	assert.Empty(t, entities)

	order, err := graph.Sort()
	require.NoError(t, err)
	assert.Empty(t, order)
}

func TestBuildDAG_SkipsNonHeryFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0755))

	writeHery(t, dir, "app.hery", `
_type: amadla.org/entity/application@v1.0.0
_body:
  name: myapp
`)

	_, entities, err := buildDAG(dir)
	require.NoError(t, err)
	assert.Len(t, entities, 1)
}

func TestBuildDAG_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.hery"), []byte(":\n  - [broken"), 0644))

	_, _, err := buildDAG(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse")
}

func TestBuildDAG_SkipsMissingType(t *testing.T) {
	dir := t.TempDir()

	// Entity without _type should be skipped
	writeHery(t, dir, "notype.hery", `
_body:
  name: something
`)

	writeHery(t, dir, "valid.hery", `
_type: amadla.org/entity/application@v1.0.0
_body:
  name: myapp
`)

	_, entities, err := buildDAG(dir)
	require.NoError(t, err)
	assert.Len(t, entities, 1)
	assert.Contains(t, entities, "amadla.org/entity/application@v1.0.0")
}

func TestBuildDAG_MultipleRequires(t *testing.T) {
	dir := t.TempDir()

	writeHery(t, dir, "secrets.hery", `
_type: amadla.org/entity/secret@v1.0.0
_body:
  provider: vault
`)

	writeHery(t, dir, "network.hery", `
_type: amadla.org/entity/system/net@v1.0.0
_body:
  cidr: 10.0.0.0/24
`)

	writeHery(t, dir, "app.hery", `
_type: amadla.org/entity/application@v1.0.0
_requires:
  - amadla.org/entity/secret@v1.0.0
  - amadla.org/entity/system/net@v1.0.0
_body:
  name: webapp
`)

	graph, entities, err := buildDAG(dir)
	require.NoError(t, err)
	assert.Len(t, entities, 3)

	order, err := graph.Sort()
	require.NoError(t, err)
	assert.Len(t, order, 3)

	idxApp := indexOf(order, "amadla.org/entity/application@v1.0.0")
	idxSecrets := indexOf(order, "amadla.org/entity/secret@v1.0.0")
	idxNet := indexOf(order, "amadla.org/entity/system/net@v1.0.0")

	assert.True(t, idxSecrets < idxApp, "secrets must come before app")
	assert.True(t, idxNet < idxApp, "network must come before app")
}

func TestBuildDAG_NonexistentDir(t *testing.T) {
	_, _, err := buildDAG("/nonexistent/dir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read directory")
}

func TestBuildDAG_CircularRequires(t *testing.T) {
	dir := t.TempDir()

	writeHery(t, dir, "a.hery", `
_type: type-a
_requires:
  - type-b
`)

	writeHery(t, dir, "b.hery", `
_type: type-b
_requires:
  - type-a
`)

	graph, _, err := buildDAG(dir)
	require.NoError(t, err)

	_, err = graph.Sort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular _requires reference")
}

// --- Integration tests for the run command ---

func TestRun_DryRun_LinearPipeline(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create config with tools
	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "doorman", Supports: []string{"amadla.org/entity/secret@v1.0.0"}},
		{Name: "lay", Supports: []string{"amadla.org/entity/application@v1.0.0"}},
		{Name: "weaver", Supports: []string{"amadla.org/entity/template@v1.0.0"}},
	})

	// Create entity dir with dependencies: secret → app → template
	entityDir := t.TempDir()
	writeTestHery(t, entityDir, "secret.hery", `
_type: amadla.org/entity/secret@v1.0.0
_body:
  provider: vault
`)
	writeTestHery(t, entityDir, "app.hery", `
_type: amadla.org/entity/application@v1.0.0
_requires:
  - amadla.org/entity/secret@v1.0.0
_body:
  name: myapp
`)
	writeTestHery(t, entityDir, "template.hery", `
_type: amadla.org/entity/template@v1.0.0
_requires:
  - amadla.org/entity/application@v1.0.0
_body:
  engine: go
`)

	stdout, _, err := executeTestCmd("run", "--dry-run", "--config", configPath, entityDir)
	require.NoError(t, err)

	assert.Contains(t, stdout, "Execution order:")
	// Verify ordering: secret before app before template
	idxSecret := len(stdout)
	idxApp := len(stdout)
	idxTemplate := len(stdout)
	for i, line := range splitLines(stdout) {
		if contains(line, "secret@v1.0.0") {
			idxSecret = i
		}
		if contains(line, "application@v1.0.0") {
			idxApp = i
		}
		if contains(line, "template@v1.0.0") {
			idxTemplate = i
		}
	}
	assert.True(t, idxSecret < idxApp, "secret must come before app")
	assert.True(t, idxApp < idxTemplate, "app must come before template")
}

func TestRun_DryRun_ParallelEntities(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "doorman", Supports: []string{"amadla.org/entity/secret@v1.0.0"}},
		{Name: "lay", Supports: []string{"amadla.org/entity/package@v1.0.0"}},
	})

	// Two independent entities — should be in one parallel tier
	entityDir := t.TempDir()
	writeTestHery(t, entityDir, "secret.hery", `
_type: amadla.org/entity/secret@v1.0.0
_body:
  provider: vault
`)
	writeTestHery(t, entityDir, "package.hery", `
_type: amadla.org/entity/package@v1.0.0
_body:
  name: nginx
`)

	stdout, _, err := executeTestCmd("run", "--dry-run", "--config", configPath, entityDir)
	require.NoError(t, err)

	assert.Contains(t, stdout, "[parallel]")
}

func TestRun_DryRun_EmptyDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{})

	entityDir := t.TempDir()

	stdout, _, err := executeTestCmd("run", "--dry-run", "--config", configPath, entityDir)
	require.NoError(t, err)

	// No entities = no execution order output beyond header
	assert.Contains(t, stdout, "Execution order:")
}

func TestRun_DryRun_CircularDeps(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "toolA", Supports: []string{"type-a"}},
		{Name: "toolB", Supports: []string{"type-b"}},
	})

	entityDir := t.TempDir()
	writeTestHery(t, entityDir, "a.hery", `
_type: type-a
_requires:
  - type-b
`)
	writeTestHery(t, entityDir, "b.hery", `
_type: type-b
_requires:
  - type-a
`)

	_, stderr, err := executeTestCmd("run", "--dry-run", "--config", configPath, entityDir)
	assert.Error(t, err)
	assert.Contains(t, stderr, "circular _requires reference")
}

func TestRun_ActualExecution(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a mock tool that writes a marker file
	binDir := t.TempDir()
	markerFile := filepath.Join(t.TempDir(), "executed.txt")

	script := fmt.Sprintf(`#!/bin/sh
echo "executed" >> %s
cat > /dev/null
`, markerFile)
	toolPath := filepath.Join(binDir, "mock-tool")
	require.NoError(t, os.WriteFile(toolPath, []byte(script), 0755))

	// Override exec command to use our mock
	origExec := runExecCommand
	defer func() { runExecCommand = origExec }()
	runExecCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(toolPath)
	}

	configDir := t.TempDir()
	configPath := createTestConfig(t, configDir, []toolconfig.Tool{
		{Name: "mock-tool", Path: toolPath, Supports: []string{"amadla.org/entity/test@v1.0.0"}},
	})

	entityDir := t.TempDir()
	writeTestHery(t, entityDir, "test.hery", `
_type: amadla.org/entity/test@v1.0.0
_body:
  name: test-entity
`)

	_, _, err := executeTestCmd("run", "--config", configPath, entityDir)
	require.NoError(t, err)

	// Verify mock tool was executed
	assert.FileExists(t, markerFile)
	data, _ := os.ReadFile(markerFile)
	assert.Contains(t, string(data), "executed")
}

func TestRun_MissingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, stderr, err := executeTestCmd("run", "--config", "/nonexistent/tools.hery")
	assert.Error(t, err)
	assert.Contains(t, stderr, "cannot read config")
}

// --- helpers ---

func writeHery(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
