package terraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// TerraformProvider implements the Terraform deployment provider
type TerraformProvider struct {
	executor      *TerraformExecutor
	stateManager  *StateManager
	workDir       string
	backendConfig map[string]interface{}
	variables     map[string]interface{}
}

// NewTerraformProvider creates a new Terraform provider
func NewTerraformProvider() *TerraformProvider {
	return &TerraformProvider{}
}

// Name returns the provider name
func (p *TerraformProvider) Name() string {
	return "terraform"
}

// GetMetadata returns provider metadata
func (p *TerraformProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "terraform",
		Version:     "1.0.0",
		Description: "Terraform infrastructure as code provider",
		Capabilities: []string{
			"plan",
			"apply",
			"destroy",
			"status",
			"output",
			"validate",
			"refresh",
		},
		RequiredTools: []string{
			"terraform",
		},
	}
}

// Plan generates a deployment plan
func (p *TerraformProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	fmt.Printf("📋 Planning Terraform deployment: %s\n", comp.Name)

	result := providers.NewPlanResult()

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// Validate configuration first
	validation, err := p.executor.Validate(ctx)
	if err != nil {
		return result, fmt.Errorf("validation failed: %w", err)
	}

	if !validation.Valid {
		for _, diag := range validation.Diagnostics {
			result.AddChange(providers.Change{
				Type:    providers.ChangeTypeNoOp,
				Address: "validation",
				After:   fmt.Sprintf("%s: %s", diag.Severity, diag.Summary),
			})
		}
		return result, fmt.Errorf("Terraform configuration is invalid")
	}

	// Generate plan
	destroy := p.shouldDestroy(comp)
	planResult, err := p.executor.Plan(ctx, destroy)
	if err != nil {
		return result, fmt.Errorf("plan failed: %w", err)
	}

	// Convert to provider changes
	for _, change := range planResult.Changes {
		result.AddChange(change)
	}

	// Add outputs
	for key, value := range planResult.Outputs {
		result.SetOutput(key, value)
	}

	result.SetOutput("provider", "terraform")
	result.SetOutput("work_dir", p.workDir)
	result.SetOutput("destroy", fmt.Sprintf("%v", destroy))

	return result, nil
}

// Apply executes the deployment
func (p *TerraformProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()

	fmt.Printf("🛠️  Applying Terraform: %s\n", comp.Name)
	fmt.Printf("  Source: %s\n", comp.Source)

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return providers.ApplyResult{}, fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// Lock state
	lockInfo := &LockInfo{
		Operation: "apply",
		Info:      fmt.Sprintf("Applying %s", comp.Name),
		Who:       "orchix",
		Version:   "1.0.0",
		Path:      p.workDir,
	}

	if err := p.stateManager.LockState(ctx, lockInfo); err != nil {
		return providers.ApplyResult{}, fmt.Errorf("failed to lock state: %w", err)
	}

	defer p.stateManager.UnlockState(ctx, lockInfo.ID)

	// Initialize Terraform
	if err := p.executor.Init(ctx, p.shouldUpgrade(comp)); err != nil {
		return providers.ApplyResult{}, fmt.Errorf("terraform init failed: %w", err)
	}

	// Apply changes
	autoApprove := p.shouldAutoApprove(comp)
	applyResult, err := p.executor.Apply(ctx, autoApprove)
	if err != nil {
		return providers.ApplyResult{}, fmt.Errorf("terraform apply failed: %w", err)
	}

	// Convert outputs
	result := providers.NewApplyResult()
	for key, value := range applyResult.Outputs {
		var outputStr string
		if str, ok := value.Value.(string); ok {
			outputStr = str
		} else {
			// Convert to JSON for complex types
			jsonBytes, _ := json.Marshal(value.Value)
			outputStr = string(jsonBytes)
		}
		result.AddOutput(key, outputStr)
		
		if value.Sensitive {
			result.AddOutput(key+".sensitive", "true")
		}
	}

	result.AddOutput("provider", "terraform")
	result.AddOutput("work_dir", p.workDir)
	result.AddOutput("applied_at", applyResult.AppliedAt.Format(time.RFC3339))
	result.Duration = applyResult.Duration

	fmt.Println("✅ Terraform apply completed")
	return result, nil
}

// Destroy removes the infrastructure
func (p *TerraformProvider) Destroy(ctx context.Context, comp types.Component) error {
	fmt.Printf("🗑️  Destroying Terraform infrastructure: %s\n", comp.Name)

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// Lock state
	lockInfo := &LockInfo{
		Operation: "destroy",
		Info:      fmt.Sprintf("Destroying %s", comp.Name),
		Who:       "orchix",
		Version:   "1.0.0",
		Path:      p.workDir,
	}

	if err := p.stateManager.LockState(ctx, lockInfo); err != nil {
		return fmt.Errorf("failed to lock state: %w", err)
	}

	defer p.stateManager.UnlockState(ctx, lockInfo.ID)

	// Initialize Terraform
	if err := p.executor.Init(ctx, false); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Destroy infrastructure
	autoApprove := p.shouldAutoApprove(comp)
	if err := p.executor.Destroy(ctx, autoApprove); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	// Clean up state if requested
	if p.shouldCleanState(comp) {
		if err := p.cleanupState(); err != nil {
			fmt.Printf("Warning: failed to cleanup state: %v\n", err)
		}
	}

	fmt.Println("✅ Terraform destroy completed")
	return nil
}

// Status checks the infrastructure status
func (p *TerraformProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()

	fmt.Printf("📊 Checking Terraform status: %s\n", comp.Name)

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to initialize: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Get Terraform version
	version, err := p.executor.Version(ctx)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to get Terraform version: %v", err)
		result.Healthy = false
		return result, nil
	}

	result.AddDetail("terraform_version", version)

	// Validate configuration
	validation, err := p.executor.Validate(ctx)
	if err != nil {
		result.Status = "invalid"
		result.Message = fmt.Sprintf("Validation failed: %v", err)
		result.Healthy = false
		return result, nil
	}

	if !validation.Valid {
		result.Status = "invalid"
		result.Message = "Configuration validation failed"
		result.Healthy = false
		
		for _, diag := range validation.Diagnostics {
			result.AddDetail(diag.Severity, diag.Summary)
		}
		
		return result, nil
	}

	// Get state resources
	resources, err := p.executor.StateList(ctx)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to list state resources: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Get outputs
	outputs, err := p.executor.Output(ctx)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to get outputs: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Determine status
	if len(resources) == 0 {
		result.Status = "empty"
		result.Message = "No resources in state"
		result.Healthy = true
	} else {
		result.Status = "managed"
		result.Message = fmt.Sprintf("Managing %d resource(s)", len(resources))
		result.Healthy = true
	}

	// Add details
	result.AddDetail("resource_count", fmt.Sprintf("%d", len(resources)))
	result.AddDetail("output_count", fmt.Sprintf("%d", len(outputs)))
	result.AddDetail("work_dir", p.workDir)

	// List resources
	for i, resource := range resources {
		if i < 10 { // Limit to first 10 resources
			result.AddDetail(fmt.Sprintf("resource_%d", i), resource.Address)
		}
	}

	// List outputs
	for key, value := range outputs {
		if !value.Sensitive {
			var valStr string
			if str, ok := value.Value.(string); ok {
				valStr = str
			} else {
				jsonBytes, _ := json.Marshal(value.Value)
				valStr = string(jsonBytes)
			}
			result.AddDetail(fmt.Sprintf("output.%s", key), valStr)
		}
	}

	return result, nil
}

// HealthCheck performs health verification
func (p *TerraformProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
	startTime := time.Now()

	status, err := p.Status(ctx, comp)
	if err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Health check failed: %v", err),
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

// Output gets Terraform outputs
func (p *TerraformProvider) Output(ctx context.Context, comp types.Component) (map[string]string, error) {
	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return nil, fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	outputs, err := p.executor.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}

	result := make(map[string]string)
	for key, value := range outputs {
		var valStr string
		if str, ok := value.Value.(string); ok {
			valStr = str
		} else {
			jsonBytes, _ := json.Marshal(value.Value)
			valStr = string(jsonBytes)
		}
		result[key] = valStr
		
		if value.Sensitive {
			result[key+".sensitive"] = "true"
		}
	}

	return result, nil
}

// Validate validates Terraform configuration
func (p *TerraformProvider) Validate(ctx context.Context, comp types.Component) error {
	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	validation, err := p.executor.Validate(ctx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !validation.Valid {
		var errors []string
		for _, diag := range validation.Diagnostics {
			if diag.Severity == "error" {
				errors = append(errors, diag.Summary)
			}
		}
		return fmt.Errorf("Terraform configuration invalid: %s", strings.Join(errors, "; "))
	}

	return nil
}

// Refresh refreshes Terraform state
func (p *TerraformProvider) Refresh(ctx context.Context, comp types.Component) error {
	fmt.Printf("🔄 Refreshing Terraform state: %s\n", comp.Name)

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// Initialize Terraform
	if err := p.executor.Init(ctx, false); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Refresh state
	if err := p.executor.Refresh(ctx); err != nil {
		return fmt.Errorf("terraform refresh failed: %w", err)
	}

	fmt.Println("✅ Terraform refresh completed")
	return nil
}

// Rollback rolls back to previous state
func (p *TerraformProvider) Rollback(ctx context.Context, comp types.Component) error {
	fmt.Printf("↩️  Rolling back Terraform: %s\n", comp.Name)

	// Initialize provider
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// List backups
	backups, err := p.stateManager.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		return fmt.Errorf("no backups available for rollback")
	}

	// Get latest backup
	latestBackup := backups[0]
	for _, backup := range backups[1:] {
		// Simple comparison based on timestamp in filename
		if backup > latestBackup {
			latestBackup = backup
		}
	}

	// Restore from backup
	if err := p.stateManager.RestoreState(latestBackup); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	fmt.Printf("✅ Restored from backup: %s\n", filepath.Base(latestBackup))
	return nil
}

// ValidateConfig validates component configuration
func (p *TerraformProvider) ValidateConfig(config map[string]interface{}) error {
	// Check for required fields
	if source, ok := config["source"].(string); !ok || source == "" {
		return fmt.Errorf("source is required for Terraform components")
	}

	// Check if source exists
	if source, ok := config["source"].(string); ok {
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("source path does not exist: %s", source)
		}
	}

	return nil
}

// Helper methods
func (p *TerraformProvider) initialize(comp types.Component) error {
	// Set work directory
	p.workDir = comp.Source

	// Parse variables
	p.variables = comp.Variables

	// Parse backend configuration
	if backend, ok := comp.Variables["backend"].(map[string]interface{}); ok {
		p.backendConfig = backend
	}

	// Create executor
	p.executor = NewTerraformExecutor(p.workDir)

	// Set timeout if specified
	if timeout, ok := comp.Variables["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeout); err == nil {
			p.executor.SetTimeout(duration)
		}
	}

	// Set backend file if specified
	if backendFile, ok := comp.Variables["backend_file"].(string); ok {
		p.executor.SetBackendFile(backendFile)
	}

	// Set state file if specified
	if stateFile, ok := comp.Variables["state_file"].(string); ok {
		p.executor.SetStateFile(stateFile)
	}

	// Create variables file
	if len(comp.Variables) > 0 {
		varsFile, err := p.createVariablesFile(comp.Variables)
		if err != nil {
			return fmt.Errorf("failed to create variables file: %w", err)
		}
		p.executor.SetVarsFile(varsFile)
	}

	// Create state manager
	p.stateManager = NewStateManager(p.workDir)

	return nil
}

func (p *TerraformProvider) createVariablesFile(variables map[string]interface{}) (string, error) {
	// Filter out special variables
	tfVars := make(map[string]interface{})
	for key, value := range variables {
		if !isSpecialVariable(key) {
			tfVars[key] = value
		}
	}

	if len(tfVars) == 0 {
		return "", nil
	}

	// Create .tfvars file
	varsFile := filepath.Join(p.workDir, "orchix.auto.tfvars.json")
	
	data, err := json.MarshalIndent(tfVars, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal variables: %w", err)
	}

	if err := os.WriteFile(varsFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write variables file: %w", err)
	}

	return varsFile, nil
}

func (p *TerraformProvider) shouldDestroy(comp types.Component) bool {
	if destroy, ok := comp.Variables["destroy"].(string); ok {
		return strings.ToLower(destroy) == "true"
	}
	if destroy, ok := comp.Variables["destroy"].(bool); ok {
		return destroy
	}
	return false
}

func (p *TerraformProvider) shouldAutoApprove(comp types.Component) bool {
	if autoApprove, ok := comp.Variables["auto_approve"].(string); ok {
		return strings.ToLower(autoApprove) == "true"
	}
	if autoApprove, ok := comp.Variables["auto_approve"].(bool); ok {
		return autoApprove
	}
	return true // Default to auto-approve for automation
}

func (p *TerraformProvider) shouldUpgrade(comp types.Component) bool {
	if upgrade, ok := comp.Variables["upgrade"].(string); ok {
		return strings.ToLower(upgrade) == "true"
	}
	if upgrade, ok := comp.Variables["upgrade"].(bool); ok {
		return upgrade
	}
	return false
}

func (p *TerraformProvider) shouldCleanState(comp types.Component) bool {
	if cleanState, ok := comp.Variables["clean_state"].(string); ok {
		return strings.ToLower(cleanState) == "true"
	}
	if cleanState, ok := comp.Variables["clean_state"].(bool); ok {
		return cleanState
	}
	return false
}

func (p *TerraformProvider) cleanupState() error {
	stateFiles := []string{
		"terraform.tfstate",
		"terraform.tfstate.backup",
		".terraform.lock.hcl",
		"orchix.auto.tfvars.json",
		"tfplan",
	}

	for _, file := range stateFiles {
		path := filepath.Join(p.workDir, file)
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove %s: %w", file, err)
			}
		}
	}

	// Remove .terraform directory
	terraformDir := filepath.Join(p.workDir, ".terraform")
	if _, err := os.Stat(terraformDir); err == nil {
		if err := os.RemoveAll(terraformDir); err != nil {
			return fmt.Errorf("failed to remove .terraform directory: %w", err)
		}
	}

	return nil
}

func isSpecialVariable(key string) bool {
	specialVars := []string{
		"backend",
		"backend_file",
		"state_file",
		"timeout",
		"auto_approve",
		"upgrade",
		"destroy",
		"clean_state",
	}

	for _, special := range specialVars {
		if key == special {
			return true
		}
	}

	return false
}