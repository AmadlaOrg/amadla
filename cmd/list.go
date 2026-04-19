package cmd

import (
	"fmt"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/spf13/cobra"
)

// ListCmd shows registered tools and their PATH status.
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show registered tools and their status",
	RunE:  runList,
}

func runList(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Root().Flags().GetString("config")
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	enrichConfig(cfg)

	cmd.Printf("%-15s %-10s %-40s %s\n", "TOOL", "STATUS", "PATH", "ENTITY TYPES")
	cmd.Printf("%-15s %-10s %-40s %s\n", "----", "------", "----", "------------")

	for _, tool := range cfg.Body.Tools {
		path, pathErr := toolconfig.ResolvePath(tool)

		status := "found"
		if pathErr != nil {
			status = "missing"
			path = "—"
		}

		entityTypes := "—"
		if len(tool.Supports) > 0 {
			entityTypes = fmt.Sprintf("%v", tool.Supports)
		}

		if pathErr != nil && tool.Required {
			status = "MISSING!"
			cmd.PrintErrf("error: required tool %s not found\n", tool.Name)
		}

		cmd.Printf("%-15s %-10s %-40s %s\n", tool.Name, status, path, entityTypes)
	}

	return nil
}
