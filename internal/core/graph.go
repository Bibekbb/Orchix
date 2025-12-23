package core

import (
	"fmt"

	"github.com/Bibekbb/Orchix/pkg/types"
)

type DependencyGraph struct {
	nodes map[string]*types.Component
	edges map[string][]string
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*types.Component),
		edges: make(map[string][]string),
	}
}

func (g *DependencyGraph) AddNode(id string, comp types.Component) {
	g.nodes[id] = &comp
}

func (g *DependencyGraph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
}

// Kahn's Algorithm for topological sort
func (g *DependencyGraph) GetExecutionOrder() ([][]string, error) {
	// Calculate in-degrees
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for _, deps := range g.edges {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	// Initialize queue with nodes having 0 in-degree
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var result [][]string

	// Process nodes level by level
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
			return nil, fmt.Errorf("dependency cycle detected")
		}
	}

	return result, nil
}
