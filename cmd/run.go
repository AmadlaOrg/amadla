package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/AmadlaOrg/amadla/dag"
	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// Function variables for test mocking.
var (
	runExecCommand = exec.Command
	osReadDir      = os.ReadDir
	osReadFile     = os.ReadFile
)

// RunCmd executes a pipeline by resolving the _requires DAG and shelling out to tools.
var RunCmd = &cobra.Command{
	Use:   "run [entity-dir]",
	Short: "Execute pipeline — resolve _requires DAG and run tools in order",
	Long:  "Reads .hery files, builds a dependency graph from _requires, topologically sorts, and executes each tool.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPipeline,
}

func init() {
	RunCmd.Flags().Bool("dry-run", false, "Print execution order without running tools")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// 1. Load tools config
	configPath, _ := cmd.Root().Flags().GetString("config")
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	// 2. Enrich tools with cached `<tool> info` discovery
	enrichConfig(cfg)

	// 3. Build tool lookup (entity_type → tool)
	toolMap := buildToolMap(cfg)

	// 4. Read entities and build DAG
	entityDir := "."
	if len(args) > 0 {
		entityDir = args[0]
	}

	graph, entities, err := buildDAG(entityDir)
	if err != nil {
		return err
	}

	// 5. Group into parallel execution levels
	levels, err := graph.Levels()
	if err != nil {
		return fmt.Errorf("DAG resolution failed: %w", err)
	}

	if dryRun {
		cmd.Println("Execution order:")
		for i, tier := range levels {
			if len(tier) == 1 {
				cmd.Printf("  %d. %s\n", i+1, tier[0])
			} else {
				cmd.Printf("  %d. [parallel] %v\n", i+1, tier)
			}
		}
		return nil
	}

	// 6. Execute level by level — nodes within a level run in parallel
	for _, tier := range levels {
		if err := executeTier(tier, entities, toolMap); err != nil {
			return err
		}
	}

	return nil
}

// executeTier runs all entities in a tier. If there's only one, it runs sequentially.
// If there are multiple, they run in parallel with errors collected.
func executeTier(tier []string, entities map[string]map[string]any, toolMap map[string]toolconfig.Tool) error {
	if len(tier) == 1 {
		return executeEntity(tier[0], entities, toolMap)
	}

	// Parallel execution for independent nodes
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for _, uri := range tier {
		wg.Add(1)
		go func(uri string) {
			defer wg.Done()
			if err := executeEntity(uri, entities, toolMap); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(uri)
	}

	wg.Wait()
	return firstErr
}

// executeEntity runs a single entity through its registered tool.
func executeEntity(uri string, entities map[string]map[string]any, toolMap map[string]toolconfig.Tool) error {
	entity, ok := entities[uri]
	if !ok {
		return nil // referenced but not present locally
	}

	typeVal, _ := entity["_type"].(string)
	tool, ok := toolMap[typeVal]
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: no tool registered for entity type %s (entity %s), skipping\n", typeVal, uri)
		return nil
	}

	toolPath, err := toolconfig.ResolvePath(tool)
	if err != nil {
		if tool.Required {
			return fmt.Errorf("required tool missing: %w", err)
		}
		fmt.Fprintf(os.Stderr, "warning: %v, skipping\n", err)
		return nil
	}

	// Pipe entity JSON to tool via stdin and env
	entityJSON, _ := json.Marshal(entity)

	fmt.Fprintf(os.Stderr, "--- running %s for %s ---\n", tool.Name, uri)
	c := runExecCommand(toolPath)
	c.Stdin = bytes.NewReader(entityJSON)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), "AMADLA_ENTITY="+string(entityJSON))

	if err := c.Run(); err != nil {
		return fmt.Errorf("tool %s failed for %s: %w", tool.Name, uri, err)
	}
	return nil
}

func loadConfig(configPath string) (*toolconfig.Config, error) {
	if configPath == "" {
		var err error
		configPath, err = toolconfig.DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	return toolconfig.Load(configPath)
}

// enrichConfig fills in missing Supports from cached `<tool> info` output.
func enrichConfig(cfg *toolconfig.Config) {
	cachePath, err := toolconfig.DefaultCachePath()
	if err != nil {
		return
	}
	cache := toolconfig.NewInfoCache(cachePath)
	toolconfig.EnrichWithInfo(cfg, cache)
}

func buildToolMap(cfg *toolconfig.Config) map[string]toolconfig.Tool {
	m := make(map[string]toolconfig.Tool)
	for _, tool := range cfg.Body.Tools {
		for _, entityType := range tool.Supports {
			m[entityType] = tool
		}
	}
	return m
}

// buildDAG reads .hery files from a directory and builds the dependency graph.
func buildDAG(dir string) (*dag.Graph, map[string]map[string]any, error) {
	entries, err := osReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read directory %s: %w", dir, err)
	}

	graph := dag.New()
	entities := make(map[string]map[string]any)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".hery" {
			continue
		}

		data, err := osReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, nil, fmt.Errorf("cannot read %s: %w", entry.Name(), err)
		}

		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, nil, fmt.Errorf("cannot parse %s: %w", entry.Name(), err)
		}

		typeVal, _ := doc["_type"].(string)
		if typeVal == "" {
			continue
		}

		// Use _type as the URI for now (entity identity from git path comes later)
		uri := typeVal

		var requires []string
		if rawRequires, ok := doc["_requires"].([]any); ok {
			for _, item := range rawRequires {
				if s, ok := item.(string); ok {
					requires = append(requires, s)
				}
			}
		}

		graph.Add(dag.Node{URI: uri, Requires: requires})
		entities[uri] = doc
	}

	return graph, entities, nil
}
