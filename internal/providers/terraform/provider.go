package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"orchestr8/pkg/types"
)

type TerraformProvider struct {
	workDir string 
}

func NewTerraformProvider() *TerraformProvider {
	return &TerraformProvider{}
}

func (p *TerraformProvidet) Name() string {
	return "terraform"
}

func (p *TerraformProvider) Apply(ctx context.Context, comp types.Component, variables map[string]string) (*providers.ApplyResult, error) {
	// Create temporary directory for Terraform files
	tmpDir, err := os.MkdirTemp("", "orchestr8-terraform-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// Copy source files to temp directory
	if err := copyFiles(comp.Source, tempDir); err != nil {
		return nil, err
	}

	// Generate terraform.tfvars from variables
	if err := p.generateTFVars(tempDir, variables); err != nil {
		return nil, err
	}

	// Initalize terraform 
	if err := p.runTerraform(ctx, tempDir, "init"); err != nil {
		return nil, err
	}

	// Apply terraform 
	if err := p.runTerraform(ctx, tempDir, "apply", "-auto-approve"); err != nil {
		return nil, err
	}

	// Get Outputs
	outputs, err := p.getOutputs(ctx, tempDir)
	if err != nil {
		return nil, err
	}

	return &providers.ApplyResult{
		Outputs: outputs,
	}, nil 
}

func (p *TerraformProvider) runTerraform(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "terraform", agrs...)
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
	for k,v := range tfOutput {
		switch val := v.Value.(type) {
		case string:
			outputs[k] = val
		default:
			// Conver to json string for complex types
			jsonVal, _ := json.Marshal(val)
			outputs[k] = string(jsonVal)
		}
	}

	return outputs, nil
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

		destPath := filepath.join(dest, relPath)
	
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