package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

type TerraformProvider struct {
	workDir string
}

func NewTerraformProvider() *TerraformProvider {
	return &TerraformProvider{}
}

func (p *TerraformProvider) Name() string {
	return "terraform"
}

func (p *TerraformProvider) Apply(ctx context.Context, comp types.Component, variables map[string]string) (*providers.ApplyResult, error) {
	// Create temporary directory for terraform files
	tmpDir, err := os.MkdirTemp("", "Orchix-terraform-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Copy source files to temp directory
	if err := copyFiles(comp.Source, tmpDir); err != nil {
		return nil, err
	}

	// Generate terraform.tfvars from variables
	if err := p.generateTFVars(tmpDir, variables); err != nil {
		return nil, err
	}

	// Initialize terraform
	if err := p.runTerraform(ctx, tmpDir, "init"); err != nil {
		return nil, err
	}

	// Apply terraform
	if err := p.runTerraform(ctx, tmpDir, "apply", "-auto-approve"); err != nil {
		return nil, err
	}

	// Get outputs
	outputs, err := p.getOutputs(ctx, tmpDir)
	if err != nil {
		return nil, err
	}

	return &providers.ApplyResult{
		Outputs: outputs,
	}, nil
}

func (p *TerraformProvider) Plan(ctx context.Context, comp types.Component, variables map[string]string) (*providers.PlanResult, error) {
	// Similar to Apply but run plan instead
	tmpDir, err := os.MkdirTemp("", "Orchix-terraform-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := copyFiles(comp.Source, tmpDir); err != nil {
		return nil, err
	}

	if err := p.generateTFVars(tmpDir, variables); err != nil {
		return nil, err
	}

	if err := p.runTerraform(ctx, tmpDir, "init"); err != nil {
		return nil, err
	}

	// Run plan and parse output
	if err := p.runTerraform(ctx, tmpDir, "plan", "-out=tfplan", "-json"); err != nil {
		return nil, err
	}

	// For simplicity, return empty plan result
	return &providers.PlanResult{
		Changes: []providers.Change{},
		Outputs: map[string]string{},
	}, nil
}

func (p *TerraformProvider) Destroy(ctx context.Context, comp types.Component, variables map[string]string) error {
	tmpDir, err := os.MkdirTemp("", "Orchix-terraform-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := copyFiles(comp.Source, tmpDir); err != nil {
		return err
	}

	if err := p.generateTFVars(tmpDir, variables); err != nil {
		return err
	}

	if err := p.runTerraform(ctx, tmpDir, "init"); err != nil {
		return err
	}

	return p.runTerraform(ctx, tmpDir, "destroy", "-auto-approve")
}

func (p *TerraformProvider) GetStatus(ctx context.Context, comp types.Component) (*providers.ComponentStatus, error) {
	// For terraform, we can check if the state file exists and is valid
	// This is a simplified implementation
	return &providers.ComponentStatus{Healthy: true}, nil
}

func (p *TerraformProvider) runTerraform(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (p *TerraformProvider) getOutputs(ctx context.Context, dir string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "terraform", "output", "-json")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tfOutput map[string]struct {
		Value interface{} `json:"value"`
	}

	if err := json.Unmarshal(output, &tfOutput); err != nil {
		return nil, err
	}

	outputs := make(map[string]string)
	for k, v := range tfOutput {
		switch val := v.Value.(type) {
		case string:
			outputs[k] = val
		default:
			// Convert to JSON string for complex types
			jsonVal, _ := json.Marshal(val)
			outputs[k] = string(jsonVal)
		}
	}

	return outputs, nil
}

func (p *TerraformProvider) generateTFVars(dir string, variables map[string]string) error {
	if len(variables) == 0 {
		return nil
	}

	file, err := os.Create(filepath.Join(dir, "terraform.tfvars"))
	if err != nil {
		return err
	}
	defer file.Close()

	for k, v := range variables {
		fmt.Fprintf(file, "%s = %q\n", k, v)
	}

	return nil
}

func copyFiles(source, dest string) error {
	// Implementation for copying terraform files
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Skip .terraform directory and state files
		if filepath.Base(path) == ".terraform" || filepath.Ext(path) == ".tfstate" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, info.Mode())
	})
}
