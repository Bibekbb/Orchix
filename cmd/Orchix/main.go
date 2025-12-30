package main

import (
	"fmt"
	"os"

	"github.com/Bibekbb/Orchix/internal/cli"
	"github.com/spf13/cobra"
)

var Version = "v0.1.0-dev"

func main() {
	var configFile string
	var dryRun bool
	var target string

	rootCmd := &cobra.Command{
		Use:		"orchix",
		Short:		"Orchix - Deployement Orchestrator",
		Version: Version,
	}

	rootCmd.SetVersionTemplate(`Orchix: {{.Version}}`)

	versionCmd := &cobra.Command{
		Use:		"version",
		Short:		"Print version",
		Run:	func(cmd *cobra.Command, args []string)  {
					fmt.Printf("Orchix: %s\n", Version)
		},
	}

	initCmd	:=	&cobra.Command{
		Use:	"init",
		Short:	"Initialize project",
		Run:	func(cmd *cobra.Command, args []string)  {
					projectName	:= 	"my-app"
					if len(args) >0 {
						projectName = args[0]
					}
					fmt.Printf("Initializing Orchix project: %s\n", projectName)
					

					// Create Directories
					os.MkdirAll(".orchix", 0755)
					os.MkdirAll("infra", 0755)
					os.MkdirAll("k8s", 0755)
					os.MkdirAll("docker", 0755)

					// Create simple orchix.yaml

					yamlContent := `apiVersion: v1alpha1
appName: "` + projectName + `"
target: local-docker

variables:
  environment: development
  app_version: "1.0.0"

components:
  - id: backend
    name: Backend API
    type: docker
    source: ./docker/backend
    variables:
      port: 8080

  - id: frontend
    name: Frontend Web
    type: docker
    source: ./docker/frontend
    depends_on: [backend]
    variables:
      api_url: "http://backend:8080"
      port: 3000
`
			err := os.WriteFile("orchix.yaml", []byte(yamlContent), 0644)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			
			fmt.Println("✅ Created orchix.yaml")
			fmt.Println("✅ Created directory structure")
			fmt.Println("\nNext: orchix deploy")
		},
	}

	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy stack",
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

			// Use simple engine for now
			engine := cli.NewSimpleEngine(manifest)
			return engine.Deploy(cmd.Context(), dryRun)
		},
	}

	deployCmd.Flags().StringVarP(&configFile, "config", "c", "orchix.yaml", "Manifest file")
	deployCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run")
	deployCmd.Flags().StringVarP(&target, "target", "t", "", "Target environment")

	destroyCmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := cli.LoadManifest(configFile)
			if err != nil {
				return err
			}

			engine := cli.NewSimpleEngine(manifest)
			return engine.Destroy(cmd.Context())
		},
	}
	destroyCmd.Flags().StringVarP(&configFile, "config", "c", "orchix.yaml", "Manifest file")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status",
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := cli.LoadManifest(configFile)
			if err != nil {
				return err
			}

			engine := cli.NewSimpleEngine(manifest)
			return engine.Status(cmd.Context())
		},
	}
	statusCmd.Flags().StringVarP(&configFile, "config", "c", "orchix.yaml", "Manifest file")

	// Add all commands
	rootCmd.AddCommand(versionCmd, initCmd, deployCmd, destroyCmd, statusCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}





























// func main() {
// 	rootCmd := &cobra.Command{
// 		Use:     "orchix",
// 		Short:   "Orchix - Deployment Orchestrator",
// 		Version: Version,
// 	}

// 	// Set custom version template
// 	rootCmd.SetVersionTemplate(`Orchix: {{.Version}}`)

// 	versionCmd := &cobra.Command{
// 		Use:   "version",
// 		Short: "Print version",
// 		Run: func(cmd *cobra.Command, args []string) {
// 			fmt.Printf("Orchix: %s\n", Version)
// 		},
// 	}

// 	initCmd := &cobra.Command{
// 		Use:   "init",
// 		Short: "Initialize project",
// 		Run: func(cmd *cobra.Command, args []string) {
// 			fmt.Println("Project initialized")
// 		},
// 	}

// 	deployCmd := &cobra.Command{
// 		Use:   "deploy",
// 		Short: "Deploy stack",
// 		Run: func(cmd *cobra.Command, args []string) {
// 			fmt.Println("Deploying...")
// 		},
// 	}

// 	destroyCmd := &cobra.Command{
// 		Use:   "destroy",
// 		Short: "Destroy stack",
// 		Run: func(cmd *cobra.Command, args []string) {
// 			fmt.Println("Destroying...")
// 		},
// 	}

// 	statusCmd := &cobra.Command{
// 		Use:   "status",
// 		Short: "Show status",
// 		Run: func(cmd *cobra.Command, args []string) {
// 			fmt.Println("Status: Running")
// 		},
// 	}

// 	// Add all commands
// 	rootCmd.AddCommand(versionCmd, initCmd, deployCmd, destroyCmd, statusCmd)

// 	if err := rootCmd.Execute(); err != nil {
// 		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
// 		os.Exit(1)
// 	}
// }