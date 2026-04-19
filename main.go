package main

import (
	"fmt"
	"os"

	"github.com/AmadlaOrg/amadla/cmd"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "amadla",
		Short: "Amadla — thin orchestrator for the Amadla tool pipeline",
		Long:  "amadla reads tools.hery, resolves _requires into a DAG, and shells out to registered tools.",
	}

	rootCmd.PersistentFlags().String("config", "", "Path to tools.hery (default: ~/.config/amadla/tools.hery)")

	rootCmd.AddCommand(cmd.RunCmd)
	rootCmd.AddCommand(cmd.InitCmd)
	rootCmd.AddCommand(cmd.ListCmd)
	rootCmd.AddCommand(cmd.DoctorCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
