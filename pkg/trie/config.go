package trie

import "fmt"

// TrieConfig configures the behavior of a PathTrie.
type TrieConfig struct {
	// SoftMaxCardinality is the soft threshold for all depths.
	// After this many unique children, NEW children are mapped to wildcard,
	// but existing children are preserved.
	// Default: 10
	SoftMaxCardinality int `json:"soft_max_cardinality"`

	// HardMaxCardinality is the hard threshold for all depths.
	// After this many unique children have been seen, ALL children are
	// collapsed to a single wildcard (including previously preserved ones).
	// Default: 100
	HardMaxCardinality int `json:"hard_max_cardinality"`

	// DepthSoftCardinalities allows per-depth soft cardinality overrides.
	// Key is the depth (0-indexed), value is the soft max cardinality.
	// Use -1 for no limit at a specific depth.
	// Depths not in the map use SoftMaxCardinality.
	DepthSoftCardinalities map[int]int `json:"depth_soft_cardinalities,omitempty"`

	// DepthHardCardinalities allows per-depth hard cardinality overrides.
	// Key is the depth (0-indexed), value is the hard max cardinality.
	// Use -1 for no limit at a specific depth.
	// Depths not in the map use HardMaxCardinality.
	DepthHardCardinalities map[int]int `json:"depth_hard_cardinalities,omitempty"`

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
		SoftMaxCardinality:     10,
		HardMaxCardinality:     100,
		DepthSoftCardinalities: nil,
		DepthHardCardinalities: nil,
		ReplaceWith:            "*",
		Separator:              "/",
		MaxDepth:               20,
	}
}

// Validate checks the configuration for errors.
func (c *TrieConfig) Validate() error {
	if c.SoftMaxCardinality <= 0 {
		return fmt.Errorf("SoftMaxCardinality must be positive, got %d", c.SoftMaxCardinality)
	}
	if c.HardMaxCardinality <= 0 {
		return fmt.Errorf("HardMaxCardinality must be positive, got %d", c.HardMaxCardinality)
	}
	if c.HardMaxCardinality < c.SoftMaxCardinality {
		return fmt.Errorf("HardMaxCardinality (%d) must be >= SoftMaxCardinality (%d)",
			c.HardMaxCardinality, c.SoftMaxCardinality)
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
	for depth, card := range c.DepthSoftCardinalities {
		if depth < 0 {
			return fmt.Errorf("DepthSoftCardinalities: depth cannot be negative, got %d", depth)
		}
		if card == 0 || card < -1 {
			return fmt.Errorf("DepthSoftCardinalities: cardinality for depth %d must be positive or -1 (no limit), got %d", depth, card)
		}
	}
	for depth, card := range c.DepthHardCardinalities {
		if depth < 0 {
			return fmt.Errorf("DepthHardCardinalities: depth cannot be negative, got %d", depth)
		}
		if card == 0 || card < -1 {
			return fmt.Errorf("DepthHardCardinalities: cardinality for depth %d must be positive or -1 (no limit), got %d", depth, card)
		}
	}
	return nil
}
