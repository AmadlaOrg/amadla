package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/spf13/cobra"
)

// Function variable for test mocking.
var doctorExecCommand = exec.Command

// DoctorCmd checks that all registered tools are installed and working.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that all tools are installed and compatible",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Root().Flags().GetString("config")
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	enrichConfig(cfg)

	issues := 0

	for _, tool := range cfg.Body.Tools {
		path, pathErr := toolconfig.ResolvePath(tool)

		if pathErr != nil {
			if tool.Required {
				cmd.PrintErrf("FAIL: %s — required but not found in PATH\n", tool.Name)
				issues++
			} else {
				cmd.Printf("SKIP: %s — not found (optional)\n", tool.Name)
			}
			continue
		}

		// Try running <tool> --version to verify it works
		out, vErr := doctorExecCommand(path, "--version").CombinedOutput()
		version := strings.TrimSpace(string(out))
		if vErr != nil {
			version = "unknown"
		}

		cmd.Printf("  OK: %s — %s (%s)\n", tool.Name, version, path)
	}

	if issues > 0 {
		return fmt.Errorf("%d required tool(s) missing", issues)
	}

	cmd.Println("\nAll required tools are available.")
	return nil
}
