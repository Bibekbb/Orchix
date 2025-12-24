package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "v0.1.0-dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "orchix",
		Short:   "Orchix - Deployment Orchestrator",
		Version: Version,
	}

	// Set custom version template
	rootCmd.SetVersionTemplate(`Orchix: {{.Version}}`)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Orchix: %s\n", Version)
		},
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize project",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Project initialized")
		},
	}

	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy stack",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Deploying...")
		},
	}

	destroyCmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy stack",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Destroying...")
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Status: Running")
		},
	}

	// Add all commands
	rootCmd.AddCommand(versionCmd, initCmd, deployCmd, destroyCmd, statusCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}