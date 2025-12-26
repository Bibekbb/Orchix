package core

import (
	"context"

	
	"github.com/Bibekbb/Orchix/pkg/types"
	"github.com/Bibekbb/Orchix/internal/providers"
)

type Engine struct {
	manifest	*types.Manifest
	state		StateManager
	providers map[string]providers.Provider
	logger		Logger
}

func NewEngine(manifest *types.Manifest) (*Engine, error) {
	engine := &Engine{
		manifest:	manifest,
		state:		NewFileStateManager(),
		providers:	make(map[string]providers.Provider),
		logger: NewLogger(),
	}

	// Initialize providers
	if err := engine.initProviders(); err != nil {
		return  nil, err
	}

	return  engine, nil
}

func (e *Engine) Deploy(ctx context.Context, dryRun bool) error {
	// 1. Build dependency graph
	graph, err := e.buildDependencyGraph()
	if err != nil {
		return  err
	}

	// 2. Generate execution plan
	plan, err := graph.GetExecutionOrder()
	if err != nil {
		return  err
	}

	if dryRun {
		return  e.printPlan(plan)
	}

	// 3. Execute plan
	return  e.executePlan(ctx, plan)
}