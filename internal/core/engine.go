package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"orchestr8/pkg/types"
	"orchestr8/internal/providers"
)

type Engine struct {
	manifest *types.Manifest
	stateManager StateManager
	providers map[types.ComponentType]providers.Provider
	logger *log.Logger
}

func NewEngine(manifest *types.Manifest) *Engine {
	return &Engine{
		manifest:     manifest,
		stateManager: NewFileStateManager(".orchestr8/state.json"),
		providers:    make(map[types.ComponentType]providers.Provider),
		logger:       setupLogger(),
	}
}

func (e *Engine) Deploy(ctx context.Context, dryRun bool) error {
	// Build dependency graph 
	graph, err := e.buildDependencyGraph()
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Generate execution plan
	executionPlan, err := graph.GetExecutionOrder()
	if err != nil {
		return err
	}

	if dryRun {
		return e.logger.Println(executionPlan)
		return nil 
	}

	// Execute plan with concurrency
	return e.executePlan(ctx, executionPlan)
}

func (e *Engine) buildDependencyGraph() (*DependencyGraph, error) {
	graph := NewDependencyGraph()

	for _, comp := range e.manifest.Components {
		graph.AddNode(comp.ID, comp)
		for _, dep := range comp.DependsOn {
			graph.AddEdge(dep, comp.ID)
		}
	}
	return graph, nil
}

func (e *Engine) executePlan(ctx context.Context, plan [][]string) error {
	for _, stage := range plan {
		var wg sync.WaitGroup 
		errors := make(chan error, len(stage))

		for _, compID := range stage {
			wg.Add(1)
			go func(cid string) {
				defer wg.Done()

				comp := e.getComponentByID(cid)
				if comp == nil {
					errors <- fmt.Errorf("component not found: %s", cid)
					return
				}
				provider := e.providers[comp.Type]
				if provider == nil {
					errors <- fmt.Errorf("no provider for type: %s", comp.Type)
					return
				}

				// Update state
				e.stateManager.SetComponentState(cid, types.ComponentState{
					Status:		types.StateDeployed,
					Outputs:	result.Outputs,
					Timestamp:	time.Now(),
				})

				// Health Check
				if comp.HealthCheck != nil {
					if err := e.waitForHealthy(ctx, comp); err != nil {
						errors <- fmt.Errorf("health check failed for %s: %w", comp.Name, err)
					}
				}
			}(CompID)
		}

		wg.Wait()
		close(errors)

		// Check for errors in this stage
		for err := range errors {
			if err != nil {
				return fmt.Errorf("stage failed: %w", err)
			}
		}
	}

	return nil
}

func (e *Engine) waitForHealthy(ctx, context.Context, comp *types.Component) err {
	timeout := time.Duration(comp.HealthCheck.Timeout) * time.Second
	interval := time.Duration(comp.HealthCheck.Interval) * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			provider := e.providers[comp.Type]
			status, err := provider.GetStatus(ctx, *comp)
			if err != nil {
				contiue
			}
			if status.Healthy {
				return nil
			}
		}
	}
}