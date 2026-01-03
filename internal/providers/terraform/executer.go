package terraform

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
	"github.com/Bibekbb/Orchix/pkg/types"
)

// TerraformExecutor executes Terraform commands
type TerraformExecutor struct {
	workDir     string
	backendFile string
	stateFile   string
	varsFile    string
	timeout     time.Duration
}

// NewTerraformExecutor creates a new Terraform executor
func NewTerraformExecutor(workDir string) *TerraformExecutor {
	return &TerraformExecutor{
		workDir: workDir,
		timeout: 10 * time.Minute,
	}
}

// SetTimeout sets the command timeout
func (e *TerraformExecutor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// SetBackendFile sets the backend configuration file
func (e *TerraformExecutor) SetBackendFile(file string) {
	e.backendFile = file
}

// SetStateFile sets the state file path
func (e *TerraformExecutor) SetStateFile(file string) {
	e.stateFile = file
}

// SetVarsFile sets the variables file path
func (e *TerraformExecutor) SetVarsFile(file string) {
	e.varsFile = file
}

// Init initializes Terraform
func (e *TerraformExecutor) Init(ctx context.Context, upgrade bool) error {
	args := []string{"init"}
	
	if upgrade {
		args = append(args, "-upgrade")
	}
	
	if e.backendFile != "" {
		args = append(args, "-backend-config", e.backendFile)
	}
	
	return e.runTerraform(ctx, args...)
}

// Plan generates a Terraform plan
func (e *TerraformExecutor) Plan(ctx context.Context, destroy bool) (*PlanResult, error) {
	args := []string{"plan", "-detailed-exitcode", "-out", "tfplan"}
	
	if destroy {
		args = append(args, "-destroy")
	}
	
	// Add variable files
	if e.varsFile != "" {
		args = append(args, "-var-file", e.varsFile)
	}
	
	// Add state file
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	// Run plan
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		// Check exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode == 0 || exitCode == 2 {
				// Exit code 0: no changes
				// Exit code 2: changes detected
				return e.parsePlanOutput(stdout.String(), stderr.String(), destroy)
			}
		}
		return nil, fmt.Errorf("terraform plan failed: %w\nStderr: %s", err, stderr.String())
	}
	
	return e.parsePlanOutput(stdout.String(), stderr.String(), destroy)
}

// Apply applies the Terraform plan
func (e *TerraformExecutor) Apply(ctx context.Context, autoApprove bool) (*ApplyResult, error) {
	args := []string{"apply"}
	
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	
	// Use plan file if it exists
	planFile := filepath.Join(e.workDir, "tfplan")
	if _, err := os.Stat(planFile); err == nil {
		args = append(args, "tfplan")
	}
	
	// Add variable files
	if e.varsFile != "" {
		args = append(args, "-var-file", e.varsFile)
	}
	
	// Add state file
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	// Run apply
	var stdout, stderr bytes.Buffer
	startTime := time.Now()
	
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w\nStderr: %s", err, stderr.String())
	}
	
	// Parse outputs
	outputs, err := e.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}
	
	return &ApplyResult{
		Outputs:    outputs,
		Duration:   time.Since(startTime),
		AppliedAt:  time.Now(),
		Logs:       stdout.String(),
		ErrorLogs:  stderr.String(),
	}, nil
}

// Destroy destroys Terraform resources
func (e *TerraformExecutor) Destroy(ctx context.Context, autoApprove bool) error {
	args := []string{"destroy"}
	
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	
	// Add variable files
	if e.varsFile != "" {
		args = append(args, "-var-file", e.varsFile)
	}
	
	// Add state file
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	return e.runTerraform(ctx, args...)
}

// Output gets Terraform outputs
func (e *TerraformExecutor) Output(ctx context.Context) (map[string]OutputValue, error) {
	args := []string{"output", "-json"}
	
	// Add state file
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("terraform output failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var output map[string]struct {
		Value     interface{} `json:"value"`
		Sensitive bool        `json:"sensitive"`
		Type      interface{} `json:"type"`
	}
	
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}
	
	result := make(map[string]OutputValue)
	for key, val := range output {
		result[key] = OutputValue{
			Value:     val.Value,
			Sensitive: val.Sensitive,
			Type:      val.Type,
		}
	}
	
	return result, nil
}

// Validate validates Terraform configuration
func (e *TerraformExecutor) Validate(ctx context.Context) (*ValidationResult, error) {
	args := []string{"validate", "-json"}
	
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("terraform validate failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var result ValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse validation result: %w", err)
	}
	
	return &result, nil
}

// Show shows the Terraform plan
func (e *TerraformExecutor) Show(ctx context.Context, planFile string) (*ShowResult, error) {
	args := []string{"show", "-json"}
	
	if planFile != "" {
		args = append(args, planFile)
	} else if e.stateFile != "" {
		args = append(args, e.stateFile)
	}
	
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("terraform show failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var result ShowResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse show result: %w", err)
	}
	
	return &result, nil
}

// StateList lists resources in state
func (e *TerraformExecutor) StateList(ctx context.Context) ([]StateResource, error) {
	args := []string{"state", "list"}
	
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return nil, fmt.Errorf("terraform state list failed: %w\nStderr: %s", err, stderr.String())
	}
	
	var resources []StateResource
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		resources = append(resources, StateResource{
			Address: line,
		})
	}
	
	return resources, nil
}

// Refresh refreshes Terraform state
func (e *TerraformExecutor) Refresh(ctx context.Context) error {
	args := []string{"refresh"}
	
	// Add variable files
	if e.varsFile != "" {
		args = append(args, "-var-file", e.varsFile)
	}
	
	// Add state file
	if e.stateFile != "" {
		args = append(args, "-state", e.stateFile)
	}
	
	return e.runTerraform(ctx, args...)
}

// Version gets Terraform version
func (e *TerraformExecutor) Version(ctx context.Context) (string, error) {
	args := []string{"version", "-json"}
	
	var stdout, stderr bytes.Buffer
	if err := e.runTerraformWithOutput(ctx, &stdout, &stderr, args...); err != nil {
		return "", fmt.Errorf("terraform version failed: %w", err)
	}
	
	var versionInfo struct {
		TerraformVersion string `json:"terraform_version"`
	}
	
	if err := json.Unmarshal(stdout.Bytes(), &versionInfo); err != nil {
		return "", fmt.Errorf("failed to parse version info: %w", err)
	}
	
	return versionInfo.TerraformVersion, nil
}

// Helper methods
func (e *TerraformExecutor) runTerraform(ctx context.Context, args ...string) error {
	var stdout, stderr bytes.Buffer
	return e.runTerraformWithOutput(ctx, &stdout, &stderr, args...)
}

func (e *TerraformExecutor) runTerraformWithOutput(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = e.workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=true")
	
	return cmd.Run()
}

func (e *TerraformExecutor) parsePlanOutput(stdout, stderr string, destroy bool) (*PlanResult, error) {
	// Try to parse JSON output first
	if strings.Contains(stdout, "{") && strings.Contains(stdout, "}") {
		// Look for JSON in output
		start := strings.Index(stdout, "{")
		end := strings.LastIndex(stdout, "}") + 1
		if start >= 0 && end > start {
			jsonStr := stdout[start:end]
			var planJSON struct {
				ResourceChanges []struct {
					Address      string `json:"address"`
					Change       struct {
						Actions []string    `json:"actions"`
						Before  interface{} `json:"before"`
						After   interface{} `json:"after"`
					} `json:"change"`
				} `json:"resource_changes"`
			}
			
			if err := json.Unmarshal([]byte(jsonStr), &planJSON); err == nil {
				result := &PlanResult{
					Changes: make([]providers.Change, 0),
				}
				
				for _, rc := range planJSON.ResourceChanges {
					changeType := providers.ChangeTypeNoOp
					if len(rc.Change.Actions) > 0 {
						switch rc.Change.Actions[0] {
						case "create":
							changeType = providers.ChangeTypeCreate
						case "update":
							changeType = providers.ChangeTypeUpdate
						case "delete":
							changeType = providers.ChangeTypeDelete
						}
					}
					
					result.Changes = append(result.Changes, providers.Change{
						Type:    changeType,
						Address: rc.Address,
						Before:  rc.Change.Before,
						After:   rc.Change.After,
					})
				}
				
				return result, nil
			}
		}
	}
	
	// Fall back to parsing text output
	result := &PlanResult{
		Changes: make([]providers.Change, 0),
	}
	
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Parse resource changes from text output
		if strings.Contains(line, "Terraform will perform the following actions:") {
			continue
		}
		
		// Look for resource lines
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "~") || strings.HasPrefix(line, "-") {
			changeType := providers.ChangeTypeNoOp
			address := strings.TrimSpace(strings.TrimPrefix(line, "+"))
			address = strings.TrimSpace(strings.TrimPrefix(address, "~"))
			address = strings.TrimSpace(strings.TrimPrefix(address, "-"))
			
			if strings.HasPrefix(line, "+") {
				changeType = providers.ChangeTypeCreate
			} else if strings.HasPrefix(line, "~") {
				changeType = providers.ChangeTypeUpdate
			} else if strings.HasPrefix(line, "-") {
				changeType = providers.ChangeTypeDelete
			}
			
			result.Changes = append(result.Changes, providers.Change{
				Type:    changeType,
				Address: address,
			})
		}
	}
	
	return result, nil
}

// Types
type PlanResult struct {
	Changes []providers.Change `json:"changes"`
	Outputs map[string]string  `json:"outputs,omitempty"`
}

type ApplyResult struct {
	Outputs    map[string]OutputValue `json:"outputs"`
	Duration   time.Duration          `json:"duration"`
	AppliedAt  time.Time              `json:"appliedAt"`
	Logs       string                 `json:"logs,omitempty"`
	ErrorLogs  string                 `json:"errorLogs,omitempty"`
}

type OutputValue struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive"`
	Type      interface{} `json:"type,omitempty"`
}

type ValidationResult struct {
	Valid        bool     `json:"valid"`
	ErrorCount   int      `json:"error_count"`
	WarningCount int      `json:"warning_count"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Range    *Range `json:"range,omitempty"`
}

type Range struct {
	Filename string `json:"filename"`
	Start    Position `json:"start"`
	End      Position `json:"end"`
}

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Byte   int `json:"byte"`
}

type ShowResult struct {
	FormatVersion string `json:"format_version"`
	// Add more fields as needed based on terraform show -json output
}

type StateResource struct {
	Address string `json:"address"`
	Type    string `json:"type,omitempty"`
	Name    string `json:"name,omitempty"`
}