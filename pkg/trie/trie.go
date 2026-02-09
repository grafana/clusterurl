package trie

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
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

	// wildcardedSegments tracks segments that were routed to wildcard after soft collapse.
	// This prevents counting the same segment multiple times toward hard collapse.
	wildcardedSegments map[string]struct{}

	// isWildcard indicates if this node represents a collapsed wildcard
	isWildcard bool

	// depth is the depth of this node in the trie (root children = 0)
	depth int

	// lastSeen is when this node was last accessed via Insert.
	// Used for TTL-based pruning.
	lastSeen time.Time
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

	// Pruning goroutine control
	stopPrune chan struct{}  // signal to stop background pruning
	pruneWg   sync.WaitGroup // wait for pruner goroutine to exit
}

// NewPathTrie creates a new PathTrie with the given configuration.
// If config is nil, DefaultTrieConfig() is used.
// If PruneInterval is configured, a background goroutine is started to prune stale patterns.
// Call Stop() when done with the trie to stop the background goroutine.
func NewPathTrie(config *TrieConfig) (*PathTrie, error) {
	if config == nil {
		config = DefaultTrieConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	t := &PathTrie{
		root: &pathNode{
			segment:  "",
			children: make(map[string]*pathNode),
			depth:    -1, // root is at depth -1, so children are at depth 0
		},
		cfg: config,
	}

	return t, nil
}

// Start starts the background routine for pruning stale paths from the trie.
// Must call `Stop` when you want to clean up the trie to avoid goroutine leaks.
func (t *PathTrie) Start() {
	// Start background pruning if configured
	if t.cfg.PruneInterval > 0 && t.cfg.PatternTTL > 0 {
		t.stopPrune = make(chan struct{})
		t.pruneWg.Add(1)
		go t.startPruneLoop(t.stopPrune)
	}
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

	// Trim leading/trailing separators and split
	path = strings.Trim(path, t.cfg.Separator)
	if path == "" {
		return nil
	}

	return strings.Split(path, t.cfg.Separator)
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

	result, changed := t.insertSegments(segments)

	// Only update pattern count and enforce limit if the trie was modified
	if changed {
		t.updatePatternCount()
		t.enforcePatternLimit()
	}

	return t.cfg.Separator + strings.Join(result, t.cfg.Separator)
}

// insertSegments inserts segments into the trie and returns the resulting path
// and whether the trie was structurally modified (new nodes created or nodes collapsed).
func (t *PathTrie) insertSegments(segments []string) ([]string, bool) {
	current := t.root
	result := make([]string, 0, len(segments))
	changed := false

	// We start at depth=0, with current=root (depth=-1).
	// This means current.depth is always `depth-1`.
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
			current = t.getOrCreateWildcardChild(current)
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

			// Check if this is a new unique segment we haven't seen before
			if _, seen := current.wildcardedSegments[segment]; !seen {
				changed = true
				current.wildcardedSegments[segment] = struct{}{}
				current.uniqueChildrenSeen++

				// Check if we've hit the hard threshold
				hardMax := t.getHardMaxCardinality(depth)
				if current.uniqueChildrenSeen > hardMax {
					t.hardCollapseNode(current)
					result = append(result, t.cfg.ReplaceWith)
					current = current.children[t.cfg.ReplaceWith]
					continue
				}
			}

			// Use wildcard
			result = append(result, t.cfg.ReplaceWith)
			current = t.getOrCreateWildcardChild(current)
			continue
		}

		// Case 3: Normal operation - not yet soft collapsed
		child, exists := current.children[segment]

		if exists {
			result = append(result, segment)
			current = child
			continue
		}

		// New segment - trie is being modified
		changed = true
		current.uniqueChildrenSeen++
		softMax := t.getSoftMaxCardinality(depth)

		if current.uniqueChildrenSeen > softMax {
			// Hit soft threshold - mark as soft collapsed
			current.softCollapsed = true

			// Track the triggering segment so it won't be counted again
			if current.wildcardedSegments == nil {
				current.wildcardedSegments = make(map[string]struct{})
			}
			current.wildcardedSegments[segment] = struct{}{}

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
			current = t.getOrCreateWildcardChild(current)
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

	// Update lastSeen on the final node for TTL tracking
	if current != t.root {
		current.lastSeen = time.Now()
	}

	return result, changed
}

// getOrCreateWildcardChild returns the wildcard child of a node, creating it if needed.
func (t *PathTrie) getOrCreateWildcardChild(node *pathNode) *pathNode {
	if node.children[t.cfg.ReplaceWith] == nil {
		node.children[t.cfg.ReplaceWith] = &pathNode{
			segment:    t.cfg.ReplaceWith,
			children:   make(map[string]*pathNode),
			isWildcard: true,
			depth:      node.depth + 1,
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
	node.wildcardedSegments = nil // no longer needed after hard collapse

	wildcardChild := t.getOrCreateWildcardChild(node)

	// Merge all explicit children into the wildcard node
	for segment, child := range node.children {
		if segment == t.cfg.ReplaceWith {
			continue // Skip the wildcard itself
		}
		t.mergeChildren(wildcardChild, child)
	}

	// Replace all children with just the wildcard
	node.children = map[string]*pathNode{
		t.cfg.ReplaceWith: wildcardChild,
	}

	// Recursively check if wildcard node needs hard collapsing
	// Use default thresholds for merged nodes
	if wildcardChild.uniqueChildrenSeen > t.cfg.HardMaxCardinality {
		t.hardCollapseNode(wildcardChild)
	}
}

// mergeChildren merges children from source into target.
// This is called during hard collapse to combine all child paths.
// TODO(goutham): Verify this works well.
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
// Root is not counted as a pattern.
func (t *PathTrie) countPatterns(node *pathNode) int {
	if node == nil {
		return 0
	}
	// Don't count root as a pattern
	if len(node.children) == 0 && node != t.root {
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

// PruneStale removes leaf patterns that haven't been seen since the given cutoff time.
// Returns the number of patterns pruned. Thread-safe.
func (t *PathTrie) PruneStale(cutoff time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cfg.PatternTTL == 0 {
		return 0 // TTL disabled
	}

	pruned := t.pruneNode(t.root, cutoff)
	if pruned > 0 {
		t.updatePatternCount()
	}
	return pruned
}

// pruneNode recursively prunes stale nodes from the given node's subtree.
// Returns the number of leaf patterns pruned.
// Must be called with lock held.
func (t *PathTrie) pruneNode(node *pathNode, cutoff time.Time) int {
	if node == nil {
		return 0
	}

	pruned := 0
	toDelete := make([]string, 0)

	for segment, child := range node.children {
		// First, recursively prune children
		pruned += t.pruneNode(child, cutoff)

		// Check if this child should be pruned:
		// 1. It's a leaf (no children) AND
		// 2. Either it has a stale lastSeen, OR it has no lastSeen (empty intermediate node)
		if len(child.children) == 0 {
			// If lastSeen is zero, this is an intermediate node that became empty after pruning
			// If lastSeen is before cutoff, this is a stale leaf
			if child.lastSeen.IsZero() || child.lastSeen.Before(cutoff) {
				toDelete = append(toDelete, segment)
				// Only count as pruned if it was a real leaf (had lastSeen set)
				if !child.lastSeen.IsZero() {
					pruned++
				}
			}
		}
	}

	// Delete stale/empty children
	for _, segment := range toDelete {
		delete(node.children, segment)
	}

	return pruned
}

// startPruneLoop runs the background pruning loop.
// It periodically calls PruneStale to remove patterns that haven't been seen recently.
func (t *PathTrie) startPruneLoop(stopChan chan struct{}) {
	defer t.pruneWg.Done()

	ticker := time.NewTicker(t.cfg.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.PruneStale(time.Now().Add(-t.cfg.PatternTTL))
		case <-stopChan:
			return
		}
	}
}

// Stop stops the background pruning goroutine if it was started.
// Call this when done with the trie to clean up resources.
// It is safe to call Stop multiple times or on a trie without background pruning.
func (t *PathTrie) Stop() {
	// If we do a simple defer t.mu.Unlock(), then the background routine in prune
	// can trigger a PruneStale() that also tries to acquire the lock. That routine
	// will be stuck, and this current routine will be stuck on `t.pruneWg.Wait()`
	// as well.
	t.mu.Lock()
	stopChan := t.stopPrune
	t.stopPrune = nil // prevent double close
	t.mu.Unlock()     // Release lock BEFORE waiting.

	if stopChan != nil {
		close(stopChan)
		t.pruneWg.Wait()
	}
}
