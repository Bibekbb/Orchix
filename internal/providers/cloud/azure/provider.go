package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/internal/utils"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// AzureProvider implements the Azure Cloud provider
type AzureProvider struct {
	subscriptionID string
	resourceGroup  string
	location       string
	credentials    *azidentity.DefaultAzureCredential
	clients        *AzureClients
	logger         *utils.Logger
}

// AzureClients holds all Azure service clients
type AzureClients struct {
	Resources      *armresources.ResourceGroupsClient
	Compute        *armcompute.VirtualMachinesClient
	Network        *armnetwork.VirtualNetworksClient
	Storage        *armstorage.AccountsClient
	KeyVault       *armkeyvault.VaultsClient
	SQL            *armsql.ServersClient
	AKS            *armcontainerservice.ManagedClustersClient
	AppService     *armappservice.WebAppsClient
	Subscriptions  *armresources.SubscriptionsClient
}

// NewAzureProvider creates a new Azure provider
func NewAzureProvider() *AzureProvider {
	return &AzureProvider{
		logger: utils.NewLogger("azure", utils.LevelInfo),
	}
}

// Name returns the provider name
func (p *AzureProvider) Name() string {
	return "azure"
}

// Plan generates a deployment plan for Azure resources
func (p *AzureProvider) Plan(ctx context.Context, comp types.Component) (providers.PlanResult, error) {
	p.logger.Info("📋 Planning Azure deployment: %s", comp.Name)

	result := providers.NewPlanResult()

	// Initialize Azure client
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	// Parse deployment type
	deploymentType := p.getDeploymentType(comp)

	switch deploymentType {
	case "arm":
		return p.planARMTemplate(ctx, comp, result)
	case "aks":
		return p.planAKSCluster(ctx, comp, result)
	case "vm":
		return p.planVirtualMachine(ctx, comp, result)
	case "storage":
		return p.planStorageAccount(ctx, comp, result)
	case "sql":
		return p.planSQLDatabase(ctx, comp, result)
	case "webapp":
		return p.planWebApp(ctx, comp, result)
	default:
		return result, fmt.Errorf("unsupported deployment type: %s", deploymentType)
	}
}

// Apply executes the Azure deployment
func (p *AzureProvider) Apply(ctx context.Context, comp types.Component) (providers.ApplyResult, error) {
	startTime := time.Now()
	result := providers.NewApplyResult()

	p.logger.Info("☁️  Deploying to Azure: %s", comp.Name)

	// Initialize Azure client
	if err := p.initialize(comp); err != nil {
		return result, fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	// Ensure resource group exists
	if err := p.ensureResourceGroup(ctx); err != nil {
		return result, fmt.Errorf("failed to create resource group: %w", err)
	}

	// Parse deployment type
	deploymentType := p.getDeploymentType(comp)

	switch deploymentType {
	case "arm":
		return p.deployARMTemplate(ctx, comp, result, startTime)
	case "aks":
		return p.deployAKSCluster(ctx, comp, result, startTime)
	case "vm":
		return p.deployVirtualMachine(ctx, comp, result, startTime)
	case "storage":
		return p.deployStorageAccount(ctx, comp, result, startTime)
	case "sql":
		return p.deploySQLDatabase(ctx, comp, result, startTime)
	case "webapp":
		return p.deployWebApp(ctx, comp, result, startTime)
	default:
		return result, fmt.Errorf("unsupported deployment type: %s", deploymentType)
	}
}

// Destroy removes Azure resources
func (p *AzureProvider) Destroy(ctx context.Context, comp types.Component) error {
	p.logger.Info("🗑️  Destroying Azure resources: %s", comp.Name)

	// Initialize Azure client
	if err := p.initialize(comp); err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	deploymentType := p.getDeploymentType(comp)

	switch deploymentType {
	case "arm":
		return p.deleteARMTemplateResources(ctx, comp)
	case "aks":
		return p.deleteAKSCluster(ctx, comp)
	case "vm":
		return p.deleteVirtualMachine(ctx, comp)
	case "storage":
		return p.deleteStorageAccount(ctx, comp)
	case "sql":
		return p.deleteSQLDatabase(ctx, comp)
	case "webapp":
		return p.deleteWebApp(ctx, comp)
	case "resource-group":
		return p.deleteResourceGroup(ctx)
	default:
		return p.deleteResourcesByName(ctx, comp)
	}
}

// Status checks the Azure deployment status
func (p *AzureProvider) Status(ctx context.Context, comp types.Component) (providers.StatusResult, error) {
	result := providers.NewStatusResult()

	p.logger.Info("📊 Checking Azure deployment status: %s", comp.Name)

	// Initialize Azure client
	if err := p.initialize(comp); err != nil {
		result.Status = "unknown"
		result.Message = fmt.Sprintf("Failed to initialize: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Check Azure connectivity
	if err := p.checkConnectivity(ctx); err != nil {
		result.Status = "unreachable"
		result.Message = fmt.Sprintf("Azure not reachable: %v", err)
		result.Healthy = false
		return result, nil
	}

	// Check resource group
	rgExists, err := p.resourceGroupExists(ctx)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to check resource group: %v", err)
		result.Healthy = false
		return result, nil
	}

	if !rgExists {
		result.Status = "not_deployed"
		result.Message = "Resource group does not exist"
		result.Healthy = false
		return result, nil
	}

	result.AddDetail("resource_group", p.resourceGroup)
	result.AddDetail("location", p.location)

	// Check specific resources based on deployment type
	deploymentType := p.getDeploymentType(comp)
	resourcesHealthy := true
	var resourceMessages []string

	switch deploymentType {
	case "aks":
		status, err := p.checkAKSClusterStatus(ctx, comp)
		if err != nil {
			resourceMessages = append(resourceMessages, fmt.Sprintf("AKS: error (%v)", err))
			resourcesHealthy = false
		} else {
			resourceMessages = append(resourceMessages, fmt.Sprintf("AKS: %s", status))
			result.AddDetail("aks_cluster", status)
		}

	case "vm":
		status, err := p.checkVMStatus(ctx, comp)
		if err != nil {
			resourceMessages = append(resourceMessages, fmt.Sprintf("VM: error (%v)", err))
			resourcesHealthy = false
		} else {
			resourceMessages = append(resourceMessages, fmt.Sprintf("VM: %s", status))
			result.AddDetail("virtual_machine", status)
		}

	case "storage":
		status, err := p.checkStorageAccountStatus(ctx, comp)
		if err != nil {
			resourceMessages = append(resourceMessages, fmt.Sprintf("Storage: error (%v)", err))
			resourcesHealthy = false
		} else {
			resourceMessages = append(resourceMessages, fmt.Sprintf("Storage: %s", status))
			result.AddDetail("storage_account", status)
		}

	default:
		resources, err := p.listResourcesInGroup(ctx)
		if err != nil {
			resourceMessages = append(resourceMessages, fmt.Sprintf("Resources: error listing (%v)", err))
			resourcesHealthy = false
		} else {
			resourceMessages = append(resourceMessages, fmt.Sprintf("Resources: %d found", len(resources)))
			result.AddDetail("resource_count", fmt.Sprintf("%d", len(resources)))
		}
	}

	result.Status = "deployed"
	result.Message = strings.Join(resourceMessages, "; ")
	result.Healthy = resourcesHealthy

	return result, nil
}

// HealthCheck performs health verification for Azure resources
func (p *AzureProvider) HealthCheck(ctx context.Context, comp types.Component) (providers.HealthCheckResult, error) {
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

// Initialize Azure provider
func (p *AzureProvider) initialize(comp types.Component) error {
	// Parse configuration
	if subID, ok := comp.Variables["subscription_id"].(string); ok {
		p.subscriptionID = subID
	} else {
		p.subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
		if p.subscriptionID == "" {
			return fmt.Errorf("subscription_id not found in configuration or environment")
		}
	}

	if rg, ok := comp.Variables["resource_group"].(string); ok {
		p.resourceGroup = rg
	} else {
		return fmt.Errorf("resource_group not found in configuration")
	}

	if loc, ok := comp.Variables["location"].(string); ok {
		p.location = loc
	} else {
		p.location = "eastus"
	}

	// Create credentials
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to create Azure credentials: %w", err)
	}
	p.credentials = cred

	// Initialize clients
	if err := p.initClients(); err != nil {
		return fmt.Errorf("failed to initialize Azure clients: %w", err)
	}

	return nil
}

func (p *AzureProvider) initClients() error {
	options := &arm.ClientOptions{}

	// Resource Groups client
	rgClient, err := armresources.NewResourceGroupsClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create resource groups client: %w", err)
	}

	// Compute client
	computeClient, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	// Network client
	networkClient, err := armnetwork.NewVirtualNetworksClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create network client: %w", err)
	}

	// Storage client
	storageClient, err := armstorage.NewAccountsClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	// Key Vault client
	keyVaultClient, err := armkeyvault.NewVaultsClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create key vault client: %w", err)
	}

	// SQL client
	sqlClient, err := armsql.NewServersClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create SQL client: %w", err)
	}

	// AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	// App Service client
	appServiceClient, err := armappservice.NewWebAppsClient(p.subscriptionID, p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create App Service client: %w", err)
	}

	// Subscriptions client
	subscriptionsClient, err := armresources.NewSubscriptionsClient(p.credentials, options)
	if err != nil {
		return fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	p.clients = &AzureClients{
		Resources:     rgClient,
		Compute:       computeClient,
		Network:       networkClient,
		Storage:       storageClient,
		KeyVault:      keyVaultClient,
		SQL:           sqlClient,
		AKS:           aksClient,
		AppService:    appServiceClient,
		Subscriptions: subscriptionsClient,
	}

	return nil
}

func (p *AzureProvider) ensureResourceGroup(ctx context.Context) error {
	_, err := p.clients.Resources.Get(ctx, p.resourceGroup, nil)
	if err == nil {
		return nil
	}

	parameters := armresources.ResourceGroup{
		Location: to.Ptr(p.location),
		Tags: map[string]*string{
			"created-by": to.Ptr("orchix"),
			"project":    to.Ptr(p.resourceGroup),
		},
	}

	_, err = p.clients.Resources.CreateOrUpdate(ctx, p.resourceGroup, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}

	p.logger.Info("Created resource group: %s", p.resourceGroup)
	return nil
}

func (p *AzureProvider) getDeploymentType(comp types.Component) string {
	if deploymentType, ok := comp.Variables["deployment_type"].(string); ok {
		return deploymentType
	}

	source := comp.Source
	if strings.Contains(source, "aks") || strings.Contains(source, "kubernetes") {
		return "aks"
	} else if strings.Contains(source, "vm") || strings.Contains(source, "virtual-machine") {
		return "vm"
	} else if strings.Contains(source, "storage") {
		return "storage"
	} else if strings.Contains(source, "sql") || strings.Contains(source, "database") {
		return "sql"
	} else if strings.Contains(source, "webapp") || strings.Contains(source, "appservice") {
		return "webapp"
	} else if strings.HasSuffix(source, ".json") || strings.HasSuffix(source, ".bicep") {
		return "arm"
	}

	return "arm"
}

// AKS-specific methods
func (p *AzureProvider) planAKSCluster(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	clusterName := p.getResourceName(comp, "aks")

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.aks.%s", clusterName),
		Action:  "create",
		After:   fmt.Sprintf("AKS cluster: %s", clusterName),
	})

	result.SetOutput("cluster_name", clusterName)
	result.SetOutput("resource_group", p.resourceGroup)
	result.SetOutput("location", p.location)
	result.SetOutput("provider", "azure")

	return result, nil
}

func (p *AzureProvider) deployAKSCluster(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	clusterName := p.getResourceName(comp, "aks")

	p.logger.Info("Creating AKS cluster: %s", clusterName)

	// Parse AKS configuration
	nodeCount := to.Ptr[int32](3)
	if nc, ok := comp.Variables["node_count"].(int); ok {
		nodeCount = to.Ptr[int32](int32(nc))
	}

	vmSize := to.Ptr("Standard_DS2_v2")
	if size, ok := comp.Variables["vm_size"].(string); ok {
		vmSize = to.Ptr(size)
	}

	// Create AKS cluster
	cluster := armcontainerservice.ManagedCluster{
		Location: to.Ptr(p.location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix:         to.Ptr(fmt.Sprintf("%s-dns", clusterName)),
			KubernetesVersion: to.Ptr("1.27"),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:   to.Ptr("agentpool"),
					Count:  nodeCount,
					VMSize: vmSize,
					MaxPods:      to.Ptr[int32](110),
					OSType:       to.Ptr(armcontainerservice.OSTypeLinux),
					Mode:         to.Ptr(armcontainerservice.AgentPoolModeSystem),
					Type:         to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
				},
			},
		},
		Tags: map[string]*string{
			"created-by": to.Ptr("orchix"),
			"cluster":    to.Ptr(clusterName),
		},
	}

	poller, err := p.clients.AKS.BeginCreateOrUpdate(ctx, p.resourceGroup, clusterName, cluster, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create AKS cluster: %w", err)
	}

	p.logger.Info("AKS cluster creation started...")

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create AKS cluster: %w", err)
	}

	p.logger.Info("✅ AKS cluster created: %s", *resp.Name)

	// Get kubeconfig
	kubeConfig, err := p.getKubeConfig(ctx, clusterName)
	if err != nil {
		p.logger.Warn("Failed to get kubeconfig: %v", err)
	} else {
		result.AddOutput("kubeconfig", kubeConfig)
	}

	result.AddOutput("cluster_name", clusterName)
	result.AddOutput("api_server", *resp.Properties.FQDN)
	result.AddOutput("node_count", fmt.Sprintf("%d", *nodeCount))
	result.Duration = time.Since(startTime)

	return result, nil
}

func (p *AzureProvider) getKubeConfig(ctx context.Context, clusterName string) (string, error) {
	credResult, err := p.clients.AKS.ListClusterUserCredentials(ctx, p.resourceGroup, clusterName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster credentials: %w", err)
	}

	if len(credResult.Kubeconfigs) == 0 {
		return "", fmt.Errorf("no kubeconfig found")
	}

	return string(credResult.Kubeconfigs[0].Value), nil
}

func (p *AzureProvider) checkAKSClusterStatus(ctx context.Context, comp types.Component) (string, error) {
	clusterName := p.getResourceName(comp, "aks")

	cluster, err := p.clients.AKS.Get(ctx, p.resourceGroup, clusterName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return "not_found", nil
		}
		return "", fmt.Errorf("failed to get AKS cluster: %w", err)
	}

	if cluster.Properties == nil || cluster.Properties.PowerState == nil {
		return "unknown", nil
	}

	return string(*cluster.Properties.PowerState.Code), nil
}

func (p *AzureProvider) deleteAKSCluster(ctx context.Context, comp types.Component) error {
	clusterName := p.getResourceName(comp, "aks")

	p.logger.Info("Deleting AKS cluster: %s", clusterName)

	poller, err := p.clients.AKS.BeginDelete(ctx, p.resourceGroup, clusterName, nil)
	if err != nil {
		if isNotFoundError(err) {
			p.logger.Info("AKS cluster not found, already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete AKS cluster: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete AKS cluster: %w", err)
	}

	p.logger.Info("✅ AKS cluster deleted: %s", clusterName)
	return nil
}

// Storage Account methods
func (p *AzureProvider) planStorageAccount(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	accountName := p.getResourceName(comp, "st")

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.storage.%s", accountName),
		Action:  "create",
		After:   fmt.Sprintf("Storage Account: %s", accountName),
	})

	result.SetOutput("storage_account", accountName)
	result.SetOutput("resource_group", p.resourceGroup)
	result.SetOutput("location", p.location)

	return result, nil
}

func (p *AzureProvider) deployStorageAccount(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	accountName := p.getResourceName(comp, "st")

	p.logger.Info("Creating Storage Account: %s", accountName)

	account := armstorage.Account{
		Location: to.Ptr(p.location),
		Kind:     to.Ptr(armstorage.KindStorageV2),
		SKU: &armstorage.SKU{
			Name: to.Ptr(armstorage.SKUNameStandardLRS),
		},
		Tags: map[string]*string{
			"created-by": to.Ptr("orchix"),
		},
	}

	poller, err := p.clients.Storage.BeginCreate(ctx, p.resourceGroup, accountName, account, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create storage account: %w", err)
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create storage account: %w", err)
	}

	p.logger.Info("✅ Storage Account created: %s", *resp.Name)

	result.AddOutput("storage_account", accountName)
	result.AddOutput("primary_endpoint", *resp.Properties.PrimaryEndpoints.Blob)
	result.Duration = time.Since(startTime)

	return result, nil
}

func (p *AzureProvider) checkStorageAccountStatus(ctx context.Context, comp types.Component) (string, error) {
	accountName := p.getResourceName(comp, "st")

	account, err := p.clients.Storage.GetProperties(ctx, p.resourceGroup, accountName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return "not_found", nil
		}
		return "", fmt.Errorf("failed to get storage account: %w", err)
	}

	if account.Properties == nil {
		return "unknown", nil
	}

	if *account.Properties.StatusOfPrimary == armstorage.AccountStatusAvailable {
		return "available", nil
	}

	return string(*account.Properties.StatusOfPrimary), nil
}

func (p *AzureProvider) deleteStorageAccount(ctx context.Context, comp types.Component) error {
	accountName := p.getResourceName(comp, "st")

	p.logger.Info("Deleting Storage Account: %s", accountName)

	_, err := p.clients.Storage.Delete(ctx, p.resourceGroup, accountName, nil)
	if err != nil {
		if isNotFoundError(err) {
			p.logger.Info("Storage account not found, already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete storage account: %w", err)
	}

	p.logger.Info("✅ Storage Account deleted: %s", accountName)
	return nil
}

// VM methods
func (p *AzureProvider) planVirtualMachine(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	vmName := p.getResourceName(comp, "vm")

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.vm.%s", vmName),
		Action:  "create",
		After:   fmt.Sprintf("Virtual Machine: %s", vmName),
	})

	result.SetOutput("vm_name", vmName)
	result.SetOutput("resource_group", p.resourceGroup)
	result.SetOutput("location", p.location)

	return result, nil
}

func (p *AzureProvider) deployVirtualMachine(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	vmName := p.getResourceName(comp, "vm")

	p.logger.Info("Creating Virtual Machine: %s", vmName)

	// Note: This is a simplified example. In production, you'd need to:
	// 1. Create VNet and Subnet first
	// 2. Create Network Interface
	// 3. Create VM with proper configuration

	// For now, use ARM template deployment
	return p.deployARMTemplate(ctx, comp, result, startTime)
}

func (p *AzureProvider) checkVMStatus(ctx context.Context, comp types.Component) (string, error) {
	vmName := p.getResourceName(comp, "vm")

	vm, err := p.clients.Compute.Get(ctx, p.resourceGroup, vmName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return "not_found", nil
		}
		return "", fmt.Errorf("failed to get VM: %w", err)
	}

	if vm.Properties == nil || vm.Properties.InstanceView == nil {
		return "unknown", nil
	}

	// Check power state
	for _, status := range vm.Properties.InstanceView.Statuses {
		if status.Code != nil && strings.HasPrefix(*status.Code, "PowerState/") {
			return strings.TrimPrefix(*status.Code, "PowerState/"), nil
		}
	}

	return "unknown", nil
}

func (p *AzureProvider) deleteVirtualMachine(ctx context.Context, comp types.Component) error {
	vmName := p.getResourceName(comp, "vm")

	p.logger.Info("Deleting Virtual Machine: %s", vmName)

	poller, err := p.clients.Compute.BeginDelete(ctx, p.resourceGroup, vmName, nil)
	if err != nil {
		if isNotFoundError(err) {
			p.logger.Info("VM not found, already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	p.logger.Info("✅ Virtual Machine deleted: %s", vmName)
	return nil
}

// ARM Template methods
func (p *AzureProvider) planARMTemplate(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	templatePath := comp.Source
	template, _, err := p.parseARMTemplate(templatePath)
	if err != nil {
		return result, fmt.Errorf("failed to parse ARM template: %w", err)
	}

	resourceTypes, err := p.extractResourceTypes(template)
	if err != nil {
		return result, fmt.Errorf("failed to extract resource types: %w", err)
	}

	for _, resourceType := range resourceTypes {
		result.AddChange(providers.Change{
			Type:    providers.ChangeTypeCreate,
			Address: fmt.Sprintf("azure.%s", strings.ToLower(resourceType)),
			Action:  "create",
			After:   fmt.Sprintf("Azure %s resource", resourceType),
		})
	}

	result.SetOutput("template_file", templatePath)
	result.SetOutput("resource_count", fmt.Sprintf("%d", len(resourceTypes)))

	return result, nil
}

func (p *AzureProvider) deployARMTemplate(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	templatePath := comp.Source
	template, parameters, err := p.parseARMTemplate(templatePath)
	if err != nil {
		return result, fmt.Errorf("failed to parse ARM template: %w", err)
	}

	deploymentName := fmt.Sprintf("orchix-%s-%d", comp.ID, time.Now().Unix())

	p.logger.Info("Deploying ARM template: %s", deploymentName)

	// Create deployment parameters
	deploymentParams := make(map[string]interface{})
	for key, value := range parameters {
		deploymentParams[key] = map[string]interface{}{
			"value": value,
		}
	}

	// Add component variables to parameters
	for key, value := range comp.Variables {
		if key != "subscription_id" && key != "resource_group" && key != "location" {
			deploymentParams[key] = map[string]interface{}{
				"value": value,
			}
		}
	}

	deployment := armresources.Deployment{
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: deploymentParams,
			Mode:       to.Ptr(armresources.DeploymentModeIncremental),
		},
	}

	// Use deployments client
	deploymentsClient, err := armresources.NewDeploymentsClient(p.subscriptionID, p.credentials, &arm.ClientOptions{})
	if err != nil {
		return result, fmt.Errorf("failed to create deployments client: %w", err)
	}

	poller, err := deploymentsClient.BeginCreateOrUpdate(ctx, p.resourceGroup, deploymentName, deployment, nil)
	if err != nil {
		return result, fmt.Errorf("failed to start deployment: %w", err)
	}

	p.logger.Info("ARM template deployment started...")

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to deploy ARM template: %w", err)
	}

	p.logger.Info("✅ ARM template deployed: %s", deploymentName)

	// Get outputs from deployment
	if resp.Properties.Outputs != nil {
		if outputs, ok := resp.Properties.Outputs.(map[string]interface{}); ok {
			for key, value := range outputs {
				if valueMap, ok := value.(map[string]interface{}); ok {
					if outputValue, ok := valueMap["value"]; ok {
						result.AddOutput(key, fmt.Sprintf("%v", outputValue))
					}
				}
			}
		}
	}

	result.AddOutput("deployment_name", deploymentName)
	result.Duration = time.Since(startTime)

	return result, nil
}

func (p *AzureProvider) deleteARMTemplateResources(ctx context.Context, comp types.Component) error {
	deploymentName := comp.ID
	if customName, ok := comp.Variables["deployment_name"].(string); ok {
		deploymentName = customName
	}

	p.logger.Info("Deleting ARM template deployment: %s", deploymentName)

	deploymentsClient, err := armresources.NewDeploymentsClient(p.subscriptionID, p.credentials, &arm.ClientOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployments client: %w", err)
	}

	poller, err := deploymentsClient.BeginDelete(ctx, p.resourceGroup, deploymentName, nil)
	if err != nil {
		if isNotFoundError(err) {
			p.logger.Info("Deployment not found, already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	p.logger.Info("✅ ARM template deployment deleted: %s", deploymentName)
	return nil
}

func (p *AzureProvider) parseARMTemplate(templatePath string) (map[string]interface{}, map[string]interface{}, error) {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read template file: %w", err)
	}

	var template map[string]interface{}
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON template: %w", err)
	}

	parameters := make(map[string]interface{})
	if params, ok := template["parameters"].(map[string]interface{}); ok {
		for key, param := range params {
			if paramMap, ok := param.(map[string]interface{}); ok {
				if defaultValue, ok := paramMap["defaultValue"]; ok {
					parameters[key] = defaultValue
				}
			}
		}
	}

	return template, parameters, nil
}

func (p *AzureProvider) extractResourceTypes(template map[string]interface{}) ([]string, error) {
	var resourceTypes []string

	resources, ok := template["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resources section in template")
	}

	for _, resource := range resources {
		if resourceMap, ok := resource.(map[string]interface{}); ok {
			if resourceType, ok := resourceMap["type"].(string); ok {
				resourceTypes = append(resourceTypes, resourceType)
			}
		}
	}

	return resourceTypes, nil
}

// Web App methods
func (p *AzureProvider) planWebApp(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	appName := p.getResourceName(comp, "app")

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.webapp.%s", appName),
		Action:  "create",
		After:   fmt.Sprintf("Web App: %s", appName),
	})

	result.SetOutput("webapp_name", appName)
	result.SetOutput("resource_group", p.resourceGroup)
	result.SetOutput("location", p.location)

	return result, nil
}

func (p *AzureProvider) deployWebApp(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	appName := p.getResourceName(comp, "app")

	p.logger.Info("Creating Web App: %s", appName)

	app := armappservice.Site{
		Location: to.Ptr(p.location),
		Properties: &armappservice.SiteProperties{
			ServerFarmID: to.Ptr(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/serverfarms/%s-plan",
				p.subscriptionID, p.resourceGroup, appName)),
			SiteConfig: &armappservice.SiteConfig{
				LinuxFxVersion: to.Ptr("DOCKER|nginx:alpine"),
			},
		},
		Tags: map[string]*string{
			"created-by": to.Ptr("orchix"),
		},
	}

	// First create App Service Plan
	appServicePlan := armappservice.AppServicePlan{
		Location: to.Ptr(p.location),
		SKU: &armappservice.SKUDescription{
			Name:     to.Ptr("B1"),
			Tier:     to.Ptr("Basic"),
			Family:   to.Ptr("B"),
			Capacity: to.Ptr[int32](1),
		},
	}

	planClient, err := armappservice.NewPlansClient(p.subscriptionID, p.credentials, &arm.ClientOptions{})
	if err != nil {
		return result, fmt.Errorf("failed to create plans client: %w", err)
	}

	planPoller, err := planClient.BeginCreateOrUpdate(ctx, p.resourceGroup, fmt.Sprintf("%s-plan", appName), appServicePlan, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create app service plan: %w", err)
	}

	_, err = planPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create app service plan: %w", err)
	}

	// Then create Web App
	poller, err := p.clients.AppService.BeginCreateOrUpdate(ctx, p.resourceGroup, appName, app, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create web app: %w", err)
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create web app: %w", err)
	}

	p.logger.Info("✅ Web App created: %s", *resp.Name)

	result.AddOutput("webapp_name", appName)
	result.AddOutput("url", fmt.Sprintf("https://%s.azurewebsites.net", appName))
	result.Duration = time.Since(startTime)

	return result, nil
}

func (p *AzureProvider) deleteWebApp(ctx context.Context, comp types.Component) error {
	appName := p.getResourceName(comp, "app")
	planName := fmt.Sprintf("%s-plan", appName)

	p.logger.Info("Deleting Web App: %s", appName)

	_, err := p.clients.AppService.Delete(ctx, p.resourceGroup, appName, nil)
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to delete web app: %w", err)
	}

	// Delete App Service Plan
	planClient, err := armappservice.NewPlansClient(p.subscriptionID, p.credentials, &arm.ClientOptions{})
	if err != nil {
		return fmt.Errorf("failed to create plans client: %w", err)
	}

	_, err = planClient.Delete(ctx, p.resourceGroup, planName, nil)
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to delete app service plan: %w", err)
	}

	p.logger.Info("✅ Web App deleted: %s", appName)
	return nil
}

// SQL Database methods (similar pattern)
func (p *AzureProvider) planSQLDatabase(ctx context.Context, comp types.Component, result providers.PlanResult) (providers.PlanResult, error) {
	serverName := p.getResourceName(comp, "sql")
	dbName := "orchixdb"

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.sql.%s", serverName),
		Action:  "create",
		After:   fmt.Sprintf("SQL Server: %s", serverName),
	})

	result.AddChange(providers.Change{
		Type:    providers.ChangeTypeCreate,
		Address: fmt.Sprintf("azure.sql.database.%s", dbName),
		Action:  "create",
		After:   fmt.Sprintf("SQL Database: %s", dbName),
	})

	result.SetOutput("sql_server", serverName)
	result.SetOutput("sql_database", dbName)

	return result, nil
}

func (p *AzureProvider) deploySQLDatabase(ctx context.Context, comp types.Component, result providers.ApplyResult, startTime time.Time) (providers.ApplyResult, error) {
	serverName := p.getResourceName(comp, "sql")
	dbName := "orchixdb"

	p.logger.Info("Creating SQL Server: %s", serverName)

	// Create SQL Server
	server := armsql.Server{
		Location: to.Ptr(p.location),
		Properties: &armsql.ServerProperties{
			AdministratorLogin:         to.Ptr("orchixadmin"),
			AdministratorLoginPassword: to.Ptr(generatePassword()),
			Version:                    to.Ptr("12.0"),
		},
		Tags: map[string]*string{
			"created-by": to.Ptr("orchix"),
		},
	}

	serverPoller, err := p.clients.SQL.BeginCreateOrUpdate(ctx, p.resourceGroup, serverName, server, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create SQL server: %w", err)
	}

	_, err = serverPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create SQL server: %w", err)
	}

	// Create SQL Database
	dbClient, err := armsql.NewDatabasesClient(p.subscriptionID, p.credentials, &arm.ClientOptions{})
	if err != nil {
		return result, fmt.Errorf("failed to create databases client: %w", err)
	}

	database := armsql.Database{
		Location: to.Ptr(p.location),
		Properties: &armsql.DatabaseProperties{
			Collation:    to.Ptr("SQL_Latin1_General_CP1_CI_AS"),
			MaxSizeBytes: to.Ptr[int64](2147483648), // 2GB
		},
	}

	dbPoller, err := dbClient.BeginCreateOrUpdate(ctx, p.resourceGroup, serverName, dbName, database, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create SQL database: %w", err)
	}

	_, err = dbPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create SQL database: %w", err)
	}

	p.logger.Info("✅ SQL Database created: %s/%s", serverName, dbName)

	result.AddOutput("sql_server", serverName)
	result.AddOutput("sql_database", dbName)
	result.AddOutput("connection_string", fmt.Sprintf("Server=tcp:%s.database.windows.net,1433;Database=%s;User ID=orchixadmin;Password=[hidden];Encrypt=true;TrustServerCertificate=false;Connection Timeout=30;", serverName, dbName))
	result.Duration = time.Since(startTime)

	return result, nil
}

// Helper methods
func (p *AzureProvider) getResourceName(comp types.Component, prefix string) string {
	if name, ok := comp.Variables["name"].(string); ok && name != "" {
		return name
	}
	// Generate a unique name based on component ID and prefix
	return fmt.Sprintf("%s-%s-%s", prefix, comp.ID, p.resourceGroup)
}

func (p *AzureProvider) checkConnectivity(ctx context.Context) error {
	_, err := p.clients.Subscriptions.Get(ctx, p.subscriptionID, nil)
	return err
}

func (p *AzureProvider) resourceGroupExists(ctx context.Context) (bool, error) {
	_, err := p.clients.Resources.Get(ctx, p.resourceGroup, nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *AzureProvider) listResourcesInGroup(ctx context.Context) ([]armresources.GenericResourceExpanded, error) {
	var resources []armresources.GenericResourceExpanded
	
	pager := p.clients.Resources.NewListByResourceGroupPager(p.resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}
		resources = append(resources, page.Value...)
	}
	
	return resources, nil
}

func (p *AzureProvider) deleteResourceGroup(ctx context.Context) error {
	p.logger.Info("Deleting resource group: %s", p.resourceGroup)

	poller, err := p.clients.Resources.BeginDelete(ctx, p.resourceGroup, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource group: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource group: %w", err)
	}

	p.logger.Info("✅ Resource group deleted: %s", p.resourceGroup)
	return nil
}

func (p *AzureProvider) deleteResourcesByName(ctx context.Context, comp types.Component) error {
	// Get resource names from component
	resourceNames := []string{}
	if names, ok := comp.Variables["resource_names"].([]interface{}); ok {
		for _, name := range names {
			if str, ok := name.(string); ok {
				resourceNames = append(resourceNames, str)
			}
		}
	}

	for _, name := range resourceNames {
		p.logger.Info("Deleting resource: %s", name)
		// This would need to determine resource type and use appropriate client
	}

	return nil
}

func generatePassword() string {
	return fmt.Sprintf("Orchix@%d", time.Now().Unix())
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	
	var azErr *azcore.ResponseError
	if ok := errors.As(err, &azErr); ok {
		return azErr.StatusCode == 404
	}
	
	return strings.Contains(err.Error(), "404") || 
	       strings.Contains(err.Error(), "not found") ||
	       strings.Contains(err.Error(), "ResourceNotFound")
}