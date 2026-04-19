package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/spf13/cobra"
)

// Function variable for test mocking.
var initExecLookPath = exec.LookPath

// InitCmd bootstraps a tools.hery file by scanning PATH for standard Amadla tools.
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap tools.hery by scanning PATH for Amadla tools",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	configPath, err := toolconfig.DefaultConfigPath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config already exists at %s — remove it first to reinitialize", configPath)
	}

	// Scan PATH for standard tools
	var found []string
	for _, name := range toolconfig.StandardToolNames() {
		if _, err := initExecLookPath(name); err == nil {
			found = append(found, name)
		}
	}

	if len(found) == 0 {
		cmd.PrintErrln("No Amadla tools found in PATH.")
		cmd.PrintErrln("Install tools and ensure they are on your PATH, then re-run amadla init.")
		return nil
	}

	// Generate tools.hery
	content := "_type: amadla.org/entity/tools@v1.0.0\n_body:\n  tools:\n"
	for _, name := range found {
		content += fmt.Sprintf("    - name: %s\n", name)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	cmd.Printf("Created %s with %d tools:\n", configPath, len(found))
	for _, name := range found {
		cmd.Printf("  - %s\n", name)
	}

	return nil
}
