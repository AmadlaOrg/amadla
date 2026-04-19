package dag

import "fmt"

// Node represents an entity in the dependency graph.
type Node struct {
	URI      string   // Entity URI (e.g., "github.com/SomeOrg/nginx@v1.0.0")
	Requires []string // URIs this entity depends on (from _requires)
}

// Graph holds the dependency graph for topological sorting.
type Graph struct {
	nodes map[string]*Node
}

// New creates an empty Graph.
func New() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

// Add inserts a node into the graph.
func (g *Graph) Add(node Node) {
	g.nodes[node.URI] = &node
}

// Sort performs a topological sort on the graph.
// Returns the execution order (dependencies first) or an error if a cycle is detected.
func (g *Graph) Sort() ([]string, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool) // cycle detection
	var order []string

	var visit func(uri string) error
	visit = func(uri string) error {
		if inStack[uri] {
			return fmt.Errorf("circular _requires reference detected: %s", uri)
		}
		if visited[uri] {
			return nil
		}

		inStack[uri] = true

		node, exists := g.nodes[uri]
		if exists {
			for _, dep := range node.Requires {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		inStack[uri] = false
		visited[uri] = true
		order = append(order, uri)
		return nil
	}

	for uri := range g.nodes {
		if err := visit(uri); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// Nodes returns all node URIs in the graph.
func (g *Graph) Nodes() []string {
	uris := make([]string, 0, len(g.nodes))
	for uri := range g.nodes {
		uris = append(uris, uri)
	}
	return uris
}

// Get returns the node for a given URI, or nil if not found.
func (g *Graph) Get(uri string) *Node {
	return g.nodes[uri]
}

// Levels returns nodes grouped into parallel execution tiers.
// All nodes in a tier have their dependencies satisfied by earlier tiers.
// Nodes within the same tier are independent and can run concurrently.
func (g *Graph) Levels() ([][]string, error) {
	// Compute in-degree for each node (only counting edges within the graph)
	inDegree := make(map[string]int)
	for uri := range g.nodes {
		inDegree[uri] = 0
	}
	for _, node := range g.nodes {
		for _, dep := range node.Requires {
			if _, exists := g.nodes[dep]; exists {
				inDegree[node.URI]++
			}
		}
	}

	// BFS by tiers (Kahn's algorithm)
	var levels [][]string
	remaining := len(g.nodes)

	for remaining > 0 {
		// Collect nodes with zero in-degree
		var tier []string
		for uri, deg := range inDegree {
			if deg == 0 {
				tier = append(tier, uri)
			}
		}

		if len(tier) == 0 {
			return nil, fmt.Errorf("circular _requires reference detected")
		}

		// Remove this tier from the graph
		for _, uri := range tier {
			delete(inDegree, uri)
		}

		// Decrease in-degree for dependents
		for depURI := range inDegree {
			node := g.nodes[depURI]
			for _, req := range node.Requires {
				for _, done := range tier {
					if req == done {
						inDegree[depURI]--
					}
				}
			}
		}

		levels = append(levels, tier)
		remaining -= len(tier)
	}

	return levels, nil
}
