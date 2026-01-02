package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// KubernetesClient provides a wrapper around Kubernetes operations
type KubernetesClient struct {
	clientset  kubernetes.Interface
	dynamic    dynamic.Interface
	config     *rest.Config
	namespace  string
	kubeconfig string
	context    string
}

// NewKubernetesClient creates a new Kubernetes client
func NewKubernetesClient(options ...ClientOption) (*KubernetesClient, error) {
	client := &KubernetesClient{
		namespace: "default",
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Load configuration
	config, err := client.loadConfig()
	if err != nil {
		return nil, err
	}
	client.config = config

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}
	client.clientset = clientset

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	client.dynamic = dynamicClient

	return client, nil
}

// ClientOption configures the Kubernetes client
type ClientOption func(*KubernetesClient)

// WithKubeconfig sets the kubeconfig path
func WithKubeconfig(path string) ClientOption {
	return func(k *KubernetesClient) {
		k.kubeconfig = path
	}
}

// WithContext sets the Kubernetes context
func WithContext(context string) ClientOption {
	return func(k *KubernetesClient) {
		k.context = context
	}
}

// WithNamespace sets the default namespace
func WithNamespace(namespace string) ClientOption {
	return func(k *KubernetesClient) {
		k.namespace = namespace
	}
}

// loadConfig loads Kubernetes configuration
func (k *KubernetesClient) loadConfig() (*rest.Config, error) {
	// If no kubeconfig specified, try in-cluster config first
	if k.kubeconfig == "" {
		// First try in-cluster config
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}

		// Fall back to default kubeconfig location
		if home := homedir.HomeDir(); home != "" {
			k.kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// Use specified kubeconfig
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: k.kubeconfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: k.context,
		},
	).ClientConfig()

	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return config, nil
}

// ApplyManifest applies a Kubernetes manifest
func (k *KubernetesClient) ApplyManifest(ctx context.Context, manifestPath string) ([]AppliedResource, error) {
	// Read manifest file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Parse YAML documents
	docs, err := k.parseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var appliedResources []AppliedResource

	for _, obj := range docs {
		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: k.resourceForKind(gvk.Kind),
		}

		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = k.namespace
		}

		name := obj.GetName()
		if name == "" {
			continue // Skip resources without name
		}

		// Apply the resource
		result, err := k.applyResource(ctx, gvr, namespace, name, obj)
		if err != nil {
			return appliedResources, fmt.Errorf("failed to apply resource %s/%s: %w", gvk.Kind, name, err)
		}

		appliedResources = append(appliedResources, AppliedResource{
			Kind:      gvk.Kind,
			Name:      name,
			Namespace: namespace,
			Resource:  result,
		})
	}

	return appliedResources, nil
}

// DeleteManifest deletes resources from a manifest
func (k *KubernetesClient) DeleteManifest(ctx context.Context, manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	docs, err := k.parseYAML(data)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Delete in reverse order (dependencies first)
	for i := len(docs) - 1; i >= 0; i-- {
		obj := docs[i]
		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: k.resourceForKind(gvk.Kind),
		}

		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = k.namespace
		}

		name := obj.GetName()
		if name == "" {
			continue
		}

		if err := k.deleteResource(ctx, gvr, namespace, name); err != nil {
			// Log error but continue with other resources
			fmt.Printf("Warning: failed to delete %s/%s: %v\n", gvk.Kind, name, err)
		}
	}

	return nil
}

// GetDeploymentStatus gets status of a deployment
func (k *KubernetesClient) GetDeploymentStatus(ctx context.Context, namespace, name string) (*DeploymentStatus, error) {
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status := &DeploymentStatus{
		Name:              deployment.Name,
		Namespace:         deployment.Namespace,
		Replicas:          *deployment.Spec.Replicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		ReadyReplicas:     deployment.Status.ReadyReplicas,
		Conditions:        deployment.Status.Conditions,
		CreatedAt:         deployment.CreationTimestamp.Time,
	}

	// Get pods for this deployment
	pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return status, nil // Return status even if pods can't be listed
	}

	for _, pod := range pods.Items {
		podStatus := PodStatus{
			Name:   pod.Name,
			Status: string(pod.Status.Phase),
			Ready:  k.isPodReady(pod),
		}

		// Get container statuses
		for _, cs := range pod.Status.ContainerStatuses {
			containerStatus := ContainerStatus{
				Name:  cs.Name,
				Ready: cs.Ready,
				State: fmt.Sprintf("%v", cs.State),
			}
			podStatus.Containers = append(podStatus.Containers, containerStatus)
		}

		status.Pods = append(status.Pods, podStatus)
	}

	return status, nil
}

// GetServiceStatus gets status of a service
func (k *KubernetesClient) GetServiceStatus(ctx context.Context, namespace, name string) (*ServiceStatus, error) {
	service, err := k.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status := &ServiceStatus{
		Name:       service.Name,
		Namespace:  service.Namespace,
		Type:       string(service.Spec.Type),
		ClusterIP:  service.Spec.ClusterIP,
		ExternalIP: service.Spec.ExternalIPs,
		Ports:      service.Spec.Ports,
		CreatedAt:  service.CreationTimestamp.Time,
	}

	// Get endpoints
	endpoints, err := k.clientset.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				status.Endpoints = append(status.Endpoints, addr.IP)
			}
		}
	}

	return status, nil
}

// WaitForDeployment waits for deployment to be ready
func (k *KubernetesClient) WaitForDeployment(ctx context.Context, namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s/%s", namespace, name)
		case <-ticker.C:
			status, err := k.GetDeploymentStatus(ctx, namespace, name)
			if err != nil {
				return err
			}

			if status.ReadyReplicas == status.Replicas && status.Replicas > 0 {
				return nil
			}
		}
	}
}

// GetNamespaces lists all namespaces
func (k *KubernetesClient) GetNamespaces(ctx context.Context) ([]string, error) {
	namespaces, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		result = append(result, ns.Name)
	}

	return result, nil
}

// CreateNamespace creates a namespace
func (k *KubernetesClient) CreateNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := k.clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	return err
}

// DeleteNamespace deletes a namespace
func (k *KubernetesClient) DeleteNamespace(ctx context.Context, name string) error {
	return k.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}

// ApplyConfigMap applies a ConfigMap
func (k *KubernetesClient) ApplyConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	_, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = k.clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	} else if err == nil {
		_, err = k.clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	}
	return err
}

// ApplySecret applies a Secret
func (k *KubernetesClient) ApplySecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	_, err := k.clientset.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = k.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	} else if err == nil {
		_, err = k.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

// GetLogs gets logs from a pod
func (k *KubernetesClient) GetLogs(ctx context.Context, namespace, podName string, containerName string, tailLines int64) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		TailLines: &tailLines,
	}
	if containerName != "" {
		podLogOpts.Container = containerName
	}

	req := k.clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	logs, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}

	return string(logs), nil
}

// CheckHealth checks if Kubernetes cluster is reachable
func (k *KubernetesClient) CheckHealth(ctx context.Context) error {
	_, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}

// GetClusterInfo gets cluster information
func (k *KubernetesClient) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	version, err := k.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	nodes, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	info := &ClusterInfo{
		Version:     version.String(),
		Platform:    version.Platform,
		GoVersion:   version.GoVersion,
		NodeCount:   len(nodes.Items),
		Kubernetes:  version.GitVersion,
	}

	return info, nil
}

// Helper methods
func (k *KubernetesClient) parseYAML(data []byte) ([]unstructured.Unstructured, error) {
	var docs []unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)

	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(obj.Object) == 0 {
			continue // Skip empty documents
		}
		docs = append(docs, obj)
	}

	return docs, nil
}

func (k *KubernetesClient) resourceForKind(kind string) string {
	switch kind {
	case "Deployment", "Deployment.apps":
		return "deployments"
	case "StatefulSet", "StatefulSet.apps":
		return "statefulsets"
	case "DaemonSet", "DaemonSet.apps":
		return "daemonsets"
	case "Service":
		return "services"
	case "ConfigMap":
		return "configmaps"
	case "Secret":
		return "secrets"
	case "PersistentVolumeClaim":
		return "persistentvolumeclaims"
	case "Ingress", "Ingress.networking.k8s.io":
		return "ingresses"
	case "Namespace":
		return "namespaces"
	case "ServiceAccount":
		return "serviceaccounts"
	case "Role", "Role.rbac.authorization.k8s.io":
		return "roles"
	case "RoleBinding", "RoleBinding.rbac.authorization.k8s.io":
		return "rolebindings"
	case "ClusterRole", "ClusterRole.rbac.authorization.k8s.io":
		return "clusterroles"
	case "ClusterRoleBinding", "ClusterRoleBinding.rbac.authorization.k8s.io":
		return "clusterrolebindings"
	case "Pod":
		return "pods"
	case "Job", "Job.batch":
		return "jobs"
	case "CronJob", "CronJob.batch":
		return "cronjobs"
	default:
		// Try to pluralize
		return strings.ToLower(kind) + "s"
	}
}

func (k *KubernetesClient) applyResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, obj unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Check if resource exists
	existing, err := k.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Create new resource
		return k.dynamic.Resource(gvr).Namespace(namespace).Create(ctx, &obj, metav1.CreateOptions{})
	} else if err == nil {
		// Update existing resource
		obj.SetResourceVersion(existing.GetResourceVersion())
		return k.dynamic.Resource(gvr).Namespace(namespace).Update(ctx, &obj, metav1.UpdateOptions{})
	}

	return nil, err
}

func (k *KubernetesClient) deleteResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	return k.dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, deleteOptions)
}

func (k *KubernetesClient) isPodReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// Types
type AppliedResource struct {
	Kind      string                    `json:"kind"`
	Name      string                    `json:"name"`
	Namespace string                    `json:"namespace"`
	Resource  *unstructured.Unstructured `json:"resource"`
}

type DeploymentStatus struct {
	Name              string                     `json:"name"`
	Namespace         string                     `json:"namespace"`
	Replicas          int32                      `json:"replicas"`
	AvailableReplicas int32                      `json:"availableReplicas"`
	ReadyReplicas     int32                      `json:"readyReplicas"`
	Conditions        []appsv1.DeploymentCondition `json:"conditions"`
	Pods              []PodStatus                `json:"pods"`
	CreatedAt         time.Time                  `json:"createdAt"`
}

type PodStatus struct {
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	Ready      bool             `json:"ready"`
	Containers []ContainerStatus `json:"containers"`
}

type ContainerStatus struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
	State string `json:"state"`
}

type ServiceStatus struct {
	Name       string             `json:"name"`
	Namespace  string             `json:"namespace"`
	Type       string             `json:"type"`
	ClusterIP  string             `json:"clusterIP"`
	ExternalIP []string           `json:"externalIP"`
	Ports      []corev1.ServicePort `json:"ports"`
	Endpoints  []string           `json:"endpoints"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type ClusterInfo struct {
	Version    string `json:"version"`
	Platform   string `json:"platform"`
	GoVersion  string `json:"goVersion"`
	Kubernetes string `json:"kubernetes"`
	NodeCount  int    `json:"nodeCount"`
}