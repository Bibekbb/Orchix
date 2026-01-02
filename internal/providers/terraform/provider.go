package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// KubernetesProvider implements the Kubernetes deployment provider
type KubernetesProvider struct {
	client    *KubernetesClient
	processor *ManifestProcessor
	namespace string
}

// NewKubernetesProvider creates a new Kubernetes provider
func NewKubernetesProvider() *KubernetesProvider {
	return &KubernetesProvider{
		namespace: "default",
	}
}

// Name returns the provider name
func (p *KubernetesProvider) Name() string {
	return "kubernetes"
}

// GetMetadata returns provider metadata
func (p *KubernetesProvider) GetMetadata() providers.ProviderMetadata {
	return providers.ProviderMetadata{
		Name:        "kubernetes",
		Version:     "1.0.0",
		Description: "Kubernetes deployment provider",
		Capabilities: []string{
			"plan",
			"apply",
			"destroy",
			"status",
			"health-check",
			"rollback",
		},
		RequiredTools: []string{
			"kubectl",
		},
	}
}

// Plan generates a deployment plan
func (p *KubernetesProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	fmt.Printf("📋 Planning Kubernetes deployment: %s\n", comp.Name)

	result := providers.NewPlanResult()

	// Initialize client if not already done
	if err := p.initializeClient(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	// Check Kubernetes connectivity
	if err := p.client.CheckHealth(ctx); err != nil {
		return result, fmt.Errorf("Kubernetes cluster not reachable: %w", err)
	}

	// Process manifests
	manifests, err := p.processManifests(comp)
	if err != nil {
		return result, fmt.Errorf("failed to process manifests: %w", err)
	}

	// Check existing resources and plan changes
	for _, manifest := range manifests {
		kind := manifest.GetKind()
		name := manifest.GetName()
		namespace := manifest.GetNamespace()
		if namespace == "" {
			namespace = p.namespace
		}

		// Determine resource action
		gvr := schema.GroupVersionResource{
			Group:    manifest.GroupVersionKind().Group,
			Version:  manifest.GroupVersionKind().Version,
			Resource: p.client.resourceForKind(kind),
		}

		// Check if resource exists
		existing, err := p.client.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		
		changeType := providers.ChangeTypeCreate
		if err == nil {
			changeType = providers.ChangeTypeUpdate
		}

		result.AddChange(providers.Change{
			Type:    changeType,
			Address: fmt.Sprintf("kubernetes.%s.%s", strings.ToLower(kind), name),
			After:   fmt.Sprintf("%s %s/%s", kind, namespace, name),
		})

		result.SetOutput(fmt.Sprintf("%s.name", strings.ToLower(kind)), name)
		result.SetOutput(fmt.Sprintf("%s.namespace", strings.ToLower(kind)), namespace)
	}

	result.SetOutput("provider", "kubernetes")
	result.SetOutput("namespace", p.namespace)
	result.SetOutput("manifest_count", fmt.Sprintf("%d", len(manifests)))

	return result, nil
}

// Apply executes the deployment
func (p *KubernetesProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()
	result := providers.NewApplyResult()

	fmt.Printf("☸️  Deploying to Kubernetes: %s\n", comp.Name)
	fmt.Printf("  Source: %s\n", comp.Source)

	// Initialize client
	if err := p.initializeClient(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	// Check Kubernetes connectivity
	if err := p.client.CheckHealth(ctx); err != nil {
		return result, fmt.Errorf("Kubernetes cluster not reachable: %w", err)
	}

	// Create namespace if it doesn't exist
	if err := p.ensureNamespace(ctx); err != nil {
		return result, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Apply manifests
	appliedResources, err := p.client.ApplyManifest(ctx, comp.Source)
	if err != nil {
		return result, fmt.Errorf("failed to apply manifests: %w", err)
	}

	// Wait for deployments to be ready
	if err := p.waitForDeployments(ctx, appliedResources); err != nil {
		return result, fmt.Errorf("failed waiting for deployments: %w", err)
	}

	// Set outputs
	for _, resource := range appliedResources {
		result.AddOutput(fmt.Sprintf("%s.%s", strings.ToLower(resource.Kind), resource.Name), 
			fmt.Sprintf("%s/%s", resource.Namespace, resource.Name))
	}

	result.AddOutput("provider", "kubernetes")
	result.AddOutput("namespace", p.namespace)
	result.AddOutput("status", "deployed")
	result.AddOutput("applied_resources", fmt.Sprintf("%d", len(appliedResources)))
	result.Duration = time.Since(startTime)

	fmt.Println("✅ Kubernetes deployment completed")
	return result, nil
}

// Destroy removes the deployment
func (p *KubernetesProvider) Destroy(ctx context.Context, comp types.Component) error {
	fmt.Printf("🗑️  Destroying Kubernetes deployment: %s\n", comp.Name)

	// Initialize client
	if err := p.initializeClient(comp); err != nil {
		return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	// Delete manifests
	if err := p.client.DeleteManifest(ctx, comp.Source); err != nil {
		return fmt.Errorf("failed to delete manifests: %w", err)
	}

	// Optionally delete namespace
	if p.shouldDeleteNamespace(comp) {
		fmt.Printf("  Deleting namespace: %s\n", p.namespace)
		if err := p.client.DeleteNamespace(ctx, p.namespace); err != nil {
			fmt.Printf("  Warning: Failed to delete namespace: %v\n", err)
		}
	}

	fmt.Println("✅ Kubernetes resources destroyed")
	return nil
}

// Status checks the deployment status
func (p *KubernetesProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()

	fmt.Printf("📊 Checking Kubernetes status: %s\n", comp.Name)

	// Initialize client
	if err := p.initializeClient(comp); err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to initialize client: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Check cluster health
	if err := p.client.CheckHealth(ctx); err != nil {
		result.Status = "unreachable"
		result.Message = fmt.Sprintf("Kubernetes cluster not reachable: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Get cluster info
	clusterInfo, err := p.client.GetClusterInfo(ctx)
	if err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to get cluster info: %v", err)
		result.Healthy = false
		return result, nil
	}

	result.AddDetail("cluster_version", clusterInfo.Version)
	result.AddDetail("node_count", fmt.Sprintf("%d", clusterInfo.NodeCount))

	// Process manifests to know what to check
	manifests, err := p.processManifests(comp)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to process manifests: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Check each resource
	allHealthy := true
	var messages []string

	for _, manifest := range manifests {
		kind := manifest.GetKind()
		name := manifest.GetName()
		namespace := manifest.GetNamespace()
		if namespace == "" {
			namespace = p.namespace
		}

		resourceStatus, err := p.checkResourceStatus(ctx, kind, namespace, name)
		if err != nil {
			messages = append(messages, fmt.Sprintf("%s/%s: error (%v)", kind, name, err))
			allHealthy = false
			continue
		}

		result.AddDetail(fmt.Sprintf("%s.%s.status", strings.ToLower(kind), name), resourceStatus.Status)
		result.AddDetail(fmt.Sprintf("%s.%s.message", strings.ToLower(kind), name), resourceStatus.Message)

		if !resourceStatus.Healthy {
			allHealthy = false
		}

		messages = append(messages, fmt.Sprintf("%s/%s: %s", kind, name, resourceStatus.Status))
	}

	result.Status = "running"
	result.Message = strings.Join(messages, "; ")
	result.Healthy = allHealthy

	return result, nil
}

// HealthCheck performs health verification
func (p *KubernetesProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
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
	}, nil
}

// Rollback rolls back to previous version
func (p *KubernetesProvider) Rollback(ctx context.Context, comp types.Component) error {
	// Kubernetes rollback would require keeping revision history
	// For now, we'll implement a simple approach
	fmt.Printf("↩️  Rolling back Kubernetes deployment: %s\n", comp.Name)

	// Get current deployment revision
	deployments, err := p.getDeploymentsFromManifest(comp)
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	for _, dep := range deployments {
		// Rollback deployment to previous revision
		if err := p.rollbackDeployment(ctx, dep); err != nil {
			return fmt.Errorf("failed to rollback deployment %s: %w", dep, err)
		}
	}

	return nil
}

// ValidateConfig validates component configuration
func (p *KubernetesProvider) ValidateConfig(config map[string]interface{}) error {
	// Check for required fields
	if source, ok := config["source"].(string); !ok || source == "" {
		return fmt.Errorf("source is required for Kubernetes components")
	}

	// Check if source exists
	if source, ok := config["source"].(string); ok {
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("source path does not exist: %s", source)
		}
	}

	// Validate namespace if provided
	if namespace, ok := config["namespace"].(string); ok && namespace != "" {
		if !isValidKubernetesName(namespace) {
			return fmt.Errorf("invalid namespace: %s", namespace)
		}
	}

	return nil
}

// Helper methods
func (p *KubernetesProvider) initializeClient(comp types.Component) error {
	if p.client != nil {
		return nil // Already initialized
	}

	// Get configuration from component variables
	kubeconfig := ""
	if kc, ok := comp.Variables["kubeconfig"].(string); ok {
		kubeconfig = kc
	}

	context := ""
	if ctx, ok := comp.Variables["context"].(string); ok {
		context = ctx
	}

	if ns, ok := comp.Variables["namespace"].(string); ok && ns != "" {
		p.namespace = ns
	}

	// Create client
	client, err := NewKubernetesClient(
		WithKubeconfig(kubeconfig),
		WithContext(context),
		WithNamespace(p.namespace),
	)
	if err != nil {
		return err
	}

	p.client = client

	// Create manifest processor
	templateData := make(map[string]interface{})
	if vars, ok := comp.Variables["template_vars"].(map[string]interface{}); ok {
		for k, v := range vars {
			templateData[k] = v
		}
	}

	p.processor = NewManifestProcessor(p.namespace, templateData)

	return nil
}

func (p *KubernetesProvider) processManifests(comp types.Component) ([]unstructured.Unstructured, error) {
	// Check if source is a file or directory
	info, err := os.Stat(comp.Source)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return p.processor.ProcessDirectory(comp.Source)
	} else {
		return p.processor.ProcessManifest(comp.Source)
	}
}

func (p *KubernetesProvider) ensureNamespace(ctx context.Context) error {
	// Check if namespace exists
	namespaces, err := p.client.GetNamespaces(ctx)
	if err != nil {
		return err
	}

	for _, ns := range namespaces {
		if ns == p.namespace {
			return nil // Namespace exists
		}
	}

	// Create namespace
	fmt.Printf("  Creating namespace: %s\n", p.namespace)
	return p.client.CreateNamespace(ctx, p.namespace)
}

func (p *KubernetesProvider) waitForDeployments(ctx context.Context, resources []AppliedResource) error {
	for _, resource := range resources {
		if resource.Kind == "Deployment" {
			fmt.Printf("  Waiting for deployment: %s/%s\n", resource.Namespace, resource.Name)
			if err := p.client.WaitForDeployment(ctx, resource.Namespace, resource.Name, 5*time.Minute); err != nil {
				return fmt.Errorf("deployment %s/%s not ready: %w", resource.Namespace, resource.Name, err)
			}
			fmt.Printf("  ✅ Deployment ready: %s/%s\n", resource.Namespace, resource.Name)
		}
	}
	return nil
}

func (p *KubernetesProvider) checkResourceStatus(ctx context.Context, kind, namespace, name string) (*ResourceStatus, error) {
	switch kind {
	case "Deployment":
		return p.checkDeploymentStatus(ctx, namespace, name)
	case "Service":
		return p.checkServiceStatus(ctx, namespace, name)
	case "Pod":
		return p.checkPodStatus(ctx, namespace, name)
	default:
		// Generic check - just see if resource exists
		return &ResourceStatus{
			Status:  "exists",
			Message: "Resource exists",
			Healthy: true,
		}, nil
	}
}

func (p *KubernetesProvider) checkDeploymentStatus(ctx context.Context, namespace, name string) (*ResourceStatus, error) {
	status, err := p.client.GetDeploymentStatus(ctx, namespace, name)
	if err != nil {
		return &ResourceStatus{
			Status:  "not_found",
			Message: fmt.Sprintf("Deployment not found: %v", err),
			Healthy: false,
		}, nil
	}

	if status.ReadyReplicas == status.Replicas && status.Replicas > 0 {
		return &ResourceStatus{
			Status:  "ready",
			Message: fmt.Sprintf("Ready (%d/%d replicas)", status.ReadyReplicas, status.Replicas),
			Healthy: true,
		}, nil
	}

	return &ResourceStatus{
		Status:  "not_ready",
		Message: fmt.Sprintf("Not ready (%d/%d replicas)", status.ReadyReplicas, status.Replicas),
		Healthy: false,
	}, nil
}

func (p *KubernetesProvider) checkServiceStatus(ctx context.Context, namespace, name string) (*ResourceStatus, error) {
	status, err := p.client.GetServiceStatus(ctx, namespace, name)
	if err != nil {
		return &ResourceStatus{
			Status:  "not_found",
			Message: fmt.Sprintf("Service not found: %v", err),
			Healthy: false,
		}, nil
	}

	return &ResourceStatus{
		Status:  "ready",
		Message: fmt.Sprintf("Type: %s, ClusterIP: %s", status.Type, status.ClusterIP),
		Healthy: true,
	}, nil
}

func (p *KubernetesProvider) checkPodStatus(ctx context.Context, namespace, name string) (*ResourceStatus, error) {
	pod, err := p.client.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ResourceStatus{
			Status:  "not_found",
			Message: fmt.Sprintf("Pod not found: %v", err),
			Healthy: false,
		}, nil
	}

	if pod.Status.Phase == corev1.PodRunning {
		return &ResourceStatus{
			Status:  "running",
			Message: "Pod is running",
			Healthy: true,
		}, nil
	}

	return &ResourceStatus{
		Status:  string(pod.Status.Phase),
		Message: fmt.Sprintf("Pod phase: %s", pod.Status.Phase),
		Healthy: pod.Status.Phase == corev1.PodRunning,
	}, nil
}

func (p *KubernetesProvider) shouldDeleteNamespace(comp types.Component) bool {
	if deleteNs, ok := comp.Variables["delete_namespace"].(string); ok {
		return strings.ToLower(deleteNs) == "true"
	}
	if deleteNs, ok := comp.Variables["delete_namespace"].(bool); ok {
		return deleteNs
	}
	return false
}

func (p *KubernetesProvider) getDeploymentsFromManifest(comp types.Component) ([]string, error) {
	manifests, err := p.processManifests(comp)
	if err != nil {
		return nil, err
	}

	var deployments []string
	for _, manifest := range manifests {
		if manifest.GetKind() == "Deployment" {
			deployments = append(deployments, manifest.GetName())
		}
	}

	return deployments, nil
}

func (p *KubernetesProvider) rollbackDeployment(ctx context.Context, deploymentName string) error {
	// Get deployment rollout history
	// This is a simplified implementation
	// In production, you'd want to use kubectl rollout undo
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "undo", "deployment", deploymentName, 
		"-n", p.namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

func isValidKubernetesName(name string) bool {
	if len(name) > 253 {
		return false
	}
	
	// Must match regex: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	for i, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
		if i == 0 && r == '-' {
			return false
		}
		if i == len(name)-1 && r == '-' {
			return false
		}
	}
	
	return true
}

// ResourceStatus represents the status of a Kubernetes resource
type ResourceStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Healthy bool   `json:"healthy"`
}