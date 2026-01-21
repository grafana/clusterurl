package trie

import (
	"math"
	"sort"
	"strings"
	"sync"
)

// pathNode represents a single segment in the URL path trie.
type pathNode struct {
	// segment is the path segment this node represents (e.g., "api", "users", "*")
	segment string

	// children maps segment strings to child nodes
	children map[string]*pathNode

	// softCollapsed indicates this node has passed the soft threshold.
	// New children will be directed to the wildcard, but existing children are preserved.
	softCollapsed bool

	// hardCollapsed indicates this node has passed the hard threshold.
	// All children have been merged into a single wildcard.
	hardCollapsed bool

	// uniqueChildrenSeen tracks the total number of unique children ever seen.
	// Used to determine when to trigger hard collapse.
	uniqueChildrenSeen int

	// isWildcard indicates if this node represents a collapsed wildcard
	isWildcard bool

	// depth is the depth of this node in the trie (root children = 0)
	depth int
}

// PathTrie is a thread-safe trie for clustering URL paths.
// It uses a two-threshold system:
// - Soft threshold: After N unique children, new children go to wildcard but existing are preserved
// - Hard threshold: After M unique children, all children collapse to a single wildcard
// Additionally, a global MaxPatterns limit can trigger hard collapse of deepest soft-collapsed nodes.
type PathTrie struct {
	root         *pathNode
	mu           sync.RWMutex
	cfg          *TrieConfig
	patternCount int // current number of unique patterns (leaf paths)
}

// NewPathTrie creates a new PathTrie with the given configuration.
// If config is nil, DefaultTrieConfig() is used.
func NewPathTrie(config *TrieConfig) (*PathTrie, error) {
	if config == nil {
		config = DefaultTrieConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &PathTrie{
		root: &pathNode{
			segment:  "",
			children: make(map[string]*pathNode),
			depth:    -1, // root is at depth -1, so children are at depth 0
		},
		cfg: config,
	}, nil
}

// getSoftMaxCardinality returns the soft max cardinality for a given depth.
func (t *PathTrie) getSoftMaxCardinality(depth int) int {
	if t.cfg.DepthSoftCardinalities != nil {
		if card, ok := t.cfg.DepthSoftCardinalities[depth]; ok {
			if card == -1 {
				return math.MaxInt
			}
			return card
		}
	}
	return t.cfg.SoftMaxCardinality
}

// getHardMaxCardinality returns the hard max cardinality for a given depth.
func (t *PathTrie) getHardMaxCardinality(depth int) int {
	if t.cfg.DepthHardCardinalities != nil {
		if card, ok := t.cfg.DepthHardCardinalities[depth]; ok {
			if card == -1 {
				return math.MaxInt
			}
			return card
		}
	}
	return t.cfg.HardMaxCardinality
}

// parsePath splits a path into segments, handling query strings and edge cases.
func (t *PathTrie) parsePath(path string) []string {
	// Strip query string
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}

	// Handle HTTP method prefix (e.g., "GET /path")
	if idx := strings.Index(path, " "); idx > 0 {
		op := path[:idx]
		if isHTTPMethod(op) && idx < len(path) {
			path = path[idx+1:]
		}
	}

	// Trim leading/trailing separators and split
	path = strings.Trim(path, t.cfg.Separator)
	if path == "" {
		return nil
	}

	return strings.Split(path, t.cfg.Separator)
}

// isHTTPMethod checks if the string is an HTTP method.
func isHTTPMethod(op string) bool {
	switch op {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
		return true
	}
	return false
}

// Insert adds a path to the trie and returns the normalized/clustered path.
// Uses two-threshold collapsing:
// - After soft threshold: new children go to wildcard, existing preserved
// - After hard threshold: all children collapse to wildcard
// Additionally, if MaxPatterns is exceeded, deepest soft-collapsed nodes are hard-collapsed.
// Thread-safe.
func (t *PathTrie) Insert(path string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	segments := t.parsePath(path)
	if len(segments) == 0 {
		return path
	}

	result := t.insertSegments(segments)

	// Update pattern count and enforce limit
	t.updatePatternCount()
	t.enforcePatternLimit()

	return t.cfg.Separator + strings.Join(result, t.cfg.Separator)
}

// insertSegments inserts segments into the trie and returns the resulting path.
func (t *PathTrie) insertSegments(segments []string) []string {
	current := t.root
	result := make([]string, 0, len(segments))

	for depth, segment := range segments {
		if segment == "" {
			result = append(result, segment)
			continue
		}

		// If we're beyond max depth, just append segments as-is
		if depth >= t.cfg.MaxDepth {
			result = append(result, segments[depth:]...)
			break
		}

		// Case 1: Node is hard collapsed - everything goes to wildcard
		if current.hardCollapsed {
			result = append(result, t.cfg.ReplaceWith)
			current = t.getOrCreateWildcard(current, depth)
			continue
		}

		// Case 2: Node is soft collapsed - check if segment exists or use wildcard
		if current.softCollapsed {
			// Check if this segment is one of the preserved explicit children
			if child, exists := current.children[segment]; exists && segment != t.cfg.ReplaceWith {
				result = append(result, segment)
				current = child
				continue
			}

			// New segment after soft collapse - count it and use wildcard
			current.uniqueChildrenSeen++

			// Check if we've hit the hard threshold
			hardMax := t.getHardMaxCardinality(depth)
			if current.uniqueChildrenSeen > hardMax {
				t.hardCollapseNode(current)
				result = append(result, t.cfg.ReplaceWith)
				current = current.children[t.cfg.ReplaceWith]
				continue
			}

			// Use wildcard
			result = append(result, t.cfg.ReplaceWith)
			current = t.getOrCreateWildcard(current, depth)
			continue
		}

		// Case 3: Normal operation - not yet soft collapsed
		child, exists := current.children[segment]

		if exists {
			result = append(result, segment)
			current = child
			continue
		}

		// New segment - check soft threshold
		current.uniqueChildrenSeen++
		softMax := t.getSoftMaxCardinality(depth)

		if current.uniqueChildrenSeen > softMax {
			// Hit soft threshold - mark as soft collapsed
			current.softCollapsed = true

			// Check if we also hit hard threshold
			hardMax := t.getHardMaxCardinality(depth)
			if current.uniqueChildrenSeen > hardMax {
				t.hardCollapseNode(current)
				result = append(result, t.cfg.ReplaceWith)
				current = current.children[t.cfg.ReplaceWith]
				continue
			}

			// Soft collapse only - use wildcard for this new segment
			result = append(result, t.cfg.ReplaceWith)
			current = t.getOrCreateWildcard(current, depth)
			continue
		}

		// Under soft threshold - create new explicit child
		child = &pathNode{
			segment:  segment,
			children: make(map[string]*pathNode),
			depth:    depth,
		}
		current.children[segment] = child
		result = append(result, segment)
		current = child
	}

	return result
}

// getOrCreateWildcard returns the wildcard child of a node, creating it if needed.
func (t *PathTrie) getOrCreateWildcard(node *pathNode, depth int) *pathNode {
	if node.children[t.cfg.ReplaceWith] == nil {
		node.children[t.cfg.ReplaceWith] = &pathNode{
			segment:    t.cfg.ReplaceWith,
			children:   make(map[string]*pathNode),
			isWildcard: true,
			depth:      depth,
		}
	}
	return node.children[t.cfg.ReplaceWith]
}

// hardCollapseNode performs a hard collapse: merges all children into a single wildcard.
func (t *PathTrie) hardCollapseNode(node *pathNode) {
	if node.hardCollapsed {
		return
	}

	node.hardCollapsed = true
	node.softCollapsed = true

	// Create or get wildcard node (children are at depth+1)
	wildcardNode := t.getOrCreateWildcard(node, node.depth+1)

	// Merge all explicit children into the wildcard node
	for segment, child := range node.children {
		if segment == t.cfg.ReplaceWith {
			continue // Skip the wildcard itself
		}
		t.mergeChildren(wildcardNode, child)
	}

	// Replace all children with just the wildcard
	node.children = map[string]*pathNode{
		t.cfg.ReplaceWith: wildcardNode,
	}

	// Recursively check if wildcard node needs hard collapsing
	// Use default thresholds for merged nodes
	if wildcardNode.uniqueChildrenSeen > t.cfg.HardMaxCardinality {
		t.hardCollapseNode(wildcardNode)
	}
}

// mergeChildren merges children from source into target.
// This is called during hard collapse to combine all child paths.
func (t *PathTrie) mergeChildren(target, source *pathNode) {
	for segment, child := range source.children {
		if existing, exists := target.children[segment]; exists {
			// Child already exists, recursively merge their children
			t.mergeChildren(existing, child)
			// Inherit collapse state
			if child.softCollapsed {
				existing.softCollapsed = true
			}
			if child.hardCollapsed {
				existing.hardCollapsed = true
			}
			// Take max of uniqueChildrenSeen
			if child.uniqueChildrenSeen > existing.uniqueChildrenSeen {
				existing.uniqueChildrenSeen = child.uniqueChildrenSeen
			}
		} else {
			// New child, add it
			target.children[segment] = child
			target.uniqueChildrenSeen++
		}
	}
}

// Lookup returns the clustered representation of a path without modifying the trie.
// Thread-safe.
func (t *PathTrie) Lookup(path string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	segments := t.parsePath(path)
	if len(segments) == 0 {
		return path
	}

	result := t.lookupSegments(segments)
	return t.cfg.Separator + strings.Join(result, t.cfg.Separator)
}

// lookupSegments traverses the trie and returns the path with wildcards applied.
func (t *PathTrie) lookupSegments(segments []string) []string {
	current := t.root
	result := make([]string, 0, len(segments))

	for i, segment := range segments {
		if segment == "" {
			result = append(result, segment)
			continue
		}

		// If node is hard collapsed, use wildcard
		if current.hardCollapsed {
			result = append(result, t.cfg.ReplaceWith)
			current = current.children[t.cfg.ReplaceWith]
			if current == nil {
				result = append(result, segments[i+1:]...)
				break
			}
			continue
		}

		// Try to find exact match first
		child, exists := current.children[segment]
		if exists {
			result = append(result, segment)
			current = child
			continue
		}

		// If soft collapsed, check for wildcard
		if current.softCollapsed {
			if wildcardChild, hasWildcard := current.children[t.cfg.ReplaceWith]; hasWildcard {
				result = append(result, t.cfg.ReplaceWith)
				current = wildcardChild
				continue
			}
		}

		// Not found at all, append remaining segments as-is
		result = append(result, segments[i:]...)
		break
	}

	return result
}

// Reset clears all nodes from the trie, resetting it to initial state.
// Thread-safe.
func (t *PathTrie) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.root = &pathNode{
		segment:  "",
		children: make(map[string]*pathNode),
		depth:    -1,
	}
	t.patternCount = 0
}

// NodeCount returns the approximate number of nodes in the trie.
// Thread-safe.
func (t *PathTrie) NodeCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countNodes(t.root)
}

// countNodes recursively counts all nodes in the subtree.
func (t *PathTrie) countNodes(node *pathNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}

// PatternCount returns the current number of unique patterns in the trie.
// A pattern is a unique path from root to a leaf node.
// Thread-safe.
func (t *PathTrie) PatternCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.patternCount
}

// countPatterns counts all leaf nodes (nodes with no children) in the subtree.
// This represents the number of unique patterns.
func (t *PathTrie) countPatterns(node *pathNode) int {
	if node == nil {
		return 0
	}
	if len(node.children) == 0 {
		return 1
	}
	count := 0
	for _, child := range node.children {
		count += t.countPatterns(child)
	}
	return count
}

// updatePatternCount recalculates the pattern count by traversing the trie.
func (t *PathTrie) updatePatternCount() {
	t.patternCount = t.countPatterns(t.root)
}

// findCollapseCandidates returns all soft-collapsed nodes that haven't been hard-collapsed.
// These are candidates for hard collapse when over the pattern limit.
func (t *PathTrie) findCollapseCandidates() []*pathNode {
	var candidates []*pathNode
	t.collectCandidates(t.root, &candidates)
	return candidates
}

// collectCandidates recursively collects soft-collapsed nodes.
func (t *PathTrie) collectCandidates(node *pathNode, candidates *[]*pathNode) {
	if node == nil {
		return
	}
	if node.softCollapsed && !node.hardCollapsed {
		*candidates = append(*candidates, node)
	}
	for _, child := range node.children {
		t.collectCandidates(child, candidates)
	}
}

// enforcePatternLimit checks if the pattern count exceeds MaxPatterns.
// If so, it hard-collapses the deepest soft-collapsed nodes until under the limit.
func (t *PathTrie) enforcePatternLimit() {
	if t.cfg.MaxPatterns <= 0 {
		return // no limit
	}

	for t.patternCount > t.cfg.MaxPatterns {
		candidates := t.findCollapseCandidates()
		if len(candidates) == 0 {
			break // no candidates to collapse, accept over limit
		}

		// Sort by depth descending (deepest first)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].depth > candidates[j].depth
		})

		// Hard collapse the deepest candidate
		t.hardCollapseNode(candidates[0])
		t.updatePatternCount()
	}
}
