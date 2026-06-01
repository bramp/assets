package render

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

// supportsScaleMode reports whether a step can satisfy the requested mode.
// Empty mode or empty capability list is treated as permissive for backward
// compatibility with tools that do not model mode constraints.
func supportsScaleMode(supported []string, mode string) bool {
	normMode := strings.ToLower(strings.TrimSpace(mode))
	if normMode == "" || len(supported) == 0 {
		return true
	}
	for _, m := range supported {
		norm := strings.ToLower(strings.TrimSpace(m))
		if norm == normMode {
			return true
		}
	}
	return false
}

type graphEdge struct {
	From graphNode
	To   graphNode
	Step ResolvedStep
}

type pathState struct {
	node  graphNode
	steps []ResolvedStep
}

type pathSearchResult struct {
	// solutions contains all candidate paths that satisfy the goal constraints.
	solutions [][]ResolvedStep
	// graph stores explored outgoing edges per node for debugging/visualization.
	graph map[graphNode][]graphEdge
}

type graphNode struct {
	// format is the file extension represented by this state (for example .png).
	format string
	// sized tracks whether a size-setting step has already occurred on the path.
	sized bool
	// optimized tracks whether the terminal optimization step has been applied.
	optimized bool
}

// graphResolver encapsulates state and helpers for bounded graph search.
type graphResolver struct {
	tools          map[string]manifest.ToolSpec
	preferenceRank map[string]int
	sourceExt      string
	outputExt      string
	scaleMode      string
	// sizeRequested captures caller intent (width/height set); whether it
	// becomes a hard goal is decided from reachable tool capabilities.
	sizeRequested     bool
	requireSized      bool
	checkAvailability bool
	toolRepo          ToolRepository
	terminalOptimizer *candidateTool
	requireOptimized  bool
	initErr           error
	maxDepth          int
}

func newGraphResolver(
	tools map[string]manifest.ToolSpec,
	optimizeByFormat map[string]string,
	preferenceRank map[string]int,
	sourceExt string,
	outputExt string,
	scaleMode string,
	requireSized bool,
	checkAvailability bool,
	toolRepo ToolRepository,
) *graphResolver {
	if toolRepo == nil {
		toolRepo = NewToolRepository()
	}
	terminalOptimizer, requireOptimized, err := resolveTerminalOptimizer(
		tools,
		optimizeByFormat,
		outputExt,
		checkAvailability,
		toolRepo,
	)
	r := &graphResolver{
		tools:             tools,
		preferenceRank:    preferenceRank,
		sourceExt:         sourceExt,
		outputExt:         outputExt,
		scaleMode:         scaleMode,
		sizeRequested:     requireSized,
		checkAvailability: checkAvailability,
		toolRepo:          toolRepo,
		terminalOptimizer: terminalOptimizer,
		requireOptimized:  requireOptimized,
		initErr:           err,
		maxDepth:          4,
	}
	if r.sizeRequested {
		r.requireSized = r.canSatisfySizedGoal()
	}

	return r
}

func (r *graphResolver) resolve() ([]ResolvedStep, error) {
	if r.initErr != nil {
		return nil, r.initErr
	}
	// Search begins in the source format with no size/optimization guarantees.
	start := graphNode{format: r.sourceExt, sized: false, optimized: false}
	search := r.findCandidatePathsWithGraph(start, r.outputExt, r.requireSized, r.requireOptimized)
	solutions := search.solutions
	if len(solutions) == 0 {
		return nil, fmt.Errorf("no compatible conversion path from %q to %q", r.sourceExt, r.outputExt)
	}
	bestPath := r.chooseBestPath(solutions)
	return append([]ResolvedStep(nil), bestPath...), nil
}

// canSatisfySizedGoal reports whether any reachable path can end at the
// requested output format with sized=true.
func (r *graphResolver) canSatisfySizedGoal() bool {
	start := graphNode{format: r.sourceExt, sized: false, optimized: false}
	search := r.findCandidatePathsWithGraph(start, r.outputExt, true, false)
	return len(search.solutions) > 0
}

// neighborEdges returns direct transitions from one graph state.
func (r *graphResolver) neighborEdges(fromNode graphNode) []graphEdge {
	// optimized is terminal by design: once optimized, no further steps are valid.
	if fromNode.optimized {
		return nil
	}

	edges := make([]graphEdge, 0)
	from := fromNode.format
	for _, candidate := range r.candidateTools() {
		if !matchesFormatList(candidate.Tool.Accepts, from) {
			continue
		}
		setsSize := candidate.Tool.SetsTargetSize()

		// This limits us to direct transitions from source to output format, but that is sufficient to
		// model optimizers and keeps the graph size manageable without precomputing a full format conversion graph.
		for _, to := range producedFormats(candidate.Tool.Produces, r.outputExt) {
			if to == "" {
				continue
			}
			// Once the size is correct, all remaining steps are considered to produce the correct size.
			toNode := graphNode{format: to, sized: fromNode.sized || setsSize, optimized: false}
			edges = append(edges, graphEdge{
				From: fromNode,
				To:   toNode,
				Step: resolvedStepFromTool(candidate.Name, candidate.Tool, from, to),
			})
		}
	}

	if r.terminalOptimizer != nil && from == r.outputExt {
		// Optimization is modeled as a same-format terminal transition.
		optNode := graphNode{format: from, sized: fromNode.sized, optimized: true}
		edges = append(edges, graphEdge{
			From: fromNode,
			To:   optNode,
			Step: resolvedStepFromTool(r.terminalOptimizer.Name, r.terminalOptimizer.Tool, from, from),
		})
	}

	return edges
}

type candidateTool struct {
	Name string
	Tool manifest.ToolSpec
}

// candidateTools returns a deterministic, filtered set of non-optimizer tools
// eligible for normal graph expansion.
func (r *graphResolver) candidateTools() []candidateTool {
	keys := make([]string, 0, len(r.tools))
	for name := range r.tools {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	toolsOut := make([]candidateTool, 0, len(keys))
	for _, name := range keys {
		tool := r.tools[name]
		normName := strings.ToLower(strings.TrimSpace(name))
		if normName == "" || normName == "none" || normName == "off" {
			continue
		}
		if r.terminalOptimizer != nil && normName == r.terminalOptimizer.Name {
			continue
		}
		if !supportsScaleMode(tool.ScaleModes, r.scaleMode) {
			continue
		}
		if r.checkAvailability && !r.toolRepo.Available(tool.Tool) {
			continue
		}
		toolsOut = append(toolsOut, candidateTool{Name: normName, Tool: tool})
	}

	return toolsOut
}

// findCandidatePathsWithGraph performs a bounded breadth-first search (BFS)
// over graphNode states.
//
// Why BFS: it naturally discovers shortest pipelines first. We still keep
// equal-depth alternatives so later preference scoring can break ties.
func (r *graphResolver) findCandidatePathsWithGraph(
	start graphNode,
	goalFormat string,
	requireGoalSized bool,
	requireGoalOptimized bool,
) pathSearchResult {
	// FIFO queue gives classic BFS traversal by path depth.
	queue := []pathState{{node: start, steps: nil}}
	// best tracks the shortest known depth per node for pruning.
	best := make(map[graphNode]int)
	best[start] = 0
	// graph is built lazily so callers can inspect what the search explored.
	result := pathSearchResult{graph: map[graphNode][]graphEdge{}}

	for len(queue) > 0 {
		// Dequeue next state in BFS order.
		state := queue[0]
		queue = queue[1:]

		edges, ok := result.graph[state.node]
		if !ok {
			// Expand neighbors once per node and memoize for reuse/debug output.
			edges = r.neighborEdges(state.node)
			result.graph[state.node] = edges
		}

		// Cap search depth to bound worst-case exploration cost.
		if len(state.steps) >= r.maxDepth {
			continue
		}

		for _, edge := range edges {
			nextSteps := append(append([]ResolvedStep(nil), state.steps...), edge.Step)
			// Goal is format + optional sized/optimized constraints.
			if edge.To.format == goalFormat &&
				(!requireGoalSized || edge.To.sized) &&
				(!requireGoalOptimized || edge.To.optimized) {
				result.solutions = append(result.solutions, nextSteps)
				continue
			}
			depth := len(nextSteps)
			// Keep equal-depth alternatives for preference tie-breaking; prune only
			// strictly worse (deeper) revisits of the same state.
			if prev, ok := best[edge.To]; ok && depth > prev {
				continue
			}
			best[edge.To] = depth
			queue = append(queue, pathState{node: edge.To, steps: nextSteps})
		}
	}

	return result
}

func (r *graphResolver) chooseBestPath(paths [][]ResolvedStep) []ResolvedStep {
	bestPath := paths[0]
	bestScore := graphPathScore(bestPath, r.preferenceRank)
	for _, p := range paths[1:] {
		s := graphPathScore(p, r.preferenceRank)
		if s < bestScore {
			bestPath = p
			bestScore = s
		}
	}
	return bestPath
}

// resolveGraphPath finds a compatible conversion path from sourceExt to
// outputExt using a bounded breadth-first search.
//
// Why: shortest-path planning gives deterministic, easy-to-reason-about
// pipelines while still allowing preference-based tie-breaking.
func resolveGraphPath(
	tools map[string]manifest.ToolSpec,
	optimizeByFormat map[string]string,
	preferenceRank map[string]int,
	sourceExt string,
	outputExt string,
	scaleMode string,
	requireSized bool,
	opts ResolveOptions,
	toolRepo ToolRepository,
) ([]ResolvedStep, error) {
	if sourceExt == "" || outputExt == "" {
		return nil, errors.New("unable to resolve conversion path for empty source/output format")
	}
	resolver := newGraphResolver(
		tools,
		optimizeByFormat,
		preferenceRank,
		sourceExt,
		outputExt,
		scaleMode,
		requireSized,
		opts.CheckAvailability,
		toolRepo,
	)
	return resolver.resolve()
}

func resolveTerminalOptimizer(
	tools map[string]manifest.ToolSpec,
	optimizeByFormat map[string]string,
	outputExt string,
	checkAvailability bool,
	toolRepo ToolRepository,
) (*candidateTool, bool, error) {
	normExt := strings.ToLower(strings.TrimSpace(outputExt))
	if normExt == "" || len(optimizeByFormat) == 0 {
		return nil, false, nil
	}

	optimizeStepName, ok := optimizeByFormat[normExt]
	if !ok {
		return nil, false, nil
	}
	normOptimizeStepName := strings.ToLower(strings.TrimSpace(optimizeStepName))
	if normOptimizeStepName == "" {
		return nil, false, nil
	}

	optimizeDef, ok := findToolSpec(tools, normOptimizeStepName)
	if !ok {
		return nil, false, fmt.Errorf(
			"optimizer %q configured for %q not found in render tools",
			normOptimizeStepName,
			normExt,
		)
	}
	if !matchesFormatList(optimizeDef.Accepts, normExt) || !matchesFormatList(optimizeDef.Produces, normExt) {
		return nil, false, fmt.Errorf(
			"optimizer %q configured for %q must accept and produce %q",
			normOptimizeStepName,
			normExt,
			normExt,
		)
	}
	if checkAvailability && !toolRepo.Available(optimizeDef.Tool) {
		return nil, false, fmt.Errorf("optimizer tool %q for %q is not available", optimizeDef.Tool, normExt)
	}

	return &candidateTool{Name: normOptimizeStepName, Tool: optimizeDef}, true, nil
}

func findToolSpec(tools map[string]manifest.ToolSpec, toolName string) (manifest.ToolSpec, bool) {
	normName := strings.ToLower(strings.TrimSpace(toolName))
	for name, spec := range tools {
		if strings.ToLower(strings.TrimSpace(name)) == normName {
			return spec, true
		}
	}
	return manifest.ToolSpec{}, false
}

func resolvedStepFromTool(
	name string,
	step manifest.ToolSpec,
	inputFormat string,
	outputFormat string,
) ResolvedStep {
	return ResolvedStep{
		Name:         strings.TrimSpace(name),
		Tool:         step.Tool,
		Command:      step.Command,
		SizeTemplate: step.SizeTemplate,
		SizeByMode:   step.SizeByMode,
		VersionArgs:  append([]string(nil), step.VersionArgs...),
		InputFormat:  strings.ToLower(strings.TrimSpace(inputFormat)),
		OutputFormat: strings.ToLower(strings.TrimSpace(outputFormat)),
	}
}

// firstCommandToken extracts the executable token from a tool declaration.
// Tool entries may include arguments, but availability checks need only the
// binary name.
func firstCommandToken(toolName string) string {
	binary := strings.TrimSpace(toolName)
	if binary == "" {
		return ""
	}
	parts := strings.Fields(binary)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// producedFormats normalizes a tool's produces list into concrete extensions.
func producedFormats(produces []string, outputExt string) []string {
	if len(produces) == 0 {
		return nil
	}
	// outputExt is currently unused after wildcard removal, but is kept to avoid
	// a wider signature change in callers.
	_ = outputExt
	result := make([]string, 0, len(produces))
	for _, p := range produces {
		norm := strings.ToLower(strings.TrimSpace(p))
		if norm == "" {
			continue
		}
		result = append(result, norm)
	}
	return result
}

// matchesFormatList reports whether format is accepted by list using
// normalized extension matching.
func matchesFormatList(list []string, format string) bool {
	if len(list) == 0 || format == "" {
		return false
	}
	normFormat := strings.ToLower(strings.TrimSpace(format))
	for _, v := range list {
		norm := strings.ToLower(strings.TrimSpace(v))
		if norm == normFormat {
			return true
		}
	}
	return false
}

// graphPathScore ranks candidate paths by favoring shorter chains first and
// then applying preference order as a deterministic tie-breaker.
func graphPathScore(path []ResolvedStep, pref map[string]int) int {
	score := len(path) * 1000
	for i, step := range path {
		rank, ok := pref[strings.ToLower(strings.TrimSpace(step.Name))]
		if !ok {
			rank = 999
		}
		score += rank * (10 + i)
	}
	return score
}

// buildPreferenceRank computes a normalized preference rank map where lower
// rank means higher priority.
//
// Why: output-level preferences can embed "auto" to splice defaults at a
// specific position rather than replacing defaults entirely.
func buildPreferenceRank(outputPref manifest.ToolPreference, defaultPref manifest.ToolPreference) map[string]int {
	var expanded []string
	if len(outputPref) == 0 {
		expanded = append([]string(nil), defaultPref...)
	} else {
		expanded = make([]string, 0, len(outputPref)+len(defaultPref))
		for _, item := range outputPref {
			norm := strings.TrimSpace(item)
			if strings.EqualFold(norm, "auto") {
				expanded = append(expanded, defaultPref...)
				continue
			}
			expanded = append(expanded, item)
		}
	}

	rank := make(map[string]int, len(expanded))
	for i, item := range expanded {
		norm := strings.TrimSpace(item)
		if norm == "" {
			continue
		}
		norm = strings.ToLower(norm)
		if _, exists := rank[norm]; exists {
			continue
		}
		rank[norm] = i
	}

	return rank
}

func hasExplicitToolPreference(pref manifest.ToolPreference) bool {
	for _, name := range pref {
		norm := strings.ToLower(strings.TrimSpace(name))
		if norm == "" || norm == "auto" {
			continue
		}
		return true
	}
	return false
}
