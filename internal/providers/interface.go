package providers

import (
	"context"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// Provider defines the interface for all deployment providers
type Provider interface {
	Name() string
	Plan(ctx context.Context, comp types.Component) (PlanResult, error)
	Apply(ctx context.Context, comp types.Component) (ApplyResult, error)
	Destroy(ctx context.Context, comp types.Component) error
	Status(ctx context.Context, comp types.Component) (StatusResult, error)
}

// PlanResult contains information about planned changes
type PlanResult struct {
	Changes []Change          `json:"changes"`
	Outputs map[string]string `json:"outputs"`
}

// ApplyResult contains information about applied changes
type ApplyResult struct {
	Outputs map[string]string `json:"outputs"`
}

// StatusResult contains component status information
type StatusResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Healthy bool   `json:"healthy"`
}

// Change represents a single change to be made
type Change struct {
	Type    ChangeType  `json:"type"`
	Address string      `json:"address"`
	Before  interface{} `json:"before,omitempty"`
	After   interface{} `json:"after,omitempty"`
}

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
)
