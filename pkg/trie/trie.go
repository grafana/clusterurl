package trie

import (
	"math"
	"strings"
	"sync"
)

// pathNode represents a single segment in the URL path trie.
type pathNode struct {
	// segment is the path segment this node represents (e.g., "api", "users", "*")
	segment string

	// children maps segment strings to child nodes
	children map[string]*pathNode

	// collapsed indicates if this node's children have been collapsed to a wildcard
	collapsed bool

	// cardinality tracks the number of distinct children observed
	cardinality int

	// isWildcard indicates if this node represents a collapsed wildcard
	isWildcard bool
}

// PathTrie is a thread-safe trie for clustering URL paths.
// It dynamically collapses high-cardinality segments into wildcards.
type PathTrie struct {
	root *pathNode
	mu   sync.RWMutex
	cfg  *TrieConfig
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
		},
		cfg: config,
	}, nil
}

// getMaxCardinality returns the max cardinality for a given depth.
// It checks DepthCardinalities first, then falls back to DefaultMaxCardinality.
// A value of -1 in DepthCardinalities means no limit (never collapse).
func (t *PathTrie) getMaxCardinality(depth int) int {
	if t.cfg.DepthCardinalities != nil {
		if card, ok := t.cfg.DepthCardinalities[depth]; ok {
			if card == -1 {
				return math.MaxInt
			}
			return card
		}
	}
	return t.cfg.DefaultMaxCardinality
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
// If a segment exceeds maxCardinality, it collapses to the wildcard.
// Thread-safe.
func (t *PathTrie) Insert(path string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	segments := t.parsePath(path)
	if len(segments) == 0 {
		return path
	}

	result := t.insertSegments(segments)
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

		// If current node is already collapsed, all children become wildcards
		if current.collapsed {
			result = append(result, t.cfg.ReplaceWith)
			// Continue with the wildcard child
			if current.children[t.cfg.ReplaceWith] == nil {
				current.children[t.cfg.ReplaceWith] = &pathNode{
					segment:    t.cfg.ReplaceWith,
					children:   make(map[string]*pathNode),
					isWildcard: true,
				}
			}
			current = current.children[t.cfg.ReplaceWith]
			continue
		}

		// Check if this segment already exists
		child, exists := current.children[segment]

		if !exists {
			maxCard := t.getMaxCardinality(depth)

			// New segment - check if we need to collapse
			if current.cardinality >= maxCard {
				// Collapse this level
				t.collapseNode(current)
				result = append(result, t.cfg.ReplaceWith)
				current = current.children[t.cfg.ReplaceWith]
				continue
			}

			// Create new child
			child = &pathNode{
				segment:  segment,
				children: make(map[string]*pathNode),
			}
			current.children[segment] = child
			current.cardinality++

			// Check if we just hit the threshold
			if current.cardinality > maxCard {
				t.collapseNode(current)
				result = append(result, t.cfg.ReplaceWith)
				current = current.children[t.cfg.ReplaceWith]
				continue
			}
		}

		result = append(result, segment)
		current = child
	}

	return result
}

// collapseNode collapses a node by replacing all children with a single wildcard
// and merging their children into the wildcard node.
func (t *PathTrie) collapseNode(node *pathNode) {
	if node.collapsed {
		return
	}

	node.collapsed = true

	// Create or get wildcard node
	wildcardNode, hasWildcard := node.children[t.cfg.ReplaceWith]
	if !hasWildcard {
		wildcardNode = &pathNode{
			segment:    t.cfg.ReplaceWith,
			children:   make(map[string]*pathNode),
			isWildcard: true,
		}
	}

	// Merge all children into the wildcard node
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
	node.cardinality = 1

	// Recursively check if wildcard node needs collapsing
	// Use depth 0 for the wildcard node's children since we don't track depth in nodes
	if wildcardNode.cardinality > t.cfg.DefaultMaxCardinality {
		t.collapseNode(wildcardNode)
	}
}

// mergeChildren merges children from source into target.
// This is called during collapse to combine all child paths.
func (t *PathTrie) mergeChildren(target, source *pathNode) {
	for segment, child := range source.children {
		if existing, exists := target.children[segment]; exists {
			// Child already exists, recursively merge their children
			t.mergeChildren(existing, child)
		} else {
			// New child, add it
			target.children[segment] = child
			target.cardinality++
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

		// If node is collapsed, use wildcard
		if current.collapsed {
			result = append(result, t.cfg.ReplaceWith)
			current = current.children[t.cfg.ReplaceWith]
			if current == nil {
				// Can't traverse further, append remaining as-is
				result = append(result, segments[i+1:]...)
				break
			}
			continue
		}

		// Try to find exact match
		child, exists := current.children[segment]
		if !exists {
			// No exact match, check for wildcard
			if wildcardChild, hasWildcard := current.children[t.cfg.ReplaceWith]; hasWildcard {
				result = append(result, t.cfg.ReplaceWith)
				current = wildcardChild
				continue
			}
			// Not found at all, append remaining segments as-is
			result = append(result, segments[i:]...)
			break
		}

		result = append(result, segment)
		current = child
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
	}
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
