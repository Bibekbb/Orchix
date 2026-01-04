package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/internal/utils"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// HelmClient provides a wrapper around Helm CLI operations
type HelmClient struct {
	kubeconfig  string
	context     string
	namespace   string
	timeout     time.Duration
	debug       bool
	logger      *utils.Logger
}

// NewHelmClient creates a new Helm client
func NewHelmClient(kubeconfig, context, namespace string) *HelmClient {
	return &HelmClient{
		kubeconfig: kubeconfig,
		context:    context,
		namespace:  namespace,
		timeout:    5 * time.Minute,
		debug:      false,
		logger:     utils.NewLogger("helm", utils.LevelInfo),
	}
}

// SetTimeout sets the command timeout
func (c *HelmClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// SetDebug enables debug logging
func (c *HelmClient) SetDebug(debug bool) {
	c.debug = debug
}

// AddRepository adds a Helm repository
func (c *HelmClient) AddRepository(ctx context.Context, name, url string) error {
	args := []string{"repo", "add", name, url}
	
	if c.debug {
		args = append(args, "--debug")
	}
	
	return c.runHelm(ctx, args...)
}

// UpdateRepositories updates all Helm repositories
func (c *HelmClient) UpdateRepositories(ctx context.Context) error {
	args := []string{"repo", "update"}
	
	if c.debug {
		args = append(args, "--debug")
	}
	
	return c.runHelm(ctx, args...)
}

// Install installs a Helm chart
func (c *HelmClient) Install(ctx context.Context, releaseName, chart string, options *InstallOptions) (*ReleaseInfo, error) {
	args := []string{"install", releaseName, chart}
	
	// Add namespace
	if options.Namespace != "" {
		args = append(args, "--namespace", options.Namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	// Add values files
	for _, file := range options.ValuesFiles {
		args = append(args, "--values", file)
	}
	
	// Add set values
	for key, value := range options.SetValues {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-string values
	for key, value := range options.SetStringValues {
		args = append(args, "--set-string", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-file values
	for key, file := range options.SetFileValues {
		args = append(args, "--set-file", fmt.Sprintf("%s=%s", key, file))
	}
	
	// Add timeout
	if options.Timeout > 0 {
		args = append(args, "--timeout", options.Timeout.String())
	}
	
	// Add wait flag
	if options.Wait {
		args = append(args, "--wait")
	}
	
	// Add atomic flag
	if options.Atomic {
		args = append(args, "--atomic")
	}
	
	// Add create namespace flag
	if options.CreateNamespace {
		args = append(args, "--create-namespace")
	}
	
	// Add debug flag
	if c.debug {
		args = append(args, "--debug")
	}
	
	// Run install
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm install failed: %w\nStderr: %s", err, stderr.String())
	}
	
	// Get release info
	return c.GetRelease(ctx, releaseName, options.Namespace)
}

// Upgrade upgrades a Helm release
func (c *HelmClient) Upgrade(ctx context.Context, releaseName, chart string, options *UpgradeOptions) (*ReleaseInfo, error) {
	args := []string{"upgrade", releaseName, chart}
	
	// Add namespace
	if options.Namespace != "" {
		args = append(args, "--namespace", options.Namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	// Add values files
	for _, file := range options.ValuesFiles {
		args = append(args, "--values", file)
	}
	
	// Add set values
	for key, value := range options.SetValues {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-string values
	for key, value := range options.SetStringValues {
		args = append(args, "--set-string", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-file values
	for key, file := range options.SetFileValues {
		args = append(args, "--set-file", fmt.Sprintf("%s=%s", key, file))
	}
	
	// Add timeout
	if options.Timeout > 0 {
		args = append(args, "--timeout", options.Timeout.String())
	}
	
	// Add wait flag
	if options.Wait {
		args = append(args, "--wait")
	}
	
	// Add atomic flag
	if options.Atomic {
		args = append(args, "--atomic")
	}
	
	// Add install flag (if release doesn't exist)
	if options.Install {
		args = append(args, "--install")
	}
	
	// Add debug flag
	if c.debug {
		args = append(args, "--debug")
	}
	
	// Run upgrade
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm upgrade failed: %w\nStderr: %s", err, stderr.String())
	}
	
	// Get release info
	return c.GetRelease(ctx, releaseName, options.Namespace)
}

// Uninstall uninstalls a Helm release
func (c *HelmClient) Uninstall(ctx context.Context, releaseName string, options *UninstallOptions) error {
	args := []string{"uninstall", releaseName}
	
	// Add namespace
	if options.Namespace != "" {
		args = append(args, "--namespace", options.Namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	// Add timeout
	if options.Timeout > 0 {
		args = append(args, "--timeout", options.Timeout.String())
	}
	
	// Add wait flag
	if options.Wait {
		args = append(args, "--wait")
	}
	
	// Add debug flag
	if c.debug {
		args = append(args, "--debug")
	}
	
	// Run uninstall
	return c.runHelm(ctx, args...)
}

// GetRelease gets information about a Helm release
func (c *HelmClient) GetRelease(ctx context.Context, releaseName, namespace string) (*ReleaseInfo, error) {
	args := []string{"status", releaseName, "--output", "json"}
	
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm status failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var info ReleaseInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	
	return &info, nil
}

// ListReleases lists all Helm releases
func (c *HelmClient) ListReleases(ctx context.Context, namespace string) ([]ReleaseInfo, error) {
	args := []string{"list", "--output", "json"}
	
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	if c.allNamespaces {
		args = append(args, "--all-namespaces")
	}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm list failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var releases []ReleaseInfo
	if err := json.Unmarshal(stdout.Bytes(), &releases); err != nil {
		return nil, fmt.Errorf("failed to parse releases: %w", err)
	}
	
	return releases, nil
}

// GetValues gets values for a Helm release
func (c *HelmClient) GetValues(ctx context.Context, releaseName, namespace string) (map[string]interface{}, error) {
	args := []string{"get", "values", releaseName, "--output", "json"}
	
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm get values failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var values map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &values); err != nil {
		return nil, fmt.Errorf("failed to parse values: %w", err)
	}
	
	return values, nil
}

// Template renders Helm templates without installing
func (c *HelmClient) Template(ctx context.Context, releaseName, chart string, options *TemplateOptions) (string, error) {
	args := []string{"template", releaseName, chart}
	
	// Add namespace
	if options.Namespace != "" {
		args = append(args, "--namespace", options.Namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	// Add values files
	for _, file := range options.ValuesFiles {
		args = append(args, "--values", file)
	}
	
	// Add set values
	for key, value := range options.SetValues {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-string values
	for key, value := range options.SetStringValues {
		args = append(args, "--set-string", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add set-file values
	for key, file := range options.SetFileValues {
		args = append(args, "--set-file", fmt.Sprintf("%s=%s", key, file))
	}
	
	// Add output directory if specified
	if options.OutputDir != "" {
		args = append(args, "--output-dir", options.OutputDir)
	}
	
	// Add debug flag
	if c.debug {
		args = append(args, "--debug")
	}
	
	// Run template
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return "", fmt.Errorf("helm template failed: %w\nStderr: %s", err, stderr.String())
	}
	
	return stdout.String(), nil
}

// History gets the release history
func (c *HelmClient) History(ctx context.Context, releaseName, namespace string) ([]RevisionInfo, error) {
	args := []string{"history", releaseName, "--output", "json"}
	
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("helm history failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var history []RevisionInfo
	if err := json.Unmarshal(stdout.Bytes(), &history); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}
	
	return history, nil
}

// Rollback rolls back a release to a previous revision
func (c *HelmClient) Rollback(ctx context.Context, releaseName string, revision int, namespace string) error {
	args := []string{"rollback", releaseName, fmt.Sprintf("%d", revision)}
	
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	} else if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}
	
	// Add wait flag
	args = append(args, "--wait")
	
	return c.runHelm(ctx, args...)
}

// Lint validates a Helm chart
func (c *HelmClient) Lint(ctx context.Context, chartPath string) (*LintResult, error) {
	args := []string{"lint", chartPath}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return &LintResult{
			Valid:   false,
			Errors:  []string{stderr.String()},
			Message: "Chart validation failed",
		}, nil
	}
	
	return &LintResult{
		Valid:   true,
		Message: "Chart is valid",
		Output:  stdout.String(),
	}, nil
}

// CheckHealth checks if Helm is properly configured
func (c *HelmClient) CheckHealth(ctx context.Context) error {
	// Check helm version
	args := []string{"version", "--short"}
	
	var stdout, stderr bytes.Buffer
	if err := c.runHelmWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return fmt.Errorf("helm not available: %w", err)
	}
	
	// Check if we can list releases (tests kubeconfig)
	_, err := c.ListReleases(ctx, "default")
	if err != nil {
		return fmt.Errorf("helm cannot connect to Kubernetes: %w", err)
	}
	
	return nil
}

// CreateValuesFile creates a values.yaml file from a map
func (c *HelmClient) CreateValuesFile(values map[string]interface{}, outputPath string) error {
	data, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to marshal values: %w", err)
	}
	
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write values file: %w", err)
	}
	
	return nil
}

// Helper methods
func (c *HelmClient) runHelm(ctx context.Context, args ...string) error {
	var stdout, stderr bytes.Buffer
	return c.runHelmWithOutput(ctx, &stdout, &stderr, args...)
}

func (c *HelmClient) runHelmWithOutput(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "helm", args...)
	
	// Set environment
	env := os.Environ()
	if c.kubeconfig != "" {
		env = append(env, fmt.Sprintf("KUBECONFIG=%s", c.kubeconfig))
	}
	cmd.Env = env
	
	// Set kubectl context if specified
	if c.context != "" {
		args = append(args, "--kube-context", c.context)
	}
	
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	
	c.logger.Debug("Running: helm %s", strings.Join(args, " "))
	
	return cmd.Run()
}

// Types

type InstallOptions struct {
	Namespace       string
	ValuesFiles     []string
	SetValues       map[string]string
	SetStringValues map[string]string
	SetFileValues   map[string]string
	Timeout         time.Duration
	Wait            bool
	Atomic          bool
	CreateNamespace bool
	Version         string
}

type UpgradeOptions struct {
	Namespace       string
	ValuesFiles     []string
	SetValues       map[string]string
	SetStringValues map[string]string
	SetFileValues   map[string]string
	Timeout         time.Duration
	Wait            bool
	Atomic          bool
	Install         bool
	Version         string
}

type UninstallOptions struct {
	Namespace string
	Timeout   time.Duration
	Wait      bool
}

type TemplateOptions struct {
	Namespace   string
	ValuesFiles []string
	SetValues   map[string]string
	SetStringValues map[string]string
	SetFileValues map[string]string
	OutputDir   string
}

type ReleaseInfo struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Revision    int       `json:"revision"`
	Updated     string    `json:"updated"`
	Status      string    `json:"status"`
	Chart       string    `json:"chart"`
	AppVersion  string    `json:"app_version"`
	Description string    `json:"description"`
	Notes       string    `json:"notes"`
	Resources   []ResourceInfo `json:"resources,omitempty"`
}

type ResourceInfo struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

type RevisionInfo struct {
	Revision    int       `json:"revision"`
	Updated     string    `json:"updated"`
	Status      string    `json:"status"`
	Chart       string    `json:"chart"`
	AppVersion  string    `json:"app_version"`
	Description string    `json:"description"`
}

type LintResult struct {
	Valid   bool     `json:"valid"`
	Message string   `json:"message"`
	Errors  []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Output  string   `json:"output,omitempty"`
}

// ChartInfo represents information about a Helm chart
type ChartInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	AppVersion  string                 `json:"appVersion"`
	Description string                 `json:"description"`
	APIVersion  string                 `json:"apiVersion"`
	Type        string                 `json:"type"`
	Annotations map[string]string      `json:"annotations,omitempty"`
	Dependencies []ChartDependency     `json:"dependencies,omitempty"`
}

type ChartDependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Condition  string `json:"condition,omitempty"`
}

// RepoInfo represents information about a Helm repository
type RepoInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// SearchResult represents a Helm search result
type SearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`
	Description string `json:"description"`
	ChartType   string `json:"chartType"`
}

// ParseChartReference parses a chart reference string
func ParseChartReference(ref string) (repo, chart, version string, err error) {
	parts := strings.Split(ref, "/")
	if len(parts) == 2 {
		// repo/chart or repo/chart@version
		repoChart := parts[1]
		if idx := strings.Index(repoChart, "@"); idx != -1 {
			chart = repoChart[:idx]
			version = repoChart[idx+1:]
		} else {
			chart = repoChart
		}
		repo = parts[0]
	} else if len(parts) == 1 {
		// chart or chart@version
		if idx := strings.Index(ref, "@"); idx != -1 {
			chart = ref[:idx]
			version = ref[idx+1:]
		} else {
			chart = ref
		}
		repo = "" // Use default repo
	} else {
		return "", "", "", fmt.Errorf("invalid chart reference: %s", ref)
	}
	
	return repo, chart, version, nil
}