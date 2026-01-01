package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// Plan represents a deployment plan
type Plan struct {
	ID           string          `json:"id"`
	CreatedAt    time.Time       `json:"createdAt"`
	Manifest     *types.Manifest `json:"manifest"`
	Stages       []Stage         `json:"stages"`
	TotalChanges int             `json:"totalChanges"`
	Summary      PlanSummary     `json:"summary"`
}

// Stage represents a deployment stage
type Stage struct {
	Number      int              `json:"number"`
	Components  []ComponentPlan  `json:"components"`
	Description string           `json:"description"`
	CanParallel bool             `json:"canParallel"`
}

// ComponentPlan represents a component in the plan
type ComponentPlan struct {
	Component  types.Component `json:"component"`
	Action     Action          `json:"action"`
	Changes    []Change        `json:"changes,omitempty"`
	Status     PlanStatus      `json:"status"`
	EstimatedDuration time.Duration `json:"estimatedDuration,omitempty"`
}

// PlanSummary summarizes the deployment plan
type PlanSummary struct {
	TotalComponents int            `json:"totalComponents"`
	TotalStages     int            `json:"totalStages"`
	CreateCount     int            `json:"createCount"`
	UpdateCount     int            `json:"updateCount"`
	DeleteCount     int            `json:"deleteCount"`
	NoChangeCount   int            `json:"noChangeCount"`
}

// Action represents the action to perform
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionNoop   Action = "noop"
)

// PlanStatus represents the plan status
type PlanStatus string

const (
	StatusPending   PlanStatus = "pending"
	StatusPlanned   PlanStatus = "planned"
	StatusExecuting PlanStatus = "executing"
	StatusSucceeded PlanStatus = "succeeded"
	StatusFailed    PlanStatus = "failed"
	StatusSkipped   PlanStatus = "skipped"
)

// Change represents a change to be made
type Change struct {
	Type    ChangeType `json:"type"`
	Address string     `json:"address"`
	From    any        `json:"from,omitempty"`
	To      any        `json:"to,omitempty"`
	Reason  string     `json:"reason,omitempty"`
}

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeRead   ChangeType = "read"
)

// Planner creates deployment plans
type Planner struct {
	manifest *types.Manifest
	state    StateManager
}

// NewPlanner creates a new planner
func NewPlanner(manifest *types.Manifest, state StateManager) *Planner {
	return &Planner{
		manifest: manifest,
		state:    state,
	}
}

// CreatePlan creates a deployment plan
func (p *Planner) CreatePlan(ctx context.Context, operation string) (*Plan, error) {
	planID := fmt.Sprintf("plan-%d", time.Now().Unix())
	
	plan := &Plan{
		ID:        planID,
		CreatedAt: time.Now(),
		Manifest:  p.manifest,
		Summary:   PlanSummary{},
	}
	
	// Validate manifest first
	validator := NewValidator(p.manifest)
	if err := validator.ValidateStrict(); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}
	
	// Build dependency graph
	graph, err := p.buildDependencyGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}
	
	// Get execution order
	executionOrder, err := graph.GetExecutionOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to get execution order: %w", err)
	}
	
	// Create stages from execution order
	plan.Stages = p.createStages(executionOrder, operation)
	
	// Calculate summary
	plan.calculateSummary()
	
	return plan, nil
}

// createStages creates deployment stages from execution order
func (p *Planner) createStages(executionOrder [][]string, operation string) []Stage {
	var stages []Stage
	
	for stageNum, stageComponents := range executionOrder {
		stage := Stage{
			Number:      stageNum + 1,
			CanParallel: len(stageComponents) > 1,
			Description: p.getStageDescription(stageNum, len(stageComponents), operation),
		}
		
		for _, compID := range stageComponents {
			// Find the component
			var component types.Component
			for _, comp := range p.manifest.Components {
				if comp.ID == compID {
					component = comp
					break
				}
			}
			
			// Determine action based on operation
			action := p.determineAction(component, operation)
			
			// Create component plan
			compPlan := ComponentPlan{
				Component:  component,
				Action:     action,
				Status:     StatusPending,
				EstimatedDuration: p.estimateDuration(component),
			}
			
			// Add changes based on action
			compPlan.Changes = p.generateChanges(component, action)
			
			stage.Components = append(stage.Components, compPlan)
		}
		
		stages = append(stages, stage)
	}
	
	return stages
}

// getStageDescription generates a description for the stage
func (p *Planner) getStageDescription(stageNum, componentCount int, operation string) string {
	operations := map[string]string{
		"deploy":   "Deploying",
		"destroy":  "Destroying",
		"update":   "Updating",
		"validate": "Validating",
	}
	
	op := operations[operation]
	if op == "" {
		op = "Processing"
	}
	
	if componentCount == 1 {
		return fmt.Sprintf("%s independent component", op)
	}
	
	stageNames := []string{
		"infrastructure",
		"foundational services",
		"core application",
		"dependent services",
		"edge services",
		"monitoring and logging",
	}
	
	if stageNum < len(stageNames) {
		return fmt.Sprintf("%s %s", op, stageNames[stageNum])
	}
	
	return fmt.Sprintf("%s stage %d", op, stageNum+1)
}

// determineAction determines the action for a component
func (p *Planner) determineAction(component types.Component, operation string) Action {
	switch operation {
	case "deploy":
		// Check if component exists in state
		if p.state.ComponentExists(component.ID) {
			return ActionUpdate
		}
		return ActionCreate
	case "destroy":
		return ActionDelete
	case "update":
		return ActionUpdate
	default:
		return ActionNoop
	}
}

// estimateDuration estimates deployment duration for a component
func (p *Planner) estimateDuration(component types.Component) time.Duration {
	// Base durations based on component type
	baseDurations := map[string]time.Duration{
		"docker":      30 * time.Second,
		"kubernetes":  45 * time.Second,
		"terraform":   60 * time.Second,
		"helm":        90 * time.Second,
		"cloudformation": 120 * time.Second,
	}
	
	duration, ok := baseDurations[component.Type]
	if !ok {
		duration = 60 * time.Second
	}
	
	// Adjust based on dependencies
	if len(component.DependsOn) > 0 {
		duration += time.Duration(len(component.DependsOn)) * 5 * time.Second
	}
	
	return duration
}

// generateChanges generates changes for a component
func (p *Planner) generateChanges(component types.Component, action Action) []Change {
	var changes []Change
	
	switch action {
	case ActionCreate:
		changes = append(changes, Change{
			Type:    ChangeTypeCreate,
			Address: fmt.Sprintf("%s.%s", component.Type, component.ID),
			To:      fmt.Sprintf("Create %s component: %s", component.Type, component.Name),
			Reason:  "New component deployment",
		})
		
	case ActionUpdate:
		changes = append(changes, Change{
			Type:    ChangeTypeUpdate,
			Address: fmt.Sprintf("%s.%s", component.Type, component.ID),
			From:    "existing configuration",
			To:      "updated configuration",
			Reason:  "Configuration update",
		})
		
	case ActionDelete:
		changes = append(changes, Change{
			Type:    ChangeTypeDelete,
			Address: fmt.Sprintf("%s.%s", component.Type, component.ID),
			From:    "existing deployment",
			To:      nil,
			Reason:  "Component removal",
		})
		
	case ActionNoop:
		changes = append(changes, Change{
			Type:    ChangeTypeRead,
			Address: fmt.Sprintf("%s.%s", component.Type, component.ID),
			Reason:  "No changes required",
		})
	}
	
	// Add variable changes if any
	if len(component.Variables) > 0 {
		for key, value := range component.Variables {
			changes = append(changes, Change{
				Type:    ChangeTypeUpdate,
				Address: fmt.Sprintf("%s.%s.variables.%s", component.Type, component.ID, key),
				To:      value,
				Reason:  "Variable configuration",
			})
		}
	}
	
	return changes
}

// buildDependencyGraph builds a dependency graph from manifest
func (p *Planner) buildDependencyGraph() (*DependencyGraph, error) {
	graph := NewDependencyGraph()
	
	for _, comp := range p.manifest.Components {
		graph.AddNode(comp.ID, comp)
		for _, dep := range comp.DependsOn {
			graph.AddEdge(dep, comp.ID)
		}
	}
	
	return graph, nil
}

// calculateSummary calculates plan summary
func (p *Plan) calculateSummary() {
	p.Summary.TotalComponents = 0
	p.Summary.TotalStages = len(p.Stages)
	
	for _, stage := range p.Stages {
		p.Summary.TotalComponents += len(stage.Components)
		
		for _, comp := range stage.Components {
			switch comp.Action {
			case ActionCreate:
				p.Summary.CreateCount++
				p.TotalChanges++
			case ActionUpdate:
				p.Summary.UpdateCount++
				p.TotalChanges++
			case ActionDelete:
				p.Summary.DeleteCount++
				p.TotalChanges++
			case ActionNoop:
				p.Summary.NoChangeCount++
			}
		}
	}
}

// Print prints the plan in human-readable format
func (p *Plan) Print() {
	fmt.Println("📋 Deployment Plan")
	fmt.Println("==================")
	fmt.Printf("Plan ID: %s\n", p.ID)
	fmt.Printf("Application: %s\n", p.Manifest.AppName)
	fmt.Printf("Target: %s\n", p.Manifest.Target)
	fmt.Printf("Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	
	fmt.Println("📊 Summary:")
	fmt.Printf("  • Total Components: %d\n", p.Summary.TotalComponents)
	fmt.Printf("  • Total Stages: %d\n", p.Summary.TotalStages)
	fmt.Printf("  • To Create: %d\n", p.Summary.CreateCount)
	fmt.Printf("  • To Update: %d\n", p.Summary.UpdateCount)
	fmt.Printf("  • To Delete: %d\n", p.Summary.DeleteCount)
	fmt.Printf("  • No Changes: %d\n", p.Summary.NoChangeCount)
	fmt.Println()
	
	for _, stage := range p.Stages {
		fmt.Printf("Stage %d: %s\n", stage.Number, stage.Description)
		if stage.CanParallel && len(stage.Components) > 1 {
			fmt.Println("  (Components can be deployed in parallel)")
		}
		
		for _, comp := range stage.Components {
			actionIcon := getActionIcon(comp.Action)
			fmt.Printf("  %s %s (%s)\n", actionIcon, comp.Component.Name, comp.Component.Type)
			
			if len(comp.Changes) > 0 {
				for _, change := range comp.Changes {
					changeIcon := getChangeIcon(change.Type)
					fmt.Printf("    %s %s\n", changeIcon, change.Address)
					if change.Reason != "" {
						fmt.Printf("      Reason: %s\n", change.Reason)
					}
				}
			}
			
			if comp.EstimatedDuration > 0 {
				fmt.Printf("    ⏱️  Estimated: %v\n", comp.EstimatedDuration)
			}
			fmt.Println()
		}
	}
	
	fmt.Println("✅ Plan is valid and ready for execution")
	fmt.Println("Run 'orchix deploy' to apply this plan")
}

// getActionIcon returns icon for action
func getActionIcon(action Action) string {
	switch action {
	case ActionCreate:
		return "🆕"
	case ActionUpdate:
		return "🔄"
	case ActionDelete:
		return "🗑️"
	case ActionNoop:
		return "✅"
	default:
		return "📝"
	}
}

// getChangeIcon returns icon for change type
func getChangeIcon(changeType ChangeType) string {
	switch changeType {
	case ChangeTypeCreate:
		return "➕"
	case ChangeTypeUpdate:
		return "📝"
	case ChangeTypeDelete:
		return "➖"
	case ChangeTypeRead:
		return "👁️"
	default:
		return "📋"
	}
}

// ExecutePlan executes the deployment plan
func (p *Planner) ExecutePlan(ctx context.Context, plan *Plan, dryRun bool) error {
	logger := NewLogger()
	
	if dryRun {
		logger.Info("Dry run mode - showing execution plan only")
		plan.Print()
		return nil
	}
	
	logger.Info("Starting deployment execution")
	logger.Infof("Plan: %s", plan.ID)
	logger.Infof("Application: %s", plan.Manifest.AppName)
	logger.Infof("Target: %s", plan.Manifest.Target)
	
	totalStages := len(plan.Stages)
	
	for stageIdx, stage := range plan.Stages {
		logger.Infof("Executing Stage %d/%d: %s", stage.Number, totalStages, stage.Description)
		
		if stage.CanParallel {
			logger.Info("Components in this stage will run in parallel")
		}
		
		// TODO: Implement actual execution logic
		// This would call the appropriate provider for each component
		
		for compIdx, compPlan := range stage.Components {
			logger.Infof("  [%d/%d] %s %s", 
				compIdx+1, len(stage.Components),
				getActionIcon(compPlan.Action),
				compPlan.Component.Name)
			
			// Simulate execution
			time.Sleep(500 * time.Millisecond)
			
			// Update status
			plan.Stages[stageIdx].Components[compIdx].Status = StatusSucceeded
			logger.Info("    ✅ Completed successfully")
		}
		
		logger.Infof("✅ Stage %d completed", stage.Number)
	}
	
	logger.Info("🎉 Deployment completed successfully!")
	
	return nil
}

// PlanToJSON converts plan to JSON string
func (p *Plan) PlanToJSON() (string, error) {
	return "", nil // Implement JSON marshaling
}

// PlanToYAML converts plan to YAML string
func (p *Plan) PlanToYAML() (string, error) {
	var output strings.Builder
	
	output.WriteString(fmt.Sprintf("# Deployment Plan: %s\n", p.ID))
	output.WriteString(fmt.Sprintf("# Application: %s\n", p.Manifest.AppName))
	output.WriteString(fmt.Sprintf("# Target: %s\n", p.Manifest.Target))
	output.WriteString(fmt.Sprintf("# Created: %s\n\n", p.CreatedAt.Format("2006-01-02 15:04:05")))
	
	output.WriteString("summary:\n")
	output.WriteString(fmt.Sprintf("  totalComponents: %d\n", p.Summary.TotalComponents))
	output.WriteString(fmt.Sprintf("  totalStages: %d\n", p.Summary.TotalStages))
	output.WriteString(fmt.Sprintf("  toCreate: %d\n", p.Summary.CreateCount))
	output.WriteString(fmt.Sprintf("  toUpdate: %d\n", p.Summary.UpdateCount))
	output.WriteString(fmt.Sprintf("  toDelete: %d\n", p.Summary.DeleteCount))
	output.WriteString(fmt.Sprintf("  noChange: %d\n\n", p.Summary.NoChangeCount))
	
	output.WriteString("stages:\n")
	for _, stage := range p.Stages {
		output.WriteString(fmt.Sprintf("  - stage: %d\n", stage.Number))
		output.WriteString(fmt.Sprintf("    description: \"%s\"\n", stage.Description))
		output.WriteString(fmt.Sprintf("    canParallel: %v\n", stage.CanParallel))
		output.WriteString("    components:\n")
		
		for _, comp := range stage.Components {
			output.WriteString(fmt.Sprintf("      - name: \"%s\"\n", comp.Component.Name))
			output.WriteString(fmt.Sprintf("        id: %s\n", comp.Component.ID))
			output.WriteString(fmt.Sprintf("        type: %s\n", comp.Component.Type))
			output.WriteString(fmt.Sprintf("        action: %s\n", comp.Action))
			output.WriteString(fmt.Sprintf("        status: %s\n", comp.Status))
			
			if len(comp.Changes) > 0 {
				output.WriteString("        changes:\n")
				for _, change := range comp.Changes {
					output.WriteString(fmt.Sprintf("          - type: %s\n", change.Type))
					output.WriteString(fmt.Sprintf("            address: %s\n", change.Address))
					if change.Reason != "" {
						output.WriteString(fmt.Sprintf("            reason: \"%s\"\n", change.Reason))
					}
				}
			}
		}
	}
	
	return output.String(), nil
}