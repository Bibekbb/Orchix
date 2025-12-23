package kubernetes

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

type KubernetesProvider struct {
	client  kubernetes.Interface
	dynamic dynamic.Interface
	config  *rest.Config
}

func NewKubernetesProvider(kubeconfigPath string) (*KubernetesProvider, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubernetesProvider{
		client:  client,
		dynamic: dynamicClient,
		config:  config,
	}, nil
}

func (p *KubernetesProvider) Name() string {
	return "kubernetes"
}

func (p *KubernetesProvider) Apply(ctx context.Context, comp types.Component, variables map[string]string) (*providers.ApplyResult, error) {
	// Read and parse Kubernetes manifests
	manifests, err := p.parseManifests(comp.Source)
	if err != nil {
		return nil, err
	}

	// Apply each manifest
	for _, obj := range manifests {
		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: p.resourceForKind(gvk.Kind),
		}

		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		// Check if resource exists
		existing, err := p.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, obj.GetName(), metav1.GetOptions{})

		if errors.IsNotFound(err) {
			// Create new resource
			_, err = p.dynamic.Resource(gvr).Namespace(namespace).Create(ctx, &obj, metav1.CreateOptions{})
		} else if err == nil {
			// Update existing resource
			obj.SetResourceVersion(existing.GetResourceVersion())
			_, err = p.dynamic.Resource(gvr).Namespace(namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		}

		if err != nil {
			return nil, fmt.Errorf("failed to apply %s %s: %w", gvk.Kind, obj.GetName(), err)
		}
	}

	return &providers.ApplyResult{}, nil
}

func (p *KubernetesProvider) Plan(ctx context.Context, comp types.Component, variables map[string]string) (*providers.PlanResult, error) {
	// For Kubernetes, plan is essentially a dry-run apply
	// This is a simplified implementation
	return &providers.PlanResult{
		Changes: []providers.Change{},
		Outputs: map[string]string{},
	}, nil
}

func (p *KubernetesProvider) Destroy(ctx context.Context, comp types.Component, variables map[string]string) error {
	manifests, err := p.parseManifests(comp.Source)
	if err != nil {
		return err
	}

	// Delete manifests in reverse order
	for i := len(manifests) - 1; i >= 0; i-- {
		obj := manifests[i]
		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: p.resourceForKind(gvk.Kind),
		}

		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		err := p.dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete %s %s: %w", gvk.Kind, obj.GetName(), err)
		}
	}

	return nil
}

func (p *KubernetesProvider) GetStatus(ctx context.Context, comp types.Component) (*providers.ComponentStatus, error) {
	// Parse manifests to find deployments/statefulsets
	manifests, err := p.parseManifests(comp.Source)
	if err != nil {
		return nil, err
	}

	for _, obj := range manifests {
		if obj.GetKind() == "Deployment" {
			return p.checkDeploymentStatus(ctx, obj.GetNamespace(), obj.GetName())
		}
	}

	return &providers.ComponentStatus{Healthy: true}, nil
}

func (p *KubernetesProvider) checkDeploymentStatus(ctx context.Context, namespace, name string) (*providers.ComponentStatus, error) {
	if namespace == "" {
		namespace = "default"
	}

	deployment, err := p.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Check if all replicas are ready
	if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
		return &providers.ComponentStatus{
			Healthy: false,
			Message: fmt.Sprintf("Deployment not ready: %d/%d replicas",
				deployment.Status.ReadyReplicas, *deployment.Spec.Replicas),
		}, nil
	}

	// Check pod conditions
	pods, err := p.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})

	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if !p.isPodReady(pod) {
			return &providers.ComponentStatus{
				Healthy: false,
				Message: fmt.Sprintf("Pod %s not ready", pod.Name),
			}, nil
		}
	}

	return &providers.ComponentStatus{Healthy: true}, nil
}

func (p *KubernetesProvider) isPodReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (p *KubernetesProvider) parseManifests(source string) ([]unstructured.Unstructured, error) {
	data, err := ioutil.ReadFile(source)
	if err != nil {
		return nil, err
	}

	// Split YAML documents
	docs := splitYAML(data)
	var manifests []unstructured.Unstructured

	for _, doc := range docs {
		if len(doc) == 0 {
			continue
		}

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			return nil, err
		}

		if obj.GetName() != "" {
			manifests = append(manifests, obj)
		}
	}

	return manifests, nil
}

func (p *KubernetesProvider) resourceForKind(kind string) string {
	// Map Kubernetes kinds to resource names
	switch kind {
	case "Deployment":
		return "deployments"
	case "StatefulSet":
		return "statefulsets"
	case "Service":
		return "services"
	case "ConfigMap":
		return "configmaps"
	case "Secret":
		return "secrets"
	case "Ingress":
		return "ingresses"
	default:
		return kind + "s"
	}
}

func splitYAML(data []byte) [][]byte {
	var docs [][]byte
	var current []byte

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if len(current) > 0 {
				docs = append(docs, []byte(strings.Join(strings.Split(string(current), "\n")[:len(strings.Split(string(current), "\n"))-1], "\n")))
				current = nil
			}
		} else {
			current = append(current, []byte(line+"\n")...)
		}
	}

	if len(current) > 0 {
		docs = append(docs, current)
	}

	return docs
}
