package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Version information
var (
	Version   = "v0.1.0"
	BuildDate = ""
	GitCommit = ""
)

// AgentConfig holds agent configuration
type AgentConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Watch struct {
		Enabled    bool          `yaml:"enabled"`
		Interval   time.Duration `yaml:"interval"`
		Directories []string     `yaml:"directories"`
	} `yaml:"watch"`
	Kubernetes struct {
		Enabled    bool   `yaml:"enabled"`
		Namespace  string `yaml:"namespace"`
		Kubeconfig string `yaml:"kubeconfig"`
	} `yaml:"kubernetes"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

// DeploymentStatus represents deployment status
type DeploymentStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Agent represents the Orchix agent
type Agent struct {
	config     *AgentConfig
	server     *http.Server
	deployments map[string]DeploymentStatus
	mu         sync.RWMutex
	stopChan   chan struct{}
}

// NewAgent creates a new agent instance
func NewAgent(config *AgentConfig) *Agent {
	return &Agent{
		config:     config,
		deployments: make(map[string]DeploymentStatus),
		stopChan:   make(chan struct{}),
	}
}

// Start starts the agent
func (a *Agent) Start() error {
	log.Println("🚀 Starting Orchix Agent...")
	log.Printf("Version: %s", Version)
	
	// Initialize HTTP server
	a.setupServer()
	
	// Start background workers
	go a.startBackgroundWorkers()
	
	// Start HTTP server
	log.Printf("🌐 Starting HTTP server on %s:%d", a.config.Server.Host, a.config.Server.Port)
	
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	
	return nil
}

// Stop stops the agent gracefully
func (a *Agent) Stop() error {
	log.Println("🛑 Stopping Orchix Agent...")
	
	// Signal background workers to stop
	close(a.stopChan)
	
	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	
	log.Println("Orchix Agent stopped gracefully")
	return nil
}

func (a *Agent) setupServer() {
	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("/health", a.handleHealth)
	
	// Metrics endpoint
	mux.HandleFunc("/metrics", a.handleMetrics)
	
	// API endpoints
	mux.HandleFunc("/api/v1/deployments", a.handleDeployments)
	mux.HandleFunc("/api/v1/deploy", a.handleDeploy)
	mux.HandleFunc("/api/v1/status/", a.handleStatus)
	mux.HandleFunc("/api/v1/logs/", a.handleLogs)
	
	// Webhook endpoint
	mux.HandleFunc("/webhook/github", a.handleGitHubWebhook)
	mux.HandleFunc("/webhook/gitlab", a.handleGitLabWebhook)
	
	a.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (a *Agent) startBackgroundWorkers() {
	// Start directory watcher if enabled
	if a.config.Watch.Enabled {
		go a.watchDirectories()
	}
	
	// Start Kubernetes watcher if enabled
	if a.config.Kubernetes.Enabled {
		go a.watchKubernetes()
	}
	
	// Start metrics collector
	go a.collectMetrics()
}

func (a *Agent) watchDirectories() {
	log.Println("👀 Starting directory watcher...")
	
	ticker := time.NewTicker(a.config.Watch.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			a.scanDirectories()
		case <-a.stopChan:
			log.Println("📁 Directory watcher stopped")
			return
		}
	}
}

func (a *Agent) scanDirectories() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for _, dir := range a.config.Watch.Directories {
		files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		if err != nil {
			log.Printf("❌ Error scanning directory %s: %v", dir, err)
			continue
		}
		
		for _, file := range files {
			log.Printf("📄 Found manifest: %s", file)
			// TODO: Process manifest files
		}
	}
}

func (a *Agent) watchKubernetes() {
	log.Println("Starting Kubernetes watcher...")
	
	// TODO: Implement Kubernetes resource watcher
	// This would watch for changes to deployments, pods, services, etc.
	
	<-a.stopChan
	log.Println("Kubernetes watcher stopped")
}

func (a *Agent) collectMetrics() {
	log.Println("📊 Starting metrics collector...")
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			a.updateMetrics()
		case <-a.stopChan:
			log.Println("Metrics collector stopped")
			return
		}
	}
}

func (a *Agent) updateMetrics() {
	// TODO: Collect and update metrics
	// This could include:
	// - Number of deployments
	// - Deployment success/failure rates
	// - Resource usage
	// - Error counts
}

// HTTP Handlers
func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   Version,
	})
}

func (a *Agent) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	// Basic metrics for now
	fmt.Fprintf(w, "# HELP orchix_deployments_total Total number of deployments\n")
	fmt.Fprintf(w, "# TYPE orchix_deployments_total gauge\n")
	fmt.Fprintf(w, "orchix_deployments_total %d\n", len(a.deployments))
	
	fmt.Fprintf(w, "# HELP orchix_deployments_running Number of running deployments\n")
	fmt.Fprintf(w, "# TYPE orchix_deployments_running gauge\n")
	running := 0
	for _, dep := range a.deployments {
		if dep.Status == "running" {
			running++
		}
	}
	fmt.Fprintf(w, "orchix_deployments_running %d\n", running)
}

func (a *Agent) handleDeployments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	deployments := make([]DeploymentStatus, 0, len(a.deployments))
	for _, dep := range a.deployments {
		deployments = append(deployments, dep)
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deployments": deployments,
		"count":       len(deployments),
	})
}

func (a *Agent) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Manifest string `json:"manifest"`
		Path     string `json:"path"`
		Target   string `json:"target"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Generate deployment ID
	deploymentID := fmt.Sprintf("dep-%d", time.Now().Unix())
	
	// Create deployment status
	deployment := DeploymentStatus{
		ID:        deploymentID,
		Name:      "Manual Deployment",
		Status:    "pending",
		Message:   "Deployment started",
		Timestamp: time.Now(),
	}
	
	a.mu.Lock()
	a.deployments[deploymentID] = deployment
	a.mu.Unlock()
	
	log.Printf("Starting deployment %s", deploymentID)
	
	// Start deployment in background
	go a.executeDeployment(deploymentID, req.Manifest, req.Path, req.Target)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      deploymentID,
		"status":  "accepted",
		"message": "Deployment started",
	})
}

func (a *Agent) handleStatus(w http.ResponseWriter, r *http.Request) {
	deploymentID := r.URL.Path[len("/api/v1/status/"):]
	
	a.mu.RLock()
	deployment, exists := a.deployments[deploymentID]
	a.mu.RUnlock()
	
	if !exists {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deployment)
}

func (a *Agent) handleLogs(w http.ResponseWriter, r *http.Request) {
	deploymentID := r.URL.Path[len("/api/v1/logs/"):]
	
	// TODO: Return actual logs for deployment
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    deploymentID,
		"logs":  []string{"Logs not implemented yet"},
		"count": 0,
	})
}

func (a *Agent) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// TODO: Implement GitHub webhook handling
	log.Println("📦 Received GitHub webhook")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "received",
		"message": "GitHub webhook received",
	})
}

func (a *Agent) handleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// TODO: Implement GitLab webhook handling
	log.Println("Received GitLab webhook")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "received",
		"message": "GitLab webhook received",
	})
}

func (a *Agent) executeDeployment(id, manifest, path, target string) {
	// Simulate deployment process
	log.Printf("⚡ Executing deployment %s", id)
	
	a.updateDeploymentStatus(id, "running", "Building components...")
	time.Sleep(2 * time.Second)
	
	a.updateDeploymentStatus(id, "running", "Deploying to target...")
	time.Sleep(3 * time.Second)
	
	a.updateDeploymentStatus(id, "completed", "Deployment successful")
	
	log.Printf("Deployment %s completed", id)
}

func (a *Agent) updateDeploymentStatus(id, status, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if dep, exists := a.deployments[id]; exists {
		dep.Status = status
		dep.Message = message
		dep.Timestamp = time.Now()
		a.deployments[id] = dep
	}
}

// LoadConfig loads agent configuration
func LoadConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config AgentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Set defaults
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Watch.Interval == 0 {
		config.Watch.Interval = 30 * time.Second
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	
	return &config, nil
}

func main() {
	var configFile string
	var verbose bool
	
	rootCmd := &cobra.Command{
		Use:     "orchix-agent",
		Short:   "Orchix Agent - GitOps deployment agent",
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(configFile, verbose)
		},
	}
	
	rootCmd.SetVersionTemplate(`Orchix Agent: {{.Version}}`)
	
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "orchix-agent.yaml", "Configuration file")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAgent(configFile string, verbose bool) error {
	// Load configuration
	config, err := LoadConfig(configFile)
	if err != nil {
		// Try to create default config if file doesn't exist
		if os.IsNotExist(err) {
			config = createDefaultConfig()
			if err := saveDefaultConfig(configFile, config); err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
			log.Printf("Created default config at %s", configFile)
		} else {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}
	
	// Setup logging
	if verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
	
	// Create agent
	agent := NewAgent(config)
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start agent in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := agent.Start(); err != nil {
			errChan <- err
		}
	}()
	
	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		if err := agent.Stop(); err != nil {
			return fmt.Errorf("failed to stop agent: %w", err)
		}
	case err := <-errChan:
		return fmt.Errorf("agent error: %w", err)
	}
	
	return nil
}

func createDefaultConfig() *AgentConfig {
	config := &AgentConfig{}
	
	// Server configuration
	config.Server.Host = "0.0.0.0"
	config.Server.Port = 8080
	
	// Watch configuration
	config.Watch.Enabled = true
	config.Watch.Interval = 30 * time.Second
	config.Watch.Directories = []string{"./manifests", "./orchix-manifests"}
	
	// Kubernetes configuration
	config.Kubernetes.Enabled = true
	config.Kubernetes.Namespace = "default"
	config.Kubernetes.Kubeconfig = ""
	
	// Logging configuration
	config.Logging.Level = "info"
	config.Logging.Format = "text"
	
	return config
}

func saveDefaultConfig(path string, config *AgentConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}
