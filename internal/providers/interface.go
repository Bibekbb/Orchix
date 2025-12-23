package providers

import (
	"context"

	"github.com/Bibekbb/Orchix/pkg/types"
)

type Provider interface {
	Name() string
	Plan(ctx context.Context, comp types.Component, variables map[string]string) (*PlanResult, error)
	Apply(ctx context.Context, comp types.Component, variables map[string]string) (*ApplyResult, error)
	Destroy(ctx context.Context, comp types.Component, variables map[string]string) error
	GetStatus(ctx context.Context, comp types.Component) (*ComponentStatus, error)
}

type PlanResult struct {
	Changes []Change          `json:"changes"`
	Outputs map[string]string `json:"outputs,omitempty"`
}

type ApplyResult struct {
	Outputs map[string]string `json:"outputs"`
}

type ComponentStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type Change struct {
	Type    ChangeType  `json:"type"`
	Address string      `json:"address"`
	From    interface{} `json:"from,omitempty"`
	To      interface{} `json:"to,omitempty"`
}

type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
)
