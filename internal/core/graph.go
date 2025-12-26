package core

import (
	"fmt"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// DependencyGraph manages component dependencies
type DependencyGraph struct {
	nodes map[string]*types.Component
	edges map[string][]string // adjacency list
}

// NewDependencyGraph creates a new graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*types.Component),
		edges: make(map[string][]string),
	}
}

// AddNode adds a component to the graph
func (g *DependencyGraph) AddNode(id string, comp types.Component) {
	g.nodes[id] = &comp
}

// AddEdge adds a dependency edge
func (g *DependencyGraph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
}

// GetExecutionOrder returns the deployment order
func (g *DependencyGraph) GetExecutionOrder() ([][]string, error) {
	// Calculate in-degrees
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}

	// Count incoming edges
	for _, neighbors := range g.edges {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}

	// Find nodes with 0 in-degree (no dependencies)
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var result [][]string

	// Process in levels (topological sort)
	for len(queue) > 0 {
		levelSize := len(queue)
		currentLevel := make([]string, 0, levelSize)

		for i := 0; i < levelSize; i++ {
			node := queue[0]
			queue = queue[1:]
			currentLevel = append(currentLevel, node)

			// Reduce in-degree of neighbors
			for _, neighbor := range g.edges[node] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					queue = append(queue, neighbor)
				}
			}
		}

		result = append(result, currentLevel)
	}

	// Check for cycles
	for _, degree := range inDegree {
		if degree > 0 {
			return nil, fmt.Errorf("cyclic dependency detected")
		}
	}

	return result, nil
}
