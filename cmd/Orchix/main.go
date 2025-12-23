package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Bibekbb/Orchix/internal/cli"
)

func main() {
	var configFile string
	var dryRun bool
	var target string

	var rootCmd = &cobra.Command{
		Use:   "orchix",
		Short: "Unified deployment orchestrator",
		Long:  "Orchix - Declarative deployment orchestrator for modern application stacks",
	}

	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy application stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load manifest
			manifest, err := cli.LoadManifest(configFile)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}

			// Override target if specified
			if target != "" {
				manifest.Target = target
			}

			// Create and run engine
			engine, err := cli.NewEngine(manifest)
			if err != nil {
				return fmt.Errorf("failed to create engine: %w", err)
			}
			return engine.Deploy(cmd.Context(), dryRun)
		},
	}

	deployCmd.Flags().StringVarP(&configFile, "config", "c", "orchix.yaml", "Manifest file")
	deployCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show execution plan without applying")
	deployCmd.Flags().StringVarP(&target, "target", "t", "", "Override target environment")

	var destroyCmd = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy application stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := cli.LoadManifest(configFile)
			if err != nil {
				return err
			}

			engine, err := cli.NewEngine(manifest)
			if err != nil {
				return fmt.Errorf("failed to create engine: %w", err)
			}
			return engine.Deploy(cmd.Context(), dryRun)
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show deployment status",
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := cli.LoadManifest(configFile)
			if err != nil {
				return err
			}

			engine, err := cli.NewEngine(manifest)
			if err != nil {
				return fmt.Errorf("failed to create engine: %w", err)
			}
			return engine.Deploy(cmd.Context(), dryRun)
		},
	}

	rootCmd.AddCommand(deployCmd, destroyCmd, statusCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
