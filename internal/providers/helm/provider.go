package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/internal/utils"
	"github.com/Bibekbb/Orchix/pkg/types"
	"gopkg.in/yaml.v2"
)

// HelmProvider implements the Helm deployment provider
type HelmProvider struct {
	client      *HelmClient
	namespace   string
	kubeconfig  string
	context     string
	timeout     time.Duration
	logger      *utils.Logger
	initialized bool
}

// NewHelmProvider creates a new Helm provider
func NewHelmProvider() *HelmProvider {
	return &HelmProvider{
		namespace:  "default",
		timeout:    10 * time.Minute,
		logger:     utils.NewLogger("helm-provider", utils.LevelInfo),
	}
}

// Name returns the provider name
func (p *HelmProvider) Name() string {
	return "helm"
}

// GetMetadata returns provider metadata
func (p *HelmProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "helm",
		Version:     "1.0.0",
		Description: "Helm chart deployment provider for Kubernetes",
		Capabilities: []string{
			"plan",
			"apply",
			"destroy",
			"status",
			"health-check",
			"rollback",
			"template",
			"lint",
		},
		RequiredTools: []string{
			"helm",
			"kubectl",
		},
		SupportedActions: []string{
			"install",
			"upgrade",
			"uninstall",
			"template",
			"lint",
			"rollback",
		},
	}
}

// Plan generates a deployment plan
func (p *HelmProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	result := providers.NewPlanResult()

	p.logger.Info("Planning Helm deployment: %s", comp.Name)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Helm provider: %w", err)
	}

	// Parse chart reference
	chart := p.getChartReference(comp)
	releaseName := p.getReleaseName(comp)

	// Check if release exists
	existingRelease, err := p.client.GetRelease(ctx, releaseName, p.namespace)
	if err != nil {
		// Release doesn't exist - will create
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeCreate,
			Address: fmt.Sprintf("helm.release.%s", releaseName),
			After:   fmt.Sprintf("Helm release %s from chart %s", releaseName, chart),
		})
	} else {
		// Release exists - will upgrade
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeUpdate,
			Address: fmt.Sprintf("helm.release.%s", releaseName),
			Before:  fmt.Sprintf("Release %s (chart: %s, version: %s)", 
				releaseName, existingRelease.Chart, existingRelease.AppVersion),
			After:   fmt.Sprintf("Updated release %s from chart %s", releaseName, chart),
		})
	}

	// Lint chart if it's a local path
	if p.isLocalChart(chart) {
		lintResult, err := p.client.Lint(ctx, chart)
		if err != nil {
			result.AddChange(providers.Change{
				Type:    providers.ChangeTypeNoOp,
				Address: fmt.Sprintf("helm.lint.%s", releaseName),
				After:   fmt.Sprintf("Chart validation failed: %v", err),
			})
		} else if !lintResult.Valid {
			result.AddChange(providers.Change{
				Type:    providers.ChangeTypeNoOp,
				Address: fmt.Sprintf("helm.lint.%s", releaseName),
				After:   fmt.Sprintf("Chart validation warnings: %s", lintResult.Message),
			})
		}
	}

	result.SetOutput("release_name", releaseName)
	result.SetOutput("chart", chart)
	result.SetOutput("namespace", p.namespace)
	result.SetOutput("provider", "helm")

	return result, nil
}

// Apply executes the deployment
func (p *HelmProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()
	result := providers.NewApplyResult()

	p.logger.Info("Deploying with Helm: %s", comp.Name)
	p.logger.Info("Source: %s", comp.Source)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Helm provider: %w", err)
	}

	// Parse chart reference
	chart := p.getChartReference(comp)
	releaseName := p.getReleaseName(comp)

	// Check if release exists
	existingRelease, err := p.client.GetRelease(ctx, releaseName, p.namespace)
	
	// Prepare values files
	valuesFiles, err := p.prepareValuesFiles(comp)
	if err != nil {
		return result, fmt.Errorf("failed to prepare values files: %w", err)
	}

	// Prepare install/upgrade options
	setValues, setStringValues, setFileValues := p.parseSetValues(comp)

	if existingRelease != nil {
		// Upgrade existing release
		p.logger.Info("Upgrading existing release: %s", releaseName)
		
		upgradeOpts := &UpgradeOptions{
			Namespace:       p.namespace,
			ValuesFiles:     valuesFiles,
			SetValues:       setValues,
			SetStringValues: setStringValues,
			SetFileValues:   setFileValues,
			Timeout:         p.timeout,
			Wait:            p.shouldWait(comp),
			Atomic:          p.shouldAtomic(comp),
			Install:         true, // Install if doesn't exist
		}
		
		if version := p.getChartVersion(comp); version != "" {
			upgradeOpts.Version = version
		}
		
		releaseInfo, err := p.client.Upgrade(ctx, releaseName, chart, upgradeOpts)
		if err != nil {
			return result, fmt.Errorf("helm upgrade failed: %w", err)
		}
		
		p.logger.Info("✅ Release upgraded: %s (revision: %d)", releaseName, releaseInfo.Revision)
		
	} else {
		// Install new release
		p.logger.Info("Installing new release: %s", releaseName)
		
		installOpts := &InstallOptions{
			Namespace:       p.namespace,
			ValuesFiles:     valuesFiles,
			SetValues:       setValues,
			SetStringValues: setStringValues,
			SetFileValues:   setFileValues,
			Timeout:         p.timeout,
			Wait:            p.shouldWait(comp),
			Atomic:          p.shouldAtomic(comp),
			CreateNamespace: p.shouldCreateNamespace(comp),
		}
		
		if version := p.getChartVersion(comp); version != "" {
			installOpts.Version = version
		}
		
		releaseInfo, err := p.client.Install(ctx, releaseName, chart, installOpts)
		if err != nil {
			return result, fmt.Errorf("helm install failed: %w", err)
		}
		
		p.logger.Info("✅ Release installed: %s (revision: %d)", releaseName, releaseInfo.Revision)
	}

	// Get final release status
	finalRelease, err := p.client.GetRelease(ctx, releaseName, p.namespace)
	if err != nil {
		return result, fmt.Errorf("failed to get release status: %w", err)
	}

	// Get values
	values, err := p.client.GetValues(ctx, releaseName, p.namespace)
	if err != nil {
		p.logger.Warn("Failed to get release values: %v", err)
	}

	// Set outputs
	result.AddOutput("release_name", finalRelease.Name)
	result.AddOutput("namespace", finalRelease.Namespace)
	result.AddOutput("revision", fmt.Sprintf("%d", finalRelease.Revision))
	result.AddOutput("status", finalRelease.Status)
	result.AddOutput("chart", finalRelease.Chart)
	result.AddOutput("app_version", finalRelease.AppVersion)
	result.AddOutput("updated", finalRelease.Updated)
	
	if len(values) > 0 {
		valuesYAML, _ := yaml.Marshal(values)
		result.AddOutput("values", string(valuesYAML))
	}
	
	result.AddOutput("provider", "helm")
	result.Duration = time.Since(startTime)

	p.logger.Info("Helm deployment completed successfully")
	return result, nil
}

// Destroy removes the deployment
func (p *HelmProvider) Destroy(ctx context.Context, comp types.Component) error {
	p.logger.Info("Destroying Helm release: %s", comp.Name)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Helm provider: %w", err)
	}

	releaseName := p.getReleaseName(comp)

	// Check if release exists
	existingRelease, err := p.client.GetRelease(ctx, releaseName, p.namespace)
	if err != nil {
		p.logger.Warn("Release %s not found or already deleted", releaseName)
		return nil
	}

	p.logger.Info("Uninstalling release: %s (revision: %d)", releaseName, existingRelease.Revision)

	// Uninstall release
	uninstallOpts := &UninstallOptions{
		Namespace: p.namespace,
		Timeout:   p.timeout,
		Wait:      p.shouldWait(comp),
	}

	if err := p.client.Uninstall(ctx, releaseName, uninstallOpts); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}

	p.logger.Info("✅ Release uninstalled: %s", releaseName)
	return nil
}

// Status checks the deployment status
func (p *HelmProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()

	p.logger.Info("Checking Helm release status: %s", comp.Name)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to initialize: %v", err)
		result.Healthy = false
		return result, nil
	}

	releaseName := p.getReleaseName(comp)

	// Get release info
	releaseInfo, err := p.client.GetRelease(ctx, releaseName, p.namespace)
	if err != nil {
		result.Status = "not_found"
		result.Message = fmt.Sprintf("Release %s not found", releaseName)
		result.Healthy = false
		return result, nil
	}

	// Determine health based on status
	healthy := false
	switch strings.ToLower(releaseInfo.Status) {
	case "deployed", "active":
		healthy = true
		result.Status = "deployed"
		result.Message = fmt.Sprintf("Release is deployed (revision: %d)", releaseInfo.Revision)
	case "pending", "pending-install", "pending-upgrade", "pending-rollback":
		result.Status = "pending"
		result.Message = fmt.Sprintf("Release is pending (status: %s)", releaseInfo.Status)
	case "failed", "error":
		result.Status = "failed"
		result.Message = fmt.Sprintf("Release failed (status: %s)", releaseInfo.Status)
	default:
		result.Status = strings.ToLower(releaseInfo.Status)
		result.Message = fmt.Sprintf("Release status: %s", releaseInfo.Status)
	}

	result.Healthy = healthy

	// Add details
	result.AddDetail("release_name", releaseInfo.Name)
	result.AddDetail("namespace", releaseInfo.Namespace)
	result.AddDetail("revision", fmt.Sprintf("%d", releaseInfo.Revision))
	result.AddDetail("chart", releaseInfo.Chart)
	result.AddDetail("app_version", releaseInfo.AppVersion)
	result.AddDetail("updated", releaseInfo.Updated)
	result.AddDetail("description", releaseInfo.Description)

	// Get values
	values, err := p.client.GetValues(ctx, releaseName, p.namespace)
	if err == nil && len(values) > 0 {
		result.AddDetail("values_count", fmt.Sprintf("%d", len(values)))
	}

	return result, nil
}

// HealthCheck performs health verification
func (p *HelmProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
	startTime := time.Now()

	// Check Helm client health
	if err := p.client.CheckHealth(ctx); err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Helm health check failed: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	// Check release status
	status, err := p.Status(ctx, comp)
	if err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Failed to check release status: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	return providers.HealthCheckResult{
		Healthy:     status.Healthy,
		Message:     status.Message,
		Duration:    time.Since(startTime),
		LastChecked: time.Now(),
		Details:     status.Details,
	}, nil
}

// Rollback rolls back to a previous revision
func (p *HelmProvider) Rollback(ctx context.Context, comp types.Component) error {
	p.logger.Info("Rolling back Helm release: %s", comp.Name)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Helm provider: %w", err)
	}

	releaseName := p.getReleaseName(comp)

	// Get revision to rollback to
	revision := 1 // Default to previous revision
	if rev, ok := comp.Variables["revision"].(int); ok {
		revision = rev
	} else if revStr, ok := comp.Variables["revision"].(string); ok {
		if revInt, err := fmt.Sscanf(revStr, "%d", &revision); err != nil || revInt != 1 {
			revision = 1
		}
	}

	// Perform rollback
	if err := p.client.Rollback(ctx, releaseName, revision, p.namespace); err != nil {
		return fmt.Errorf("helm rollback failed: %w", err)
	}

	p.logger.Info("✅ Rolled back %s to revision %d", releaseName, revision)
	return nil
}

// Template renders Helm templates without installing
func (p *HelmProvider) Template(ctx context.Context, comp types.Component) (string, error) {
	p.logger.Info("Rendering Helm templates: %s", comp.Name)

	// Initialize client
	if err := p.initialize(comp); err != nil {
		return "", fmt.Errorf("failed to initialize Helm provider: %w", err