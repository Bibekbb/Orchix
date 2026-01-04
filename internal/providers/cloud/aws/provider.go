package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// AWSProvider implements AWS cloud deployment provider
type AWSProvider struct {
	config    aws.Config
	region    string
	profile   string
	resources map[string]interface{}
	clients   map[string]interface{}
}

// NewAWSProvider creates a new AWS provider
func NewAWSProvider() *AWSProvider {
	return &AWSProvider{
		resources: make(map[string]interface{}),
		clients:   make(map[string]interface{}),
	}
}

// Name returns the provider name
func (p *AWSProvider) Name() string {
	return "aws"
}

// GetMetadata returns provider metadata
func (p *AWSProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "aws",
		Version:     "1.0.0",
		Description: "Amazon Web Services cloud provider",
		Capabilities: []string{
			"plan",
			"apply",
			"destroy",
			"status",
			"health-check",
			"cloudformation",
			"ec2",
			"ecs",
			"eks",
			"s3",
			"rds",
			"iam",
		},
		RequiredTools: []string{
			"aws-cli",
		},
		RequiredPermissions: []string{
			"CloudFormation:*",
			"EC2:*",
			"ECS:*",
			"EKS:*",
			"S3:*",
			"RDS:*",
			"IAM:*",
		},
	}
}

// Plan generates a deployment plan
func (p *AWSProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	fmt.Printf("☁️  Planning AWS deployment: %s\n", comp.Name)

	result := providers.NewPlanResult()

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize AWS: %w", err)
	}

	// Determine resource type and plan accordingly
	resourceType := p.getResourceType(comp)

	switch resourceType {
	case "cloudformation":
		return p.planCloudFormation(ctx, comp)
	case "ec2":
		return p.planEC2(ctx, comp)
	case "ecs":
		return p.planECS(ctx, comp)
	case "eks":
		return p.planEKS(ctx, comp)
	case "s3":
		return p.planS3(ctx, comp)
	case "rds":
		return p.planRDS(ctx, comp)
	case "iam":
		return p.planIAM(ctx, comp)
	default:
		return result, fmt.Errorf("unsupported AWS resource type: %s", resourceType)
	}
}

// Apply executes the deployment
func (p *AWSProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()
	result := providers.NewApplyResult()

	fmt.Printf("☁️  Deploying to AWS: %s\n", comp.Name)

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize AWS: %w", err)
	}

	// Determine resource type and apply accordingly
	resourceType := p.getResourceType(comp)

	var outputs map[string]string
	var err error

	switch resourceType {
	case "cloudformation":
		outputs, err = p.applyCloudFormation(ctx, comp)
	case "ec2":
		outputs, err = p.applyEC2(ctx, comp)
	case "ecs":
		outputs, err = p.applyECS(ctx, comp)
	case "eks":
		outputs, err = p.applyEKS(ctx, comp)
	case "s3":
		outputs, err = p.applyS3(ctx, comp)
	case "rds":
		outputs, err = p.applyRDS(ctx, comp)
	case "iam":
		outputs, err = p.applyIAM(ctx, comp)
	default:
		err = fmt.Errorf("unsupported AWS resource type: %s", resourceType)
	}

	if err != nil {
		return result, fmt.Errorf("AWS deployment failed: %w", err)
	}

	// Add outputs
	for key, value := range outputs {
		result.AddOutput(key, value)
	}

	result.AddOutput("provider", "aws")
	result.AddOutput("region", p.region)
	result.AddOutput("resource_type", resourceType)
	result.AddOutput("deployed_at", time.Now().Format(time.RFC3339))
	result.Duration = time.Since(startTime)

	fmt.Printf("✅ AWS deployment completed: %s\n", comp.Name)
	return result, nil
}

// Destroy removes the deployment
func (p *AWSProvider) Destroy(ctx context.Context, comp types.Component) error {
	fmt.Printf("🗑️  Destroying AWS resources: %s\n", comp.Name)

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize AWS: %w", err)
	}

	// Determine resource type and destroy accordingly
	resourceType := p.getResourceType(comp)

	var err error

	switch resourceType {
	case "cloudformation":
		err = p.destroyCloudFormation(ctx, comp)
	case "ec2":
		err = p.destroyEC2(ctx, comp)
	case "ecs":
		err = p.destroyECS(ctx, comp)
	case "eks":
		err = p.destroyEKS(ctx, comp)
	case "s3":
		err = p.destroyS3(ctx, comp)
	case "rds":
		err = p.destroyRDS(ctx, comp)
	case "iam":
		err = p.destroyIAM(ctx, comp)
	default:
		err = fmt.Errorf("unsupported AWS resource type: %s", resourceType)
	}

	if err != nil {
		return fmt.Errorf("AWS destroy failed: %w", err)
	}

	fmt.Printf("✅ AWS resources destroyed: %s\n", comp.Name)
	return nil
}

// Status checks the deployment status
func (p *AWSProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()

	fmt.Printf("📊 Checking AWS status: %s\n", comp.Name)

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to initialize AWS: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Determine resource type and check status
	resourceType := p.getResourceType(comp)

	var status string
	var message string
	var healthy bool
	var details map[string]string

	switch resourceType {
	case "cloudformation":
		status, message, healthy, details = p.statusCloudFormation(ctx, comp)
	case "ec2":
		status, message, healthy, details = p.statusEC2(ctx, comp)
	case "ecs":
		status, message, healthy, details = p.statusECS(ctx, comp)
	case "eks":
		status, message, healthy, details = p.statusEKS(ctx, comp)
	case "s3":
		status, message, healthy, details = p.statusS3(ctx, comp)
	case "rds":
		status, message, healthy, details = p.statusRDS(ctx, comp)
	case "iam":
		status, message, healthy, details = p.statusIAM(ctx, comp)
	default:
		status = "unknown"
		message = fmt.Sprintf("Unsupported resource type: %s", resourceType)
		healthy = false
	}

	result.Status = status
	result.Message = message
	result.Healthy = healthy
	result.Details = details

	return result, nil
}

// HealthCheck performs health verification
func (p *AWSProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
	startTime := time.Now()

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Failed to initialize AWS: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	// Check AWS service health
	if err := p.checkAWSHealth(ctx); err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("AWS service unhealthy: %v", err),
			Duration:    time.Since(startTime),
			LastChecked: time.Now(),
		}, nil
	}

	// Get resource status
	status, err := p.Status(ctx, comp)
	if err != nil {
		return providers.HealthCheckResult{
			Healthy:     false,
			Message:     fmt.Sprintf("Failed to get status: %v", err),
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

// ValidateConfig validates component configuration
func (p *AWSProvider) ValidateConfig(config map[string]interface{}) error {
	// Check for required fields
	if _, ok := config["resource_type"]; !ok {
		return fmt.Errorf("resource_type is required for AWS components")
	}

	// Validate region if provided
	if region, ok := config["region"].(string); ok && region != "" {
		if !isValidAWSRegion(region) {
			return fmt.Errorf("invalid AWS region: %s", region)
		}
	}

	// Validate credentials
	if profile, ok := config["profile"].(string); ok && profile != "" {
		if !p.checkAWSCredentials(profile) {
			return fmt.Errorf("AWS credentials not found for profile: %s", profile)
		}
	}

	return nil
}

// Rollback rolls back to previous state
func (p *AWSProvider) Rollback(ctx context.Context, comp types.Component) error {
	fmt.Printf("↩️  Rolling back AWS deployment: %s\n", comp.Name)

	// Initialize AWS config
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize AWS: %w", err)
	}

	resourceType := p.getResourceType(comp)

	if resourceType == "cloudformation" {
		return p.rollbackCloudFormation(ctx, comp)
	}

	// For other resource types, we need to implement specific rollback logic
	return fmt.Errorf("rollback not supported for resource type: %s", resourceType)
}

// Helper methods
func (p *AWSProvider) initialize(comp types.Component) error {
	// Get configuration
	p.region = "us-east-1" // Default region
	if region, ok := comp.Variables["region"].(string); ok && region != "" {
		p.region = region
	}

	p.profile = "default"
	if profile, ok := comp.Variables["profile"].(string); ok && profile != "" {
		p.profile = profile
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(p.region),
		config.WithSharedConfigProfile(p.profile),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	p.config = cfg

	// Initialize clients
	p.initClients()

	return nil
}

func (p *AWSProvider) initClients() {
	p.clients["cloudformation"] = cloudformation.NewFromConfig(p.config)
	p.clients["ec2"] = ec2.NewFromConfig(p.config)
	p.clients["ecs"] = ecs.NewFromConfig(p.config)
	p.clients["eks"] = eks.NewFromConfig(p.config)
	p.clients["s3"] = s3.NewFromConfig(p.config)
	p.clients["rds"] = rds.NewFromConfig(p.config)
	p.clients["iam"] = iam.NewFromConfig(p.config)
	p.clients["secretsmanager"] = secretsmanager.NewFromConfig(p.config)
	p.clients["ssm"] = ssm.NewFromConfig(p.config)
}

func (p *AWSProvider) getResourceType(comp types.Component) string {
	if resourceType, ok := comp.Variables["resource_type"].(string); ok {
		return resourceType
	}

	// Try to infer from source path
	if strings.Contains(comp.Source, "cloudformation") {
		return "cloudformation"
	} else if strings.Contains(comp.Source, "ec2") {
		return "ec2"
	} else if strings.Contains(comp.Source, "ecs") {
		return "ecs"
	} else if strings.Contains(comp.Source, "eks") {
		return "eks"
	} else if strings.Contains(comp.Source, "s3") {
		return "s3"
	} else if strings.Contains(comp.Source, "rds") {
		return "rds"
	}

	return "cloudformation" // Default
}

func (p *AWSProvider) checkAWSHealth(ctx context.Context) error {
	// Simple health check by listing regions
	ec2Client := ec2.NewFromConfig(p.config)
	_, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	return err
}

func (p *AWSProvider) checkAWSCredentials(profile string) bool {
	// Check if AWS credentials exist
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check credentials file
	credsPath := fmt.Sprintf("%s/.aws/credentials", home)
	if _, err := os.Stat(credsPath); err == nil {
		// File exists
		return true
	}

	// Check config file
	configPath := fmt.Sprintf("%s/.aws/config", home)
	if _, err := os.Stat(configPath); err == nil {
		// File exists
		return true
	}

	// Check environment variables
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	return false
}

// CloudFormation methods
func (p *AWSProvider) planCloudFormation(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	result := providers.NewPlanResult()

	// Read CloudFormation template
	templateBody, err := os.ReadFile(comp.Source)
	if err != nil {
		return result, fmt.Errorf("failed to read CloudFormation template: %w", err)
	}

	// Get CloudFormation client
	client := p.clients["cloudformation"].(*cloudformation.Client)

	// Validate template
	validateInput := &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String(string(templateBody)),
	}

	_, err = client.ValidateTemplate(ctx, validateInput)
	if err != nil {
		return result, fmt.Errorf("CloudFormation template validation failed: %w", err)
	}

	// Check if stack exists
	stackName := p.getStackName(comp)
	describeInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	_, err = client.DescribeStacks(ctx, describeInput)
	stackExists := err == nil

	// Create plan changes
	if stackExists {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeUpdate,
			Address: fmt.Sprintf("aws.cloudformation.stack.%s", stackName),
			Action:  "update",
			After:   "Update CloudFormation stack",
		})
	} else {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeCreate,
			Address: fmt.Sprintf("aws.cloudformation.stack.%s", stackName),
			Action:  "create",
			After:   "Create CloudFormation stack",
		})
	}

	result.SetOutput("stack_name", stackName)
	result.SetOutput("template_file", comp.Source)
	result.SetOutput("region", p.region)

	return result, nil
}

func (p *AWSProvider) applyCloudFormation(ctx context.Context, comp types.Component) (map[string]string, error) {
	outputs := make(map[string]string)

	// Read CloudFormation template
	templateBody, err := os.ReadFile(comp.Source)
	if err != nil {
		return outputs, fmt.Errorf("failed to read CloudFormation template: %w", err)
	}

	// Get CloudFormation client
	client := p.clients["cloudformation"].(*cloudformation.Client)

	// Get stack name
	stackName := p.getStackName(comp)

	// Check if stack exists
	describeInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	_, err = client.DescribeStacks(ctx, describeInput)
	stackExists := err == nil

	// Prepare parameters
	parameters := p.getCloudFormationParameters(comp)

	if stackExists {
		// Update existing stack
		updateInput := &cloudformation.UpdateStackInput{
			StackName:    aws.String(stackName),
			TemplateBody: aws.String(string(templateBody)),
			Parameters:   parameters,
			Capabilities: []types.Capability{
				types.CapabilityCapabilityIam,
				types.CapabilityCapabilityNamedIam,
				types.CapabilityCapabilityAutoExpand,
			},
		}

		fmt.Printf("  Updating CloudFormation stack: %s\n", stackName)
		_, err = client.UpdateStack(ctx, updateInput)
		if err != nil {
			return outputs, fmt.Errorf("failed to update CloudFormation stack: %w", err)
		}
	} else {
		// Create new stack
		createInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(stackName),
			TemplateBody: aws.String(string(templateBody)),
			Parameters:   parameters,
			Capabilities: []types.Capability{
				types.CapabilityCapabilityIam,
				types.CapabilityCapabilityNamedIam,
				types.CapabilityCapabilityAutoExpand,
			},
			OnFailure: types.OnFailureDelete,
		}

		fmt.Printf("  Creating CloudFormation stack: %s\n", stackName)
		_, err = client.CreateStack(ctx, createInput)
		if err != nil {
			return outputs, fmt.Errorf("failed to create CloudFormation stack: %w", err)
		}
	}

	// Wait for stack to complete
	fmt.Printf("  Waiting for stack to complete...\n")
	err = p.waitForCloudFormationStack(ctx, stackName)
	if err != nil {
		return outputs, fmt.Errorf("stack creation/update failed: %w", err)
	}

	// Get stack outputs
	stackOutputs, err := p.getCloudFormationOutputs(ctx, stackName)
	if err != nil {
		return outputs, fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Merge outputs
	for key, value := range stackOutputs {
		outputs[key] = value
	}

	outputs["stack_name"] = stackName
	outputs["stack_status"] = "CREATE_COMPLETE"

	return outputs, nil
}

func (p *AWSProvider) destroyCloudFormation(ctx context.Context, comp types.Component) error {
	// Get CloudFormation client
	client := p.clients["cloudformation"].(*cloudformation.Client)

	// Get stack name
	stackName := p.getStackName(comp)

	// Check if stack exists
	describeInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	_, err := client.DescribeStacks(ctx, describeInput)
	if err != nil {
		// Stack doesn't exist
		fmt.Printf("  Stack %s doesn't exist\n", stackName)
		return nil
	}

	// Delete stack
	fmt.Printf("  Deleting CloudFormation stack: %s\n", stackName)
	deleteInput := &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	}

	_, err = client.DeleteStack(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete CloudFormation stack: %w", err)
	}

	// Wait for deletion to complete
	fmt.Printf("  Waiting for stack deletion...\n")
	err = p.waitForCloudFormationDeletion(ctx, stackName)
	if err != nil {
		return fmt.Errorf("stack deletion failed: %w", err)
	}

	return nil
}

func (p *AWSProvider) statusCloudFormation(ctx context.Context, comp types.Component) (string, string, bool, map[string]string) {
	details := make(map[string]string)

	// Get CloudFormation client
	client := p.clients["cloudformation"].(*cloudformation.Client)

	// Get stack name
	stackName := p.getStackName(comp)

	// Describe stack
	describeInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	output, err := client.DescribeStacks(ctx, describeInput)
	if err != nil {
		return "not_found", "Stack not found", false, details
	}

	if len(output.Stacks) == 0 {
		return "not_found", "Stack not found", false, details
	}

	stack := output.Stacks[0]
	status := string(stack.StackStatus)
	statusReason := ""
	if stack.StackStatusReason != nil {
		statusReason = *stack.StackStatusReason
	}

	// Determine if stack is healthy
	healthy := false
	switch stack.StackStatus {
	case types.StackStatusCreateComplete,
		types.StackStatusUpdateComplete,
		types.StackStatusUpdateRollbackComplete,
		types.StackStatusImportComplete:
		healthy = true
	}

	// Add details
	details["stack_id"] = *stack.StackId
	details["creation_time"] = stack.CreationTime.Format(time.RFC3339)
	if stack.LastUpdatedTime != nil {
		details["last_updated"] = stack.LastUpdatedTime.Format(time.RFC3339)
	}
	details["stack_status"] = status
	if statusReason != "" {
		details["status_reason"] = statusReason
	}

	// Add outputs
	for _, output := range stack.Outputs {
		if output.OutputKey != nil && output.OutputValue != nil {
			details[fmt.Sprintf("output.%s", *output.OutputKey)] = *output.OutputValue
		}
	}

	return status, statusReason, healthy, details
}

// EC2 methods (simplified for example)
func (p *AWSProvider) planEC2(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	result := providers.NewPlanResult()
	// Implementation for EC2 planning
	return result, nil
}

func (p *AWSProvider) applyEC2(ctx context.Context, comp types.Component) (map[string]string, error) {
	outputs := make(map[string]string)
	// Implementation for EC2 deployment
	return outputs, nil
}

// Utility methods
func (p *AWSProvider) getStackName(comp types.Component) string {
	if stackName, ok := comp.Variables["stack_name"].(string); ok && stackName != "" {
		return stackName
	}
	return fmt.Sprintf("orchix-%s", comp.ID)
}

func (p *AWSProvider) getCloudFormationParameters(comp types.Component) []types.Parameter {
	var parameters []types.Parameter

	if params, ok := comp.Variables["parameters"].(map[string]interface{}); ok {
		for key, value := range params {
			paramValue := fmt.Sprintf("%v", value)
			parameters = append(parameters, types.Parameter{
				ParameterKey:   aws.String(key),
				ParameterValue: aws.String(paramValue),
			})
		}
	}

	return parameters
}

func (p *AWSProvider) waitForCloudFormationStack(ctx context.Context, stackName string) error {
	client := p.clients["cloudformation"].(*cloudformation.Client)

	waiter := cloudformation.NewStackCreateCompleteWaiter(client)
	return waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, 30*time.Minute)
}

func (p *AWSProvider) waitForCloudFormationDeletion(ctx context.Context, stackName string) error {
	client := p.clients["cloudformation"].(*cloudformation.Client)

	waiter := cloudformation.NewStackDeleteCompleteWaiter(client)
	return waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, 30*time.Minute)
}

func (p *AWSProvider) getCloudFormationOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	outputs := make(map[string]string)

	client := p.clients["cloudformation"].(*cloudformation.Client)
	resp, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})

	if err != nil {
		return outputs, err
	}

	if len(resp.Stacks) > 0 && resp.Stacks[0].Outputs != nil {
		for _, output := range resp.Stacks[0].Outputs {
			if output.OutputKey != nil && output.OutputValue != nil {
				outputs[*output.OutputKey] = *output.OutputValue
			}
		}
	}

	return outputs, nil
}

func (p *AWSProvider) rollbackCloudFormation(ctx context.Context, comp types.Component) error {
	client := p.clients["cloudformation"].(*cloudformation.Client)
	stackName := p.getStackName(comp)

	// Get stack events to find previous successful deployment
	eventsInput := &cloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	}

	events, err := client.DescribeStackEvents(ctx, eventsInput)
	if err != nil {
		return fmt.Errorf("failed to get stack events: %w", err)
	}

	// Find previous successful stack set ID for rollback
	// This is simplified - real implementation would need to track change sets
	fmt.Printf("  Initiating CloudFormation rollback for stack: %s\n", stackName)

	// For now, we'll use continue update rollback
	rollbackInput := &cloudformation.ContinueUpdateRollbackInput{
		StackName: aws.String(stackName),
	}

	_, err = client.ContinueUpdateRollback(ctx, rollbackInput)
	if err != nil {
		return fmt.Errorf("failed to rollback CloudFormation stack: %w", err)
	}

	return nil
}

func isValidAWSRegion(region string) bool {
	// List of valid AWS regions
	validRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
		"ap-south-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-southeast-1", "ap-southeast-2", "ca-central-1", "sa-east-1",
	}

	for _, valid := range validRegions {
		if region == valid {
			return true
		}
	}
	return false
}