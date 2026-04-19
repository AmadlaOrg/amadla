package toolconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

// Function variables for test mocking.
var (
	osStat       = os.Stat
	execLookPath = exec.LookPath
	osReadFile   = os.ReadFile
	osUserHomeDir = os.UserHomeDir
)

// Tool represents a single tool entry from tools.hery.
type Tool struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Required    bool     `yaml:"required,omitempty"`
	Path        string   `yaml:"path,omitempty"`
	Supports []string `yaml:"supports,omitempty"`
}

// Config represents the parsed tools.hery file.
type Config struct {
	Type string `yaml:"_type"`
	Body struct {
		Tools []Tool `yaml:"tools"`
	} `yaml:"_body"`
}

// DefaultConfigPath returns ~/.config/amadla/tools.hery.
func DefaultConfigPath() (string, error) {
	home, err := osUserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "amadla", "tools.hery"), nil
}

// Load reads and parses a tools.hery file.
func Load(path string) (*Config, error) {
	data, err := osReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config %s: %w", path, err)
	}

	return &cfg, nil
}

// ResolvePath returns the full path to a tool binary.
// If the tool has an explicit Path, it returns that. Otherwise it looks up PATH.
func ResolvePath(tool Tool) (string, error) {
	if tool.Path != "" {
		if _, err := osStat(tool.Path); err != nil {
			return "", fmt.Errorf("tool %s: explicit path %s not found", tool.Name, tool.Path)
		}
		return tool.Path, nil
	}

	path, err := execLookPath(tool.Name)
	if err != nil {
		return "", fmt.Errorf("tool %s: not found in PATH", tool.Name)
	}
	return path, nil
}

// StandardToolNames returns the default set of Amadla tool names for PATH scanning.
func StandardToolNames() []string {
	return []string{
		"hery", "doorman", "weaver", "lay", "enjoin", "waiter", "raise",
		"unravel", "judge", "conduct", "lighthouse", "garbage", "dryrun",
	}
}

// ToolInfo holds cached metadata from `<tool> info` output.
type ToolInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Supports     []string `json:"supports,omitempty"`
	Extensions   []string `json:"file_extensions,omitempty"`
	Description  string   `json:"description,omitempty"`
	BinaryPath   string   `json:"binary_path"`
	BinaryMtime  int64    `json:"binary_mtime"`
	CachedAt     int64    `json:"cached_at"`
}

// InfoCache manages cached tool info with mtime-based invalidation.
type InfoCache struct {
	mu      sync.RWMutex
	entries map[string]*ToolInfo
	path    string
}

// NewInfoCache creates a cache backed by the given file path.
func NewInfoCache(cachePath string) *InfoCache {
	c := &InfoCache{
		entries: make(map[string]*ToolInfo),
		path:    cachePath,
	}
	c.load()
	return c
}

// Get returns cached tool info, or calls `<tool> info` if cache is stale or missing.
// Cache is invalidated when the binary's mtime changes.
func (c *InfoCache) Get(toolName string) (*ToolInfo, error) {
	toolPath, err := execLookPath(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool %s not found in PATH", toolName)
	}

	stat, err := osStat(toolPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat %s: %w", toolPath, err)
	}
	binaryMtime := stat.ModTime().Unix()

	// Check cache
	c.mu.RLock()
	cached, ok := c.entries[toolName]
	c.mu.RUnlock()

	if ok && cached.BinaryPath == toolPath && cached.BinaryMtime == binaryMtime {
		return cached, nil
	}

	// Cache miss or stale — call `<tool> info`
	info, err := fetchToolInfo(toolPath)
	if err != nil {
		return nil, err
	}
	info.BinaryPath = toolPath
	info.BinaryMtime = binaryMtime
	info.CachedAt = time.Now().Unix()

	c.mu.Lock()
	c.entries[toolName] = info
	c.mu.Unlock()

	c.save()
	return info, nil
}

// fetchToolInfo runs `<tool> info` and parses JSON output.
func fetchToolInfo(toolPath string) (*ToolInfo, error) {
	cmd := execCommand(toolPath, "info", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run %s info: %w", toolPath, err)
	}

	var info ToolInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("failed to parse %s info output: %w", toolPath, err)
	}
	return &info, nil
}

// load reads the cache file from disk.
func (c *InfoCache) load() {
	data, err := osReadFile(c.path)
	if err != nil {
		return // no cache file yet
	}
	var entries map[string]*ToolInfo
	if err := json.Unmarshal(data, &entries); err != nil {
		return // corrupt cache, start fresh
	}
	c.entries = entries
}

// save writes the cache to disk.
func (c *InfoCache) save() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(c.path)
	os.MkdirAll(dir, 0755)
	os.WriteFile(c.path, data, 0644)
}

// DefaultCachePath returns ~/.cache/amadla/tool-info.json.
func DefaultCachePath() (string, error) {
	home, err := osUserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "amadla", "tool-info.json"), nil
}

// EnrichWithInfo fills in missing Supports from cached `<tool> info` output.
// Tools that already have Supports configured in tools.hery are left unchanged.
func EnrichWithInfo(cfg *Config, cache *InfoCache) {
	for i := range cfg.Body.Tools {
		if len(cfg.Body.Tools[i].Supports) > 0 {
			continue
		}
		info, err := cache.Get(cfg.Body.Tools[i].Name)
		if err != nil {
			continue
		}
		cfg.Body.Tools[i].Supports = info.Supports
		if cfg.Body.Tools[i].Description == "" && info.Description != "" {
			cfg.Body.Tools[i].Description = info.Description
		}
	}
}

// Function variable for test mocking.
var execCommand = exec.Command
