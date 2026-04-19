package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadlaOrg/amadla/toolconfig"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// newTestRoot creates a fresh root command with fresh subcommand instances
// to avoid Cobra flag state leaking between tests.
func newTestRoot() *cobra.Command {
	rootCmd := &cobra.Command{Use: "amadla"}
	rootCmd.PersistentFlags().String("config", "", "")

	runCmd := &cobra.Command{
		Use:  "run [entity-dir]",
		Args: cobra.MaximumNArgs(1),
		RunE: runPipeline,
	}
	runCmd.Flags().Bool("dry-run", false, "Print execution order without running tools")

	initCmd := &cobra.Command{
		Use:  "init",
		RunE: runInit,
	}

	listCmd := &cobra.Command{
		Use:  "list",
		RunE: runList,
	}

	doctorCmd := &cobra.Command{
		Use:  "doctor",
		RunE: runDoctor,
	}

	rootCmd.AddCommand(runCmd, initCmd, listCmd, doctorCmd)
	return rootCmd
}

// executeTestCmd runs a command and returns captured stdout, stderr, and error.
func executeTestCmd(args ...string) (string, string, error) {
	rootCmd := newTestRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

// createMockTool creates an executable shell script that responds to "info" and "--version".
func createMockTool(t *testing.T, dir, name string, entityTypes []string) string {
	t.Helper()

	typesJSON := "[]"
	if len(entityTypes) > 0 {
		typesJSON = "["
		for i, et := range entityTypes {
			if i > 0 {
				typesJSON += ","
			}
			typesJSON += fmt.Sprintf(`"%s"`, et)
		}
		typesJSON += "]"
	}

	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
    info)
        echo '{"name":"%s","version":"1.0.0","supports":%s,"description":"%s tool"}'
        ;;
    --version)
        echo "%s 1.0.0"
        ;;
    *)
        cat
        ;;
esac
`, name, typesJSON, name, name)

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

// createTestConfig writes a tools.hery file and returns its path.
func createTestConfig(t *testing.T, dir string, tools []toolconfig.Tool) string {
	t.Helper()

	content := "_type: amadla.org/entity/tools@v1.0.0\n_body:\n  tools:\n"
	for _, tool := range tools {
		content += fmt.Sprintf("    - name: %s\n", tool.Name)
		if tool.Path != "" {
			content += fmt.Sprintf("      path: %s\n", tool.Path)
		}
		if tool.Required {
			content += "      required: true\n"
		}
		if len(tool.Supports) > 0 {
			content += "      supports:\n"
			for _, et := range tool.Supports {
				content += fmt.Sprintf("        - %s\n", et)
			}
		}
	}

	configPath := filepath.Join(dir, "tools.hery")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	return configPath
}

// writeTestHery creates a .hery file in the given directory.
func writeTestHery(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}
