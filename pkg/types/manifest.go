package types

type Manifest struct {
	APIVersion 	string					`yaml:"apiVersion"`
	AppName 	string					`yaml:"appName"`
	Target 		string					`yaml:"target"`
	Description string					`yaml:"description,omitempty"`
	Variables	map[string]interface{}	`yaml:"variables,omitempty"`
	Secrets		[]SecretRef				`yaml:"secrets,omitempty"`
	Components	[]Component				`yaml:"components"`
}

type Component struct {
	ID			string					`yaml:"id"`
	Name		string					`yaml:"name"`
	Type		string					`yaml:"type"`
	Source		string					`yaml:"source"`
	DependsOn	[]string				`yaml:"dependsOn,omitempty"`
	Variables	map[string]interface{}	`yaml:"variables,omitempty"`
	HealthCheck	*HealthCheck			`yaml:"healthCheck,omitempty"`
}

type HealthCheck struct{
	Type 		string					`yaml:"type"`
	Endpoint 	string					`yaml:"endpoint,omitempty"`
	Interval	string					`yaml:"interval,omitempty"`
	Timeout 	string					`yaml:"timeout,omitempty"`
}

type SecretRef struct {
	Name		string					`yaml:"name"`
	Source		string					`yaml:"source"`
	Key			string					`yaml:"key,omitempty"`
	Path		string					`yaml:"path,omitempty"`
}