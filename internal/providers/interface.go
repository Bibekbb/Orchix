package providers

import (
	"context"
	"orchestr8/internal/types"
)

type Provider interface {
	Name() string
	Plan(ctx context.Context, component types.Component, variables map[string]string) (*PlanResult, error)
	Apply(ctx context.context, comp types.Component, variables map[string]string) (*ApplyResult, error)
	Destroy(ctx context.Context, comp types.Component) (*ComponentStatus, error)
}

type PlanResult struct {
	Changes []Change 	`json:"changes"`
	Outputs map[string]string `json:"outputs, omitempty"`
}

type ApplyResult struct {
	Outputs map[string]string `json:"outputs"`
}

type ComponentStatus struct {
	Healthy bool `json:"healthy"`
	Message string `json:"message"`
}

type Change struct {
	Type        ChangeType 	`json:"type"`
	Address string 			`json:"address"`
	From 	interface{} 	`json:"from"`
	To 		interface{} 	`json:"to, omitempty"`
}

