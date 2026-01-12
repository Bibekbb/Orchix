package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the API client for interacting with Orchix API
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
	headers    map[string]string
}

// ClientOptions configures the API client
type ClientOptions struct {
	BaseURL    string
	AuthToken  string
	Timeout    time.Duration
	Headers    map[string]string
	HTTPClient *http.Client
}

// NewClient creates a new Orchix API client
func NewClient(options ClientOptions) *Client {
	if options.BaseURL == "" {
		options.BaseURL = "http://localhost:8080"
	}

	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}

	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{
			Timeout: options.Timeout,
		}
	}

	return &Client{
		baseURL:    options.BaseURL,
		httpClient: options.HTTPClient,
		authToken:  options.AuthToken,
		headers:    options.Headers,
	}
}

// HealthCheck checks the health of the Orchix API
func (c *Client) HealthCheck(ctx context.Context) (*HealthResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}

	var response HealthResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListDeployments retrieves all deployments
func (c *Client) ListDeployments(ctx context.Context) (*DeploymentsResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/api/v1/deployments", nil)
	if err != nil {
		return nil, err
	}

	var response DeploymentsResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetDeployment retrieves a specific deployment by ID
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/v1/deployments/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var response Deployment
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateDeployment creates a new deployment
func (c *Client) CreateDeployment(ctx context.Context, req *CreateDeploymentRequest) (*Deployment, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, "POST", "/api/v1/deployments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var response Deployment
	if err := c.do(httpReq, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateDeployment updates an existing deployment
func (c *Client) UpdateDeployment(ctx context.Context, id string, req *UpdateDeploymentRequest) (*Deployment, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, "PUT", fmt.Sprintf("/api/v1/deployments/%s", id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var response Deployment
	if err := c.do(httpReq, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteDeployment deletes a deployment
func (c *Client) DeleteDeployment(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/deployments/%s", id), nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

// GetDeploymentStatus retrieves the status of a deployment
func (c *Client) GetDeploymentStatus(ctx context.Context, id string) (*DeploymentStatus, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/v1/deployments/%s/status", id), nil)
	if err != nil {
		return nil, err
	}

	var response DeploymentStatus
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetDeploymentLogs retrieves logs for a deployment
func (c *Client) GetDeploymentLogs(ctx context.Context, id string, opts *LogOptions) (*LogsResponse, error) {
	url := fmt.Sprintf("/api/v1/deployments/%s/logs", id)
	if opts != nil {
		// Add query parameters
		// TODO: Implement query params
	}

	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var response LogsResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetMetrics retrieves metrics from the Orchix API
func (c *Client) GetMetrics(ctx context.Context) (*MetricsResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/api/v1/metrics", nil)
	if err != nil {
		return nil, err
	}

	var response MetricsResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ExecuteDeployment executes a deployment from a manifest
func (c *Client) ExecuteDeployment(ctx context.Context, req *ExecuteDeploymentRequest) (*DeploymentExecution, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, "POST", "/api/v1/deploy/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var response DeploymentExecution
	if err := c.do(httpReq, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// StreamDeploymentLogs streams logs from a deployment
func (c *Client) StreamDeploymentLogs(ctx context.Context, id string, opts *LogOptions) (<-chan LogEntry, error) {
	url := fmt.Sprintf("/api/v1/deployments/%s/logs/stream", id)
	if opts != nil {
		// Add query parameters
	}

	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	logChan := make(chan LogEntry, 100)
	go c.streamLogs(resp.Body, logChan)

	return logChan, nil
}

func (c *Client) streamLogs(body io.ReadCloser, logChan chan<- LogEntry) {
	defer body.Close()
	defer close(logChan)

	decoder := json.NewDecoder(body)
	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			if err != io.EOF {
				// Send error entry
				logChan <- LogEntry{
					Timestamp: time.Now(),
					Level:     "error",
					Message:   fmt.Sprintf("Failed to decode log: %v", err),
				}
			}
			return
		}
		logChan <- entry
	}
}

// newRequest creates a new HTTP request
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Set default headers
	req.Header.Set("Accept", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// Set custom headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// do executes an HTTP request and handles the response
func (c *Client) do(req *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// WithAuthToken sets the authentication token
func (c *Client) WithAuthToken(token string) *Client {
	c.authToken = token
	return c
}

// WithHeader adds a custom header
func (c *Client) WithHeader(key, value string) *Client {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
	return c
}

// WithTimeout sets a custom timeout
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	c.httpClient.Timeout = timeout
	return c
}

// APIError represents an API error response
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %d - %s", e.StatusCode, e.Message)
}