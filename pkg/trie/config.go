package trie

import (
	"fmt"
	"time"
)

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

	// MaxPatterns is the global maximum number of unique patterns allowed.
	// When exceeded, the deepest soft-collapsed nodes are hard-collapsed first.
	// Default: 1000 (0 = no limit)
	MaxPatterns int `json:"max_patterns"`

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

	// PatternTTL is the time after which unused patterns are eligible for pruning.
	// Patterns not accessed (via Insert) within this duration will be removed.
	// Default: 10 minutes (0 = no expiration)
	PatternTTL time.Duration `json:"pattern_ttl"`

	// PruneInterval is how often the background pruning goroutine runs.
	// Only used if PatternTTL > 0.
	// Default: 60 seconds (0 = no background pruning)
	PruneInterval time.Duration `json:"prune_interval"`
}

// DefaultTrieConfig returns a sensible default configuration.
func DefaultTrieConfig() *TrieConfig {
	return &TrieConfig{
		SoftMaxCardinality:     10,
		HardMaxCardinality:     100,
		MaxPatterns:            1000,
		DepthSoftCardinalities: nil,
		DepthHardCardinalities: nil,
		ReplaceWith:            "*",
		Separator:              "/",
		MaxDepth:               20,
		PatternTTL:             10 * time.Minute,
		PruneInterval:          60 * time.Second,
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
	if c.MaxPatterns < 0 {
		return fmt.Errorf("MaxPatterns must be >= 0, got %d", c.MaxPatterns)
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
	if c.PatternTTL < 0 {
		return fmt.Errorf("PatternTTL cannot be negative, got %v", c.PatternTTL)
	}
	if c.PruneInterval < 0 {
		return fmt.Errorf("PruneInterval cannot be negative, got %v", c.PruneInterval)
	}
	if c.PruneInterval > 0 && c.PatternTTL == 0 {
		return fmt.Errorf("PruneInterval requires PatternTTL to be set")
	}
	return nil
}
