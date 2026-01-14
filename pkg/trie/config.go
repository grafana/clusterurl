package trie

import "fmt"

// TrieConfig configures the behavior of a PathTrie.
type TrieConfig struct {
	// DefaultMaxCardinality is the default threshold for all depths.
	// When a node's children exceed this count, they are collapsed to a wildcard.
	// Default: 100
	DefaultMaxCardinality int `json:"default_max_cardinality"`

	// DepthCardinalities allows per-depth cardinality overrides.
	// Key is the depth (0-indexed, where 0 is the first segment after root),
	// value is the max cardinality for that depth.
	// Example: {0: 10, 1: 10} means depths 0 and 1 allow 10 children each.
	// Depths not in the map use DefaultMaxCardinality.
	DepthCardinalities map[int]int `json:"depth_cardinalities,omitempty"`

	// ReplaceWith is the wildcard string used for collapsed segments.
	// Default: "*"
	ReplaceWith string `json:"replace_with"`

	// Separator is the path segment separator.
	// Default: "/"
	Separator string `json:"separator"`

	// MaxDepth limits the maximum depth of the trie to prevent unbounded growth.
	// Segments beyond this depth are still processed but appended to the result.
	// Default: 20
	MaxDepth int `json:"max_depth"`
}

// DefaultTrieConfig returns a sensible default configuration.
func DefaultTrieConfig() *TrieConfig {
	return &TrieConfig{
		DefaultMaxCardinality: 100,
		DepthCardinalities:    nil,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	}
}

// Validate checks the configuration for errors.
func (c *TrieConfig) Validate() error {
	if c.DefaultMaxCardinality <= 0 {
		return fmt.Errorf("DefaultMaxCardinality must be positive, got %d", c.DefaultMaxCardinality)
	}
	if c.ReplaceWith == "" {
		return fmt.Errorf("ReplaceWith cannot be empty")
	}
	if c.Separator == "" {
		return fmt.Errorf("Separator cannot be empty")
	}
	if c.MaxDepth <= 0 {
		return fmt.Errorf("MaxDepth must be positive, got %d", c.MaxDepth)
	}
	if c.MaxDepth > 100 {
		return fmt.Errorf("MaxDepth cannot exceed 100, got %d", c.MaxDepth)
	}
	for depth, card := range c.DepthCardinalities {
		if depth < 0 {
			return fmt.Errorf("DepthCardinalities: depth cannot be negative, got %d", depth)
		}
		if card == 0 || card < -1 {
			return fmt.Errorf("DepthCardinalities: cardinality for depth %d must be positive or -1 (no limit), got %d", depth, card)
		}
	}
	return nil
}
