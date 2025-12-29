package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Bibekbb/Orchix/internal/providers"
	"github.com/Bibekbb/Orchix/pkg/types"
)

// Logger interface (placeholder - create logger.go)
type Logger interface {
	Printf(format string, args ...interface{})
}

// SimpleLogger implements Logger
type SimpleLogger struct{}

func (l *SimpleLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func NewLogger() Logger {
	return &SimpleLogger{}
}

type Engine struct {
	manifest  *types.Manifest
	state     StateManager
	providers map[string]providers.Provider
	logger    Logger
}

func NewEngine(manifest *types.Manifest) (*Engine, error) {
	engine := &Engine{
		manifest:  manifest,
		state:     NewStateManager(),
		providers: make(map[string]providers.Provider),
		logger:    NewLogger(),
	}

	// Initialize providers
	if err := engine.initProviders(); err != nil {
		return nil, err
	}

	return engine, nil
}

func (e *Engine) initProviders() error {
	// Initialize providers here
	// For now, just log
	e.logger.Printf("Initializing Providers...")
	return nil
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

func (e *Engine) printPlan(plan [][]string) error {
	fmt.Println("📋 Deployment Plan")
	fmt.Println("==================")
	fmt.Printf("Application: %s\n", e.manifest.AppName)
	fmt.Printf("Target: %s\n", e.manifest.Target)

	for i, stage := range plan {
		fmt.Printf("\nStage %d (parallel):\n", i+1)
		for _, compID := range stage {
			// Find Component
			for _, comp := range e.manifest.Components {
				if comp.ID == compID {
					fmt.Printf("	• %s (%s) from %s\n", comp.Name, comp.Type, comp.Source)
					break
				}
			}
		}
	}
	return nil
}

func (e *Engine) executePlan(ctx context.Context, plan [][]string) error {
	for StageIndex, stage := range plan {
		fmt.Printf("\n⚡ Executing Stage %d/%d\n", StageIndex+1, len(plan))

		var wg sync.WaitGroup
		errors := make(chan error, len(stage))

		for _, compID := range stage {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				// Find component
				var comp types.Component
				found := false
				for _, c := range e.manifest.Components {
					if c.ID == id {
						comp = c
						found = true
						break
					}
				}
				if !found {
					errors <- fmt.Errorf("Component %s not found", id)
					return
				}
				fmt.Printf("	Deploying %s...\n", comp.Name)
				time.Sleep(1 * time.Second) // Simulate Deployment

				fmt.Printf("	✅ %s deployed\n", comp.Name)
			}(compID)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) Deploy(ctx context.Context, dryRun bool) error {
	// 1. Build dependency graph
	graph, err := e.buildDependencyGraph()
	if err != nil {
		return err
	}

	// 2. Generate execution plan
	plan, err := graph.GetExecutionOrder()
	if err != nil {
		return err
	}

	if dryRun {
		return e.printPlan(plan)
	}

	// 3. Execute plan
	return e.executePlan(ctx, plan)
}

// func (e *Engine) Deploy(ctx context.Context, dryRun bool) error {
// 	// 1. Build dependency graph
// 	graph, err := e.buildDependencyGraph()
// 	if err != nil {
// 		return  err
// 	}

// 	// 2. Generate execution plan
// 	plan, err := graph.GetExecutionOrder()
// 	if err != nil {
// 		return  err
// 	}

// 	if dryRun {
// 		return  e.printPlan(plan)
// 	}

// 	// 3. Execute plan
// 	return  e.executePlan(ctx, plan)
// }
