package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

type Engine struct {
	manifest     *types.Manifest
	stateManager StateManager
	providers    map[types.ComponentType]providers.Provider
	logger       *log.Logger
}

func NewEngine(manifest *types.Manifest) *Engine {
	return &Engine{
		manifest:     manifest,
		stateManager: NewFileStateManager(".Orchix/state.json"),
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
		return e.printPlan(executionPlan)
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

		for _, componentID := range stage {
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

				e.logger.Printf("Deploying component: %s", comp.Name)

				result, err := provider.Apply(ctx, *comp, e.manifest.Variables)
				if err != nil {
					errors <- fmt.Errorf("failed to deploy %s: %w", comp.Name, err)
					return
				}

				// Update state
				e.stateManager.SetComponentState(cid, types.ComponentState{
					Status:    types.StateDeployed,
					Outputs:   result.Outputs,
					Timestamp: time.Now(),
				})

				// Health check
				if comp.HealthCheck != nil {
					if err := e.waitForHealthy(ctx, comp); err != nil {
						errors <- fmt.Errorf("health check failed for %s: %w", comp.Name, err)
					}
				}
			}(componentID)
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

func (e *Engine) waitForHealthy(ctx context.Context, comp *types.Component) error {
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
				continue
			}
			if status.Healthy {
				return nil
			}
		}
	}
}

func (e *Engine) printPlan(plan [][]string) error {
	e.logger.Println("Execution Plan:")
	for i, stage := range plan {
		e.logger.Printf("Stage %d:", i+1)
		for _, compID := range stage {
			comp := e.getComponentByID(compID)
			if comp != nil {
				e.logger.Printf("  - %s (%s)", comp.Name, comp.Type)
			}
		}
	}
	return nil
}

func (e *Engine) RegisterProvider(componentType types.ComponentType, provider providers.Provider) {
	e.providers[componentType] = provider
}

func (e *Engine) Destroy(ctx context.Context) error {
	// Get execution plan in reverse order
	graph, err := e.buildDependencyGraph()
	if err != nil {
		return err
	}

	plan, err := graph.GetExecutionOrder()
	if err != nil {
		return err
	}

	// Reverse the plan for destruction
	for i, j := 0, len(plan)-1; i < j; i, j = i+1, j-1 {
		plan[i], plan[j] = plan[j], plan[i]
	}

	// Execute destroy in reverse order
	for _, stage := range plan {
		var wg sync.WaitGroup
		errors := make(chan error, len(stage))

		for _, componentID := range stage {
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

				e.logger.Printf("Destroying component: %s", comp.Name)

				err := provider.Destroy(ctx, *comp, e.manifest.Variables)
				if err != nil {
					errors <- fmt.Errorf("failed to destroy %s: %w", comp.Name, err)
					return
				}

				// Update state
				e.stateManager.SetComponentState(cid, types.ComponentState{
					Status:    types.StateDestroyed,
					Timestamp: time.Now(),
				})
			}(componentID)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			if err != nil {
				return fmt.Errorf("stage failed: %w", err)
			}
		}
	}

	return nil
}

func (e *Engine) Status(ctx context.Context) error {
	states, err := e.stateManager.GetAllStates()
	if err != nil {
		return err
	}

	e.logger.Println("Deployment Status:")
	for id, state := range states {
		comp := e.getComponentByID(id)
		name := id
		if comp != nil {
			name = comp.Name
		}
		e.logger.Printf("  %s: %s", name, state.Status)
	}

	return nil
}

func (e *Engine) getComponentByID(id string) *types.Component {
	for _, comp := range e.manifest.Components {
		if comp.ID == id {
			return &comp
		}
	}
	return nil
}

func setupLogger() *log.Logger {
	return log.New(log.Writer(), "[Orchix] ", log.LstdFlags)
}
