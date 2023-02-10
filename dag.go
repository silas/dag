package dag

import (
	"fmt"
	"sort"
	"strings"
)

// AcyclicGraph is a specialization of Graph that cannot have cycles.
type AcyclicGraph struct {
	Graph
}

// WalkFunc is the callback used for walking the graph.
type WalkFunc func(Vertex) Diagnostics

// DepthWalkFunc is a walk function that also receives the current depth of the
// walk as an argument
type DepthWalkFunc func(Vertex, int) error

// BreadthWalkFun is a walk function that also receives the current depth of the
// walk as an argument
type BreadthWalkFunc func(Vertex, int) error

func (g *AcyclicGraph) DirectedGraph() Grapher {
	return g
}

// Returns a Set that includes every Vertex yielded by walking down from the
// provided starting Vertex v.
func (g *AcyclicGraph) Descendants(v Vertex) (Set, error) {
	s := make(Set)
	memoFunc := func(v Vertex, d int) error {
		s.Add(v)
		return nil
	}

	if err := g.DepthFirstWalk(g.downEdgesNoCopy(v), memoFunc); err != nil {
		return nil, err
	}

	return s, nil
}

// Returns a Set that includes every Vertex yielded by walking up from the
// provided starting Vertex v.
func (g *AcyclicGraph) Ancestors(v Vertex) (Set, error) {
	s := make(Set)
	memoFunc := func(v Vertex, d int) error {
		s.Add(v)
		return nil
	}

	if err := g.ReverseDepthFirstWalk(g.upEdgesNoCopy(v), memoFunc); err != nil {
		return nil, err
	}

	return s, nil
}

// Root returns the root of the DAG, or an error.
//
// Complexity: O(V)
func (g *AcyclicGraph) Root() (Vertex, error) {
	roots := make([]Vertex, 0, 1)
	for _, v := range g.Vertices() {
		if g.upEdgesNoCopy(v).Len() == 0 {
			roots = append(roots, v)
		}
	}

	if len(roots) > 1 {
		// TODO(mitchellh): make this error message a lot better
		return nil, fmt.Errorf("multiple roots: %#v", roots)
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("no roots found")
	}

	return roots[0], nil
}

// TransitiveReduction performs the transitive reduction of graph g in place.
// The transitive reduction of a graph is a graph with as few edges as
// possible with the same reachability as the original graph. This means
// that if there are three nodes A => B => C, and A connects to both
// B and C, and B connects to C, then the transitive reduction is the
// same graph with only a single edge between A and B, and a single edge
// between B and C.
//
// The graph must be free of cycles for this operation to behave properly.
//
// Complexity: O(V(V+E)), or asymptotically O(VE)
func (g *AcyclicGraph) TransitiveReduction() {
	// For each vertex u in graph g, do a DFS starting from each vertex
	// v such that the edge (u,v) exists (v is a direct descendant of u).
	//
	// For each v-prime reachable from v, remove the edge (u, v-prime).
	for _, u := range g.Vertices() {
		uTargets := g.downEdgesNoCopy(u)

		g.DepthFirstWalk(g.downEdgesNoCopy(u), func(v Vertex, d int) error {
			shared := uTargets.Intersection(g.downEdgesNoCopy(v))
			for _, vPrime := range shared {
				g.RemoveEdge(BasicEdge(u, vPrime))
			}

			return nil
		})
	}
}

// Validate validates the DAG. A DAG is valid if it has a single root
// with no cycles.
func (g *AcyclicGraph) Validate() error {
	if _, err := g.Root(); err != nil {
		return err
	}

	// Look for cycles of more than 1 component
	var diags Diagnostics
	cycles := g.Cycles()
	if len(cycles) > 0 {
		for _, cycle := range cycles {
			cycleStr := make([]string, len(cycle))
			for j, vertex := range cycle {
				cycleStr[j] = VertexName(vertex)
			}

			diags = diags.Append(fmt.Errorf(
				"Cycle: %s", strings.Join(cycleStr, ", ")))
		}
	}

	// Look for cycles to self
	for _, e := range g.Edges() {
		if hashcode(e.Source()) == hashcode(e.Target()) {
			diags = diags.Append(fmt.Errorf(
				"Self reference: %s", VertexName(e.Source())))
		}
	}

	return diags.Err()
}

// Cycles reports any cycles between graph nodes.
// Self-referencing nodes are not reported, and must be detected separately.
func (g *AcyclicGraph) Cycles() [][]Vertex {
	var cycles [][]Vertex
	for _, cycle := range StronglyConnected(&g.Graph) {
		if len(cycle) > 1 {
			cycles = append(cycles, cycle)
		}
	}
	return cycles
}

// Walk walks the graph, calling your callback as each node is visited.
// This will walk nodes in parallel if it can. The resulting diagnostics
// contains problems from all graphs visited, in no particular order.
func (g *AcyclicGraph) Walk(cb WalkFunc) Diagnostics {
	w := &Walker{Callback: cb, Reverse: true}
	w.Update(g)
	return w.Wait()
}

// simple convenience helper for converting a dag.Set to a []Vertex
func AsVertexList(s Set) []Vertex {
	vertexList := make([]Vertex, 0, len(s))
	for _, raw := range s {
		vertexList = append(vertexList, raw.(Vertex))
	}
	return vertexList
}

type vertexAtDepth struct {
	Vertex Vertex
	Depth  int
}

// DepthFirstWalk does a depth-first walk of the graph starting from
// the vertices in start.
// The algorithm used here does not do a complete topological sort. To ensure
// correct overall ordering run TransitiveReduction first.
func (g *AcyclicGraph) DepthFirstWalk(start Set, f DepthWalkFunc) error {
	seen := make(map[Vertex]struct{})
	frontier := make([]*vertexAtDepth, 0, len(start))
	for _, v := range start {
		frontier = append(frontier, &vertexAtDepth{
			Vertex: v,
			Depth:  0,
		})
	}
	for len(frontier) > 0 {
		// Pop the current vertex
		n := len(frontier)
		current := frontier[n-1]
		frontier = frontier[:n-1]

		// Check if we've seen this already and return...
		if _, ok := seen[hashcode(current.Vertex)]; ok {
			continue
		}
		seen[hashcode(current.Vertex)] = struct{}{}

		// Visit the current node
		if err := f(current.Vertex, current.Depth); err != nil {
			return err
		}

		for _, v := range g.downEdgesNoCopy(current.Vertex) {
			frontier = append(frontier, &vertexAtDepth{
				Vertex: v,
				Depth:  current.Depth + 1,
			})
		}
	}

	return nil
}

// BreadthFirstWalk does a breadth-first walk of the graph starting from
// the vertices in start.
func (g *AcyclicGraph) BreadthFirstWalk(start Set, f BreadthWalkFunc) error {
	seen := make(map[Vertex]struct{})
	frontier := make([]*vertexAtDepth, 0, len(start))
	for _, v := range start {
		frontier = append(frontier, &vertexAtDepth{
			Vertex: v,
			Depth:  0,
		})
	}

	for len(frontier) > 0 {
		n := len(frontier)

		for i := 0; i < n; i++ {
			current := frontier[i]

			// Check if we've seen this already and return...
			if _, ok := seen[hashcode(current.Vertex)]; ok {
				continue
			}
			seen[hashcode(current.Vertex)] = struct{}{}

			// Visit the nodes in frontier
			if err := f(current.Vertex, current.Depth); err != nil {
				return err
			}

			for _, v := range g.downEdgesNoCopy(current.Vertex) {
				// // if vertex is a subgraph, add vertexes to frontier
				// sg, ok := v.(Subgrapher)
				// if ok {
				// 	var graph *AcyclicGraph
				// 	switch grapherType := sg.Subgraph().DirectedGraph().(type) {
				// 	case *Graph:
				// 		panic("can't traverse *Graph, use *AcyclicGraph instead")
				// 	case *AcyclicGraph:
				// 		graph = grapherType
				// 	}
				// 	subraphRoot, err := graph.Root()
				// 	if err != nil {
				// 		panic("can't find root of subgraph")
				// 	}
				// 	graph.BreadthFirstWalk(Set{"root": subraphRoot}, func(v Vertex, depth int) error {
				// 		frontier = append(frontier, &vertexAtDepth{
				// 			Vertex: v,
				// 			Depth:  current.Depth + 1,
				// 		})
				// 		return nil
				// 	})
				// } else {
				frontier = append(frontier, &vertexAtDepth{
					Vertex: v,
					Depth:  current.Depth + 1,
				})
				// }
			}
		}

		frontier = frontier[n:]
	}

	return nil
}

// SortedDepthFirstWalk does a depth-first walk of the graph starting from
// the vertices in start, always iterating the nodes in a consistent order.
func (g *AcyclicGraph) SortedDepthFirstWalk(start []Vertex, f DepthWalkFunc) error {
	seen := make(map[Vertex]struct{})
	frontier := make([]*vertexAtDepth, len(start))
	for i, v := range start {
		frontier[i] = &vertexAtDepth{
			Vertex: v,
			Depth:  0,
		}
	}
	for len(frontier) > 0 {
		// Pop the current vertex
		n := len(frontier)
		current := frontier[n-1]
		frontier = frontier[:n-1]

		// Check if we've seen this already and return...
		if _, ok := seen[current.Vertex]; ok {
			continue
		}
		seen[current.Vertex] = struct{}{}

		// Visit the current node
		if err := f(current.Vertex, current.Depth); err != nil {
			return err
		}

		// Visit targets of this in a consistent order.
		targets := AsVertexList(g.downEdgesNoCopy(current.Vertex))
		sort.Sort(byVertexName(targets))

		for _, t := range targets {
			frontier = append(frontier, &vertexAtDepth{
				Vertex: t,
				Depth:  current.Depth + 1,
			})
		}
	}

	return nil
}

// ReverseDepthFirstWalk does a depth-first walk _up_ the graph starting from
// the vertices in start.
// The algorithm used here does not do a complete topological sort. To ensure
// correct overall ordering run TransitiveReduction first.
func (g *AcyclicGraph) ReverseDepthFirstWalk(start Set, f DepthWalkFunc) error {
	seen := make(map[Vertex]struct{})
	frontier := make([]*vertexAtDepth, 0, len(start))
	for _, v := range start {
		frontier = append(frontier, &vertexAtDepth{
			Vertex: v,
			Depth:  0,
		})
	}
	for len(frontier) > 0 {
		// Pop the current vertex
		n := len(frontier)
		current := frontier[n-1]
		frontier = frontier[:n-1]

		// Check if we've seen this already and return...
		if _, ok := seen[current.Vertex]; ok {
			continue
		}
		seen[current.Vertex] = struct{}{}

		for _, t := range g.upEdgesNoCopy(current.Vertex) {
			frontier = append(frontier, &vertexAtDepth{
				Vertex: t,
				Depth:  current.Depth + 1,
			})
		}

		// Visit the current node
		if err := f(current.Vertex, current.Depth); err != nil {
			return err
		}
	}

	return nil
}

// SortedReverseDepthFirstWalk does a depth-first walk _up_ the graph starting from
// the vertices in start, always iterating the nodes in a consistent order.
func (g *AcyclicGraph) SortedReverseDepthFirstWalk(start []Vertex, f DepthWalkFunc) error {
	seen := make(map[Vertex]struct{})
	frontier := make([]*vertexAtDepth, len(start))
	for i, v := range start {
		frontier[i] = &vertexAtDepth{
			Vertex: v,
			Depth:  0,
		}
	}
	for len(frontier) > 0 {
		// Pop the current vertex
		n := len(frontier)
		current := frontier[n-1]
		frontier = frontier[:n-1]

		// Check if we've seen this already and return...
		if _, ok := seen[current.Vertex]; ok {
			continue
		}
		seen[current.Vertex] = struct{}{}

		// Add next set of targets in a consistent order.
		targets := AsVertexList(g.upEdgesNoCopy(current.Vertex))
		sort.Sort(byVertexName(targets))
		for _, t := range targets {
			frontier = append(frontier, &vertexAtDepth{
				Vertex: t,
				Depth:  current.Depth + 1,
			})
		}

		// Visit the current node
		if err := f(current.Vertex, current.Depth); err != nil {
			return err
		}
	}

	return nil
}

// byVertexName implements sort.Interface so a list of Vertices can be sorted
// consistently by their VertexName
type byVertexName []Vertex

func (b byVertexName) Len() int      { return len(b) }
func (b byVertexName) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byVertexName) Less(i, j int) bool {
	return VertexName(b[i]) < VertexName(b[j])
}
