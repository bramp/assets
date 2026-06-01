package render

import (
	"fmt"
	"sort"
	"strings"
)

type dotEdge struct {
	from graphNode
	to   graphNode
	step ResolvedStep
}

func (r *graphResolver) graphDOT(title string) (string, error) {
	if r.initErr != nil {
		return "", r.initErr
	}

	// DOT defaults to a compact graph that includes only states needed to
	// satisfy requested goals, without expanding past terminal goal states.

	startNode := r.startNode()
	search := r.findCandidatePathsWithGraph(startNode, r.outputExt, r.requireSized, r.requireOptimized)
	graph := search.graph
	nodes := r.dotNodes(graph, startNode)
	nodeID := dotNodeIDs(nodes)
	edges := dotEdges(graph, nodes)

	return r.renderDOT(title, startNode, nodes, nodeID, edges), nil
}

func (r *graphResolver) dotNodes(graph map[graphNode][]graphEdge, startNode graphNode) []graphNode {
	nodeSet := make(map[graphNode]struct{}, len(graph))
	for from, out := range graph {
		nodeSet[from] = struct{}{}
		for _, edge := range out {
			nodeSet[edge.To] = struct{}{}
		}
	}

	nodes := make([]graphNode, 0, len(nodeSet))
	for node := range nodeSet {
		if !r.includeDOTNode(node, startNode) {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return graphNodeLess(nodes[i], nodes[j])
	})

	return nodes
}

func dotNodeIDs(nodes []graphNode) map[graphNode]string {
	nodeID := make(map[graphNode]string, len(nodes))
	for i, node := range nodes {
		nodeID[node] = fmt.Sprintf("n%d", i)
	}
	return nodeID
}

func dotEdges(graph map[graphNode][]graphEdge, nodes []graphNode) []dotEdge {
	edges := make([]dotEdge, 0, len(nodes)*2)
	included := make(map[graphNode]struct{}, len(nodes))
	for _, node := range nodes {
		included[node] = struct{}{}
	}
	for from, out := range graph {
		if _, ok := included[from]; !ok {
			continue
		}
		for _, edge := range out {
			if _, ok := included[edge.To]; !ok {
				continue
			}
			edges = append(edges, dotEdge{from: from, to: edge.To, step: edge.Step})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return graphNodeLess(edges[i].from, edges[j].from)
		}
		if edges[i].to != edges[j].to {
			return graphNodeLess(edges[i].to, edges[j].to)
		}
		return strings.TrimSpace(edges[i].step.Name) < strings.TrimSpace(edges[j].step.Name)
	})

	return edges
}

func (r *graphResolver) renderDOT(
	title string,
	startNode graphNode,
	nodes []graphNode,
	nodeID map[graphNode]string,
	edges []dotEdge,
) string {
	var b strings.Builder
	b.WriteString("digraph render_graph {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box];\n")
	if strings.TrimSpace(title) != "" {
		fmt.Fprintf(&b, "  label=\"%s\";\n", dotEscape(strings.TrimSpace(title)))
		b.WriteString("  labelloc=t;\n")
	}

	for _, node := range nodes {
		attrs := []string{
			fmt.Sprintf("label=\"%s\\nsized=%t\\noptimized=%t\"", dotEscape(node.format), node.sized, node.optimized),
		}
		if node == startNode {
			attrs = append(attrs, "style=\"filled,bold\"", "fillcolor=\"#d9f2d9\"", "color=\"#2b9348\"")
		}
		if r.isGoalNode(node) {
			attrs = append(attrs, "style=\"filled,bold\"", "fillcolor=\"#ffe0b2\"", "color=\"#c77d00\"")
		}

		fmt.Fprintf(
			&b,
			"  %s [%s];\n",
			nodeID[node],
			strings.Join(attrs, " "),
		)
	}

	for _, edge := range edges {
		fmt.Fprintf(
			&b,
			"  %s -> %s [label=\"%s\"];\n",
			nodeID[edge.from],
			nodeID[edge.to],
			dotEscape(strings.TrimSpace(edge.step.Name)),
		)
	}
	b.WriteString("}\n")

	return b.String()
}

func (r *graphResolver) includeDOTNode(node graphNode, start graphNode) bool {
	if node.format == r.sourceExt && node != start {
		return false
	}
	return true
}

func (r *graphResolver) startNode() graphNode {
	return graphNode{format: r.sourceExt, sized: false, optimized: false}
}

func (r *graphResolver) isGoalNode(node graphNode) bool {
	if node.format != r.outputExt {
		return false
	}
	if r.requireSized && !node.sized {
		return false
	}
	if r.requireOptimized {
		return node.optimized
	}
	return !node.optimized
}

func graphNodeLess(a, b graphNode) bool {
	if a.format != b.format {
		return a.format < b.format
	}
	if a.sized != b.sized {
		return !a.sized && b.sized
	}
	if a.optimized != b.optimized {
		return !a.optimized && b.optimized
	}
	return false
}

func dotEscape(s string) string {
	s = strings.ReplaceAll(s, `\\`, `\\\\`)
	s = strings.ReplaceAll(s, `"`, `\\"`)
	return s
}
