package api

import (
	"time"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime,omitempty"`
	Services  []Service `json:"services,omitempty"`
}

// Service represents a service status
type Service struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// DeploymentsResponse represents a list of deployments
type DeploymentsResponse struct {
	Deployments []Deployment `json:"deployments"`
	Total       int          `json:"total"`
	Page        int          `json:"page,omitempty"`
	PageSize    int          `json:"page_size,omitempty"`
}

// Deployment represents a deployment
type Deployment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Manifest    string            `json:"manifest,omitempty"`
	Status      string            `json:"status"`
	Environment string            `json:"environment"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
	Version     string            `json:"version,omitempty"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	DeploymentID string            `json:"deployment_id"`
	Status       string            `json:"status"`
	Phase        string            `json:"phase"`
	Message      string            `json:"message"`
	Progress     int               `json:"progress"` // 0-100
	Components   []ComponentStatus `json:"components,omitempty"`
	Errors       []string          `json:"errors,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// ComponentStatus represents the status of a component
type ComponentStatus struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Status     string            `json:"status"`
	Health     string            `json:"health"` // healthy, unhealthy, unknown
	Message    string            `json:"message,omitempty"`
	Conditions []Condition       `json:"conditions,omitempty"`
	Resources  []ResourceStatus  `json:"resources,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Condition represents a component condition
type Condition struct {
	Type    string    `json:"type"`
	Status  string    `json:"status"`
	Reason  string    `json:"reason,omitempty"`
	Message string    `json:"message,omitempty"`
	LastUpdate time.Time `json:"last_update"`
}

// ResourceStatus represents the status of a resource
type ResourceStatus struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Status    string            `json:"status"`
	Ready     bool              `json:"ready"`
	Message   string            `json:"message,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// CreateDeploymentRequest represents a request to create a deployment
type CreateDeploymentRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Manifest    string            `json:"manifest"`
	Environment string            `json:"environment"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Secrets     map[string]string `json:"secrets,omitempty"`
	DryRun      bool              `json:"dry_run,omitempty"`
	Wait        bool              `json:"wait,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
}

// UpdateDeploymentRequest represents a request to update a deployment
type UpdateDeploymentRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Manifest    *string           `json:"manifest,omitempty"`
	Status      *string           `json:"status,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ExecuteDeploymentRequest represents a request to execute a deployment
type ExecuteDeploymentRequest struct {
	ManifestPath string            `json:"manifest_path"`
	Target       string            `json:"target"`
	Variables    map[string]string `json:"variables,omitempty"`
	Secrets      map[string]string `json:"secrets,omitempty"`
	DryRun       bool              `json:"dry_run"`
	Force        bool              `json:"force,omitempty"`
	Wait         bool              `json:"wait,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
}

// DeploymentExecution represents the execution of a deployment
type DeploymentExecution struct {
	ExecutionID string    `json:"execution_id"`
	DeploymentID string  `json:"deployment_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	LogsURL     string    `json:"logs_url,omitempty"`
	StatusURL   string    `json:"status_url,omitempty"`
}

// LogsResponse represents a response with logs
type LogsResponse struct {
	DeploymentID string    `json:"deployment_id"`
	Logs         []LogEntry `json:"logs"`
	Total        int       `json:"total"`
	From         time.Time `json:"from,omitempty"`
	To           time.Time `json:"to,omitempty"`
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // debug, info, warn, error
	Component string    `json:"component,omitempty"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"` // stdout, stderr, system
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// LogOptions represents options for retrieving logs
type LogOptions struct {
	Since   *time.Time `json:"since,omitempty"`
	Until   *time.Time `json:"until,omitempty"`
	Level   string     `json:"level,omitempty"`
	Component string   `json:"component,omitempty"`
	Limit   int        `json:"limit,omitempty"`
	Follow  bool       `json:"follow,omitempty"`
	Tail    int        `json:"tail,omitempty"`
}

// MetricsResponse represents metrics from the Orchix API
type MetricsResponse struct {
	Timestamp time.Time      `json:"timestamp"`
	Metrics   []Metric       `json:"metrics"`
	Summary   MetricsSummary `json:"summary"`
}

// Metric represents a single metric
type Metric struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"` // counter, gauge, histogram
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// MetricsSummary represents a summary of metrics
type MetricsSummary struct {
	TotalDeployments    int `json:"total_deployments"`
	RunningDeployments  int `json:"running_deployments"`
	FailedDeployments   int `json:"failed_deployments"`
	SuccessfulDeployments int `json:"successful_deployments"`
	TotalComponents     int `json:"total_components"`
	ActiveComponents    int `json:"active_components"`
	FailedComponents    int `json:"failed_components"`
	Uptime              string `json:"uptime"`
	ErrorRate           float64 `json:"error_rate"`
	SuccessRate         float64 `json:"success_rate"`
}

// WebhookEvent represents a webhook event
type WebhookEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // deployment, status, error, etc.
	Source    string            `json:"source"` // github, gitlab, manual, etc.
	Payload   interface{}       `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Signature string            `json:"signature,omitempty"`
}

// Notification represents a notification
type Notification struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // info, warning, error, success
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Severity  string            `json:"severity"` // low, medium, high, critical
	Source    string            `json:"source"`
	Timestamp time.Time         `json:"timestamp"`
	Read      bool              `json:"read"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Pagination represents pagination information
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
	Pages    int `json:"pages"`
}

// SortOption represents sorting options
type SortOption struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // asc, desc
}

// FilterOption represents filtering options
type FilterOption struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, etc.
	Value    interface{} `json:"value"`
}

// ListOptions represents options for listing resources
type ListOptions struct {
	Page     int            `json:"page,omitempty"`
	PageSize int            `json:"page_size,omitempty"`
	Sort     []SortOption   `json:"sort,omitempty"`
	Filter   []FilterOption `json:"filter,omitempty"`
	Search   string         `json:"search,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}