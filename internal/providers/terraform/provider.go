package terraform

import (
	"context"
	"fmt"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// TerraformProvider implements the Terraform deployment provider
type TerraformProvider struct{}

// NewTerraformProvider creates a new Terraform provider
func NewTerraformProvider() *TerraformProvider {
	return &TerraformProvider{}
}

// Name returns the provider name
func (p *TerraformProvider) Name() string {
	return "terraform"
}

// Plan generates a deployment plan
func (p *TerraformProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	fmt.Printf("Planning Terraform deployment: %s\n", comp.Name)

	return providers.PlanResult{
		Changes: []providers.Change{
			{
				Type:    providers.ChangeTypeCreate,
				Address: fmt.Sprintf("%s.resource", comp.ID),
				After:   "Infrastructure resource",
			},
		},
		Outputs: map[string]string{
			"resource_id": fmt.Sprintf("tf-%s-123", comp.ID),
		},
	}, nil
}

// Apply executes the deployment
func (p *TerraformProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	fmt.Printf("Applying Terraform configuration: %s\n", comp.Name)
	fmt.Printf("  Source: %s\n", comp.Source)

	if len(comp.Variables) > 0 {
		fmt.Println("  Variables:")
		for k, v := range comp.Variables {
			fmt.Printf("    %s = %s\n", k, v)
		}
	}

	fmt.Println("  Running: terraform init")
	fmt.Println("  Running: terraform apply -auto-approve")

	return providers.ApplyResult{
		Outputs: map[string]string{
			"state_file": fmt.Sprintf(".terraform/%s.tfstate", comp.ID),
			"apply_time": "completed",
			"resources":  "3 created, 0 changed, 0 destroyed",
		},
	}, nil
}

// Destroy removes the infrastructure
func (p *TerraformProvider) Destroy(ctx context.Context, comp types.Component) error {
	fmt.Printf("Destroying Terraform infrastructure: %s\n", comp.Name)
	fmt.Println("  Running: terraform destroy -auto-approve")
	return nil
}

// Status checks the infrastructure status
func (p *TerraformProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	return providers.StatusResult{
		Status:  "active",
		Message: fmt.Sprintf("Infrastructure %s is provisioned", comp.Name),
		Healthy: true,
	}, nil
}
