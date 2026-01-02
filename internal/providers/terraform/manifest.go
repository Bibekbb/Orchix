package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

// ManifestProcessor handles Kubernetes manifest processing
type ManifestProcessor struct {
	templateData map[string]interface{}
	namespace    string
}

// NewManifestProcessor creates a new manifest processor
func NewManifestProcessor(namespace string, templateData map[string]interface{}) *ManifestProcessor {
	return &ManifestProcessor{
		namespace:    namespace,
		templateData: templateData,
	}
}

// ProcessManifest processes a Kubernetes manifest file with templating
func (p *ManifestProcessor) ProcessManifest(filePath string) ([]unstructured.Unstructured, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Apply templating if needed
	processedData, err := p.applyTemplating(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to apply templating: %w", err)
	}

	// Parse YAML documents
	return p.parseManifests([]byte(processedData))
}

// ProcessDirectory processes all manifest files in a directory
func (p *ManifestProcessor) ProcessDirectory(dirPath string) ([]unstructured.Unstructured, error) {
	var allManifests []unstructured.Unstructured

	// Walk directory
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Process only YAML files
		if !isYAMLFile(path) {
			return nil
		}

		manifests, err := p.ProcessManifest(path)
		if err != nil {
			return fmt.Errorf("failed to process %s: %w", path, err)
		}

		allManifests = append(allManifests, manifests...)
		return nil
	})

	return allManifests, err
}

// GenerateDeployment generates a Deployment manifest
func (p *ManifestProcessor) GenerateDeployment(name, image string, replicas int32, config map[string]interface{}) (*unstructured.Unstructured, error) {
	// Default labels
	labels := map[string]string{
		"app": name,
	}

	// Merge custom labels
	if customLabels, ok := config["labels"].(map[string]interface{}); ok {
		for k, v := range customLabels {
			if strVal, ok := v.(string); ok {
				labels[k] = strVal
			}
		}
	}

	// Create deployment object
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": p.namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": name,
						},
					},
					"spec": p.generatePodSpec(name, image, config),
				},
			},
		},
	}

	return deployment, nil
}

// GenerateService generates a Service manifest
func (p *ManifestProcessor) GenerateService(name string, config map[string]interface{}) (*unstructured.Unstructured, error) {
	serviceType := "ClusterIP"
	if svcType, ok := config["type"].(string); ok {
		serviceType = svcType
	}

	ports := []interface{}{}
	if portConfig, ok := config["ports"].([]interface{}); ok {
		for _, port := range portConfig {
			ports = append(ports, port)
		}
	} else if port, ok := config["port"].(int); ok {
		ports = append(ports, map[string]interface{}{
			"port":       port,
			"targetPort": port,
			"name":       "http",
		})
	}

	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": p.namespace,
			},
			"spec": map[string]interface{}{
				"type":  serviceType,
				"ports": ports,
				"selector": map[string]interface{}{
					"app": name,
				},
			},
		},
	}

	return service, nil
}

// GenerateConfigMap generates a ConfigMap manifest
func (p *ManifestProcessor) GenerateConfigMap(name string, data map[string]interface{}) (*unstructured.Unstructured, error) {
	configMap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": p.namespace,
			},
			"data": data,
		},
	}

	return configMap, nil
}

// GenerateSecret generates a Secret manifest
func (p *ManifestProcessor) GenerateSecret(name string, secretType string, data map[string]interface{}) (*unstructured.Unstructured, error) {
	if secretType == "" {
		secretType = "Opaque"
	}

	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": p.namespace,
			},
			"type": secretType,
			"data": data,
		},
	}

	return secret, nil
}

// GenerateIngress generates an Ingress manifest
func (p *ManifestProcessor) GenerateIngress(name string, config map[string]interface{}) (*unstructured.Unstructured, error) {
	rules := []interface{}{}
	if rulesConfig, ok := config["rules"].([]interface{}); ok {
		rules = rulesConfig
	} else if host, ok := config["host"].(string); ok {
		rule := map[string]interface{}{
			"host": host,
			"http": map[string]interface{}{
				"paths": []interface{}{
					map[string]interface{}{
						"path":     config["path"],
						"pathType": "Prefix",
						"backend": map[string]interface{}{
							"service": map[string]interface{}{
								"name": config["serviceName"],
								"port": map[string]interface{}{
									"number": config["servicePort"],
								},
							},
						},
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	ingress := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": p.namespace,
			},
			"spec": map[string]interface{}{
				"rules": rules,
			},
		},
	}

	// Add annotations if specified
	if annotations, ok := config["annotations"].(map[string]interface{}); ok {
		metadata := ingress.Object["metadata"].(map[string]interface{})
		metadata["annotations"] = annotations
	}

	return ingress, nil
}

// ValidateManifest validates a Kubernetes manifest
func (p *ManifestProcessor) ValidateManifest(manifest *unstructured.Unstructured) error {
	// Check required fields
	if manifest.GetAPIVersion() == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if manifest.GetKind() == "" {
		return fmt.Errorf("kind is required")
	}
	if manifest.GetName() == "" {
		return fmt.Errorf("name is required")
	}

	// Validate based on kind
	switch manifest.GetKind() {
	case "Deployment":
		return p.validateDeployment(manifest)
	case "Service":
		return p.validateService(manifest)
	case "ConfigMap":
		return p.validateConfigMap(manifest)
	case "Secret":
		return p.validateSecret(manifest)
	case "Ingress":
		return p.validateIngress(manifest)
	}

	return nil
}

// RenderManifests renders manifests to YAML
func (p *ManifestProcessor) RenderManifests(manifests []unstructured.Unstructured) (string, error) {
	var output strings.Builder

	for i, manifest := range manifests {
		if i > 0 {
			output.WriteString("\n---\n")
		}

		yamlData, err := yaml.Marshal(manifest.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal manifest to YAML: %w", err)
		}

		output.Write(yamlData)
	}

	return output.String(), nil
}

// Helper methods
func (p *ManifestProcessor) applyTemplating(content string) (string, error) {
	// Check if content contains template markers
	if !strings.Contains(content, "{{") || !strings.Contains(content, "}}") {
		return content, nil
	}

	// Create template
	tmpl, err := template.New("manifest").Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p.templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (p *ManifestProcessor) parseManifests(data []byte) ([]unstructured.Unstructured, error) {
	var manifests []unstructured.Unstructured
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)

	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode YAML: %w", err)
		}
		if len(obj.Object) == 0 {
			continue // Skip empty documents
		}

		// Set namespace if not specified
		if obj.GetNamespace() == "" && p.namespace != "" {
			obj.SetNamespace(p.namespace)
		}

		manifests = append(manifests, obj)
	}

	return manifests, nil
}

func (p *ManifestProcessor) generatePodSpec(name, image string, config map[string]interface{}) map[string]interface{} {
	containers := []interface{}{
		map[string]interface{}{
			"name":  name,
			"image": image,
		},
	}

	// Add container configuration
	if containerConfig, ok := config["container"].(map[string]interface{}); ok {
		// Update the first container with config
		container := containers[0].(map[string]interface{})
		
		if ports, ok := containerConfig["ports"].([]interface{}); ok {
			container["ports"] = ports
		}
		
		if env, ok := containerConfig["env"].([]interface{}); ok {
			container["env"] = env
		}
		
		if resources, ok := containerConfig["resources"].(map[string]interface{}); ok {
			container["resources"] = resources
		}
		
		if livenessProbe, ok := containerConfig["livenessProbe"].(map[string]interface{}); ok {
			container["livenessProbe"] = livenessProbe
		}
		
		if readinessProbe, ok := containerConfig["readinessProbe"].(map[string]interface{}); ok {
			container["readinessProbe"] = readinessProbe
		}
	}

	spec := map[string]interface{}{
		"containers": containers,
	}

	// Add volumes if specified
	if volumes, ok := config["volumes"].([]interface{}); ok {
		spec["volumes"] = volumes
	}

	// Add service account if specified
	if serviceAccount, ok := config["serviceAccount"].(string); ok {
		spec["serviceAccountName"] = serviceAccount
	}

	return spec
}

func (p *ManifestProcessor) validateDeployment(manifest *unstructured.Unstructured) error {
	spec, ok := manifest.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("deployment spec is required")
	}

	replicas, ok := spec["replicas"].(int64)
	if !ok || replicas < 0 {
		return fmt.Errorf("valid replicas is required")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("deployment template is required")
	}

	podSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("pod spec is required")
	}

	containers, ok := podSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return fmt.Errorf("at least one container is required")
	}

	return nil
}

func (p *ManifestProcessor) validateService(manifest *unstructured.Unstructured) error {
	spec, ok := manifest.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("service spec is required")
	}

	ports, ok := spec["ports"].([]interface{})
	if !ok || len(ports) == 0 {
		return fmt.Errorf("at least one port is required")
	}

	return nil
}

func (p *ManifestProcessor) validateConfigMap(manifest *unstructured.Unstructured) error {
	_, hasData := manifest.Object["data"]
	_, hasBinaryData := manifest.Object["binaryData"]
	
	if !hasData && !hasBinaryData {
		return fmt.Errorf("configMap must have data or binaryData")
	}

	return nil
}

func (p *ManifestProcessor) validateSecret(manifest *unstructured.Unstructured) error {
	_, hasData := manifest.Object["data"]
	_, hasStringData := manifest.Object["stringData"]
	
	if !hasData && !hasStringData {
		return fmt.Errorf("secret must have data or stringData")
	}

	return nil
}

func (p *ManifestProcessor) validateIngress(manifest *unstructured.Unstructured) error {
	spec, ok := manifest.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("ingress spec is required")
	}

	rules, ok := spec["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	return nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// ManifestOptions provides options for manifest generation
type ManifestOptions struct {
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	Replicas    int32
	Image       string
	Ports       []PortSpec
	Env         []EnvVar
	Resources   ResourceRequirements
	Volumes     []VolumeSpec
}

type PortSpec struct {
	Name          string
	ContainerPort int32
	Protocol      string
}

type EnvVar struct {
	Name  string
	Value string
}

type ResourceRequirements struct {
	Requests map[string]string
	Limits   map[string]string
}

type VolumeSpec struct {
	Name      string
	MountPath string
	ConfigMap *corev1.ConfigMapVolumeSource
	Secret    *corev1.SecretVolumeSource
	EmptyDir  *corev1.EmptyDirVolumeSource
}