package trie

import (
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathTrie_SoftThreshold(t *testing.T) {
	// Soft=3, Hard=10: First 3 children explicit, 4th+ go to wildcard
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 3,
		HardMaxCardinality: 10,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// First 3 should be explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))
	assert.Equal(t, "/api/v3/users", trie.Insert("api/v3/users"))

	// 4th should go to wildcard (soft collapse)
	assert.Equal(t, "/api/*/users", trie.Insert("api/v4/users"))

	// But existing paths should still be explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))
	assert.Equal(t, "/api/v3/users", trie.Insert("api/v3/users"))

	// New paths should go to wildcard
	assert.Equal(t, "/api/*/users", trie.Insert("api/v5/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v999/users"))
}

func TestPathTrie_HardThreshold(t *testing.T) {
	// Soft=2, Hard=5: After 5 unique children, everything collapses
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 5,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// First 2 explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))

	// Next go to wildcard (soft collapse)
	assert.Equal(t, "/api/*/users", trie.Insert("api/v3/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v4/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v5/users"))

	// v1 and v2 should still be explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))

	// 6th unique triggers hard collapse - now even v1, v2 become wildcard
	assert.Equal(t, "/api/*/users", trie.Insert("api/v6/users"))

	// Now ALL lookups should return wildcard
	assert.Equal(t, "/api/*/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v2/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v999/users"))
}

func TestPathTrie_DepthBasedThresholds(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 5,
		DepthSoftCardinalities: map[int]int{
			0: 5, // First segment allows 5 explicit
		},
		DepthHardCardinalities: map[int]int{
			0: 20, // First segment allows 20 before hard collapse
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})
	require.NoError(t, err)

	// Add 5 different first segments - should all be explicit
	for i := 0; i < 5; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i))
	}

	// 6th first segment should go to wildcard (soft collapse at depth 0)
	result := trie.Insert("segment5/sub/path")
	assert.Equal(t, "/*/sub/path", result)

	// But first 5 should still be explicit
	for i := 0; i < 5; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i))
	}

	// Test hard collapse limits.
	for i := 0; i < 20; i++ {
		trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
	}
	for i := 0; i < 5; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i))
	}
	trie.Insert("segment20/sub/path")
	for i := 0; i < 5; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Equal(t, "/*/sub/path", result)
	}
}

func TestPathTrie_NoLimitWithMinusOne(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 5,
		DepthSoftCardinalities: map[int]int{
			0: -1, // First segment never soft collapses
		},
		DepthHardCardinalities: map[int]int{
			0: -1, // First segment never hard collapses
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})
	require.NoError(t, err)

	// Add many first segments - should never collapse
	for i := 0; i < 100; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i), "depth 0 should never collapse with -1")
	}

	// But second level should still collapse at soft=2
	trie2, _ := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 5,
		DepthSoftCardinalities: map[int]int{
			0: -1,
		},
		DepthHardCardinalities: map[int]int{
			0: -1,
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})

	trie2.Insert("api/v1/users")
	trie2.Insert("api/v2/users")
	result := trie2.Insert("api/v3/users")
	assert.Equal(t, "/api/*/users", result, "depth 1 should still soft collapse")

	// v1 and v2 should still be explicit
	assert.Equal(t, "/api/v1/users", trie2.Lookup("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie2.Lookup("api/v2/users"))
}

func TestPathTrie_EmptyPath(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	assert.Empty(t, trie.Insert(""))
	assert.Empty(t, trie.Lookup(""))
}

func TestPathTrie_SingleSegment(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	result := trie.Insert("test")
	assert.Equal(t, "/test", result)

	result = trie.Lookup("test")
	assert.Equal(t, "/test", result)
}

func TestPathTrie_QueryString(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	result := trie.Insert("/attach?session_id=ddfsdsf&track_id=sjdklnfldsn")
	assert.Equal(t, "/attach", result)
}

func TestPathTrie_Reset(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	trie.Insert("/api/v1/users")
	trie.Insert("/api/v2/products")

	assert.Greater(t, trie.NodeCount(), 1)

	trie.Reset()

	assert.Equal(t, 1, trie.NodeCount()) // Only root node
}

func TestPathTrie_NodeCount(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	assert.Equal(t, 1, trie.NodeCount()) // Root only

	trie.Insert("/a/b/c")
	assert.Equal(t, 4, trie.NodeCount()) // Root + a + b + c

	trie.Insert("/a/b/d")
	assert.Equal(t, 5, trie.NodeCount()) // Root + a + b + c + d
}

func TestPathTrie_Concurrent(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 10,
		HardMaxCardinality: 100,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	numGoroutines := 100
	insertsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < insertsPerGoroutine; j++ {
				path := "/api/v" + strconv.Itoa(id) + "/resource/" + strconv.Itoa(j)
				trie.Insert(path)
				trie.Lookup(path)
			}
		}(i)
	}

	wg.Wait()

	// Verify trie is still functional
	result := trie.Lookup("/api/v0/resource/0")
	assert.NotEmpty(t, result)
}

func TestPathTrie_HardThreshold_UniqueSegmentCounting(t *testing.T) {
	// Verify that repeated segments don't count multiple times toward hard threshold
	// Soft=2, Hard=5: hard collapse should only trigger after 5 UNIQUE children
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 5,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// First 2 explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))

	// v3 goes to wildcard (soft collapse)
	assert.Equal(t, "/api/*/users", trie.Insert("api/v3/users"))

	// Insert v3 many more times - should NOT count toward hard threshold
	for i := 0; i < 100; i++ {
		assert.Equal(t, "/api/*/users", trie.Insert("api/v3/users"))
	}

	// v1 and v2 should still be explicit (not hard collapsed yet)
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))

	// Now add v4 and v5 (still under hard threshold)
	assert.Equal(t, "/api/*/users", trie.Insert("api/v4/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v5/users"))

	// v1 and v2 should STILL be explicit
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))

	// v6 triggers hard collapse (6th unique child > hard threshold of 5)
	assert.Equal(t, "/api/*/users", trie.Insert("api/v6/users"))

	// Now v1 and v2 should be wildcarded
	assert.Equal(t, "/api/*/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/*/users", trie.Insert("api/v2/users"))
}

func TestPathTrie_CascadingCollapse(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 3,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// Build tree with multiple branches
	trie.Insert("root/child1/grandchild1")
	trie.Insert("root/child1/grandchild2")
	trie.Insert("root/child2/grandchild3")

	// This triggers soft collapse at child level
	result := trie.Insert("root/child3/grandchild4")
	assert.Equal(t, "/root/*/grandchild4", result)

	// Trigger hard collapse
	result = trie.Insert("root/child4/grandchild5")
	assert.Equal(t, "/root/*/*", result)

	// After hard collapse, all lookups should return wildcard
	assert.Equal(t, "/root/*/*", trie.Lookup("root/child1/grandchild1"))
	assert.Equal(t, "/root/*/*", trie.Lookup("root/anything/else"))
}

func TestTrieConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *TrieConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultTrieConfig(),
			wantErr: false,
		},
		{
			name: "zero soft cardinality",
			config: &TrieConfig{
				SoftMaxCardinality: 0,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
			},
			wantErr: true,
		},
		{
			name: "zero hard cardinality",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 0,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
			},
			wantErr: true,
		},
		{
			name: "hard less than soft",
			config: &TrieConfig{
				SoftMaxCardinality: 50,
				HardMaxCardinality: 10,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
			},
			wantErr: true,
		},
		{
			name: "empty replace with",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "",
				Separator:          "/",
				MaxDepth:           20,
			},
			wantErr: true,
		},
		{
			name: "valid with depth overrides",
			config: &TrieConfig{
				SoftMaxCardinality:     10,
				HardMaxCardinality:     100,
				DepthSoftCardinalities: map[int]int{0: 5, 1: 3},
				DepthHardCardinalities: map[int]int{0: 50, 1: 30},
				ReplaceWith:            "*",
				Separator:              "/",
				MaxDepth:               20,
			},
			wantErr: false,
		},
		{
			name: "-1 for no limit is valid",
			config: &TrieConfig{
				SoftMaxCardinality:     10,
				HardMaxCardinality:     100,
				DepthSoftCardinalities: map[int]int{0: -1},
				DepthHardCardinalities: map[int]int{0: -1},
				ReplaceWith:            "*",
				Separator:              "/",
				MaxDepth:               20,
			},
			wantErr: false,
		},
		{
			name: "negative MaxPatterns",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				MaxPatterns:        -1,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
			},
			wantErr: true,
		},
		{
			name: "valid with TTL and prune interval",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         time.Hour,
				PruneInterval:      time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid with no TTL or pruning",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         0,
				PruneInterval:      0,
			},
			wantErr: false,
		},
		{
			name: "valid with TTL only (no background pruning)",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         time.Hour,
				PruneInterval:      0,
			},
			wantErr: false,
		},
		{
			name: "invalid prune interval without TTL",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         0,
				PruneInterval:      time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid negative TTL",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         -time.Hour,
				PruneInterval:      0,
			},
			wantErr: true,
		},
		{
			name: "invalid negative prune interval",
			config: &TrieConfig{
				SoftMaxCardinality: 10,
				HardMaxCardinality: 100,
				ReplaceWith:        "*",
				Separator:          "/",
				MaxDepth:           20,
				PatternTTL:         time.Hour,
				PruneInterval:      -time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Tests for global pattern limit (MaxPatterns)

func TestPathTrie_MaxPatterns_UnderLimit(t *testing.T) {
	// Test that patterns under the limit are not affected
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 5,
		HardMaxCardinality: 100,
		MaxPatterns:        20,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// Insert paths that create patterns under the limit
	for i := 0; i < 5; i++ {
		trie.Insert("/api/v" + strconv.Itoa(i) + "/users")
	}

	// All should be explicit since under both soft threshold and max patterns
	for i := 0; i < 5; i++ {
		result := trie.Lookup("/api/v" + strconv.Itoa(i) + "/users")
		assert.Contains(t, result, "v"+strconv.Itoa(i), "should remain explicit when under limits")
	}

	assert.Equal(t, trie.PatternCount(), 5)

	// Insert more paths, however, the patternCount() goes up by only 1 after softMax and before hardMax.
	for i := 0; i < 100; i++ {
		trie.Insert("/api/v" + strconv.Itoa(i) + "/users")
	}
	assert.Equal(t, trie.PatternCount(), 6)

	trie.Insert("api/v100/users")
	assert.Equal(t, trie.PatternCount(), 1)
}

func TestPathTrie_MaxPatterns_EnforceLimit(t *testing.T) {
	// Test that exceeding MaxPatterns triggers hard collapse of deepest nodes
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 3,
		HardMaxCardinality: 100, // High hard limit so it doesn't interfere
		MaxPatterns:        10,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// Insert many paths at different depths to create many patterns
	// These will create paths like /a/b1/c1, /a/b1/c2, etc.
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			trie.Insert("/a/b" + strconv.Itoa(i) + "/c" + strconv.Itoa(j))
		}
	}

	// Pattern count should be enforced to stay at or below MaxPatterns
	assert.LessOrEqual(t, trie.PatternCount(), 10, "pattern count should not exceed MaxPatterns")
}

func TestPathTrie_MaxPatterns_DeepestCollapsedFirst(t *testing.T) {
	// Test that deeper soft-collapsed nodes are hard-collapsed before shallower ones
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 2,
		HardMaxCardinality: 100,
		MaxPatterns:        5,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// Create a structure with soft-collapsed nodes at different depths
	// First, create explicit paths at depth 1 (under soft threshold)
	trie.Insert("/api/v1/resource/item1")
	trie.Insert("/api/v1/resource/item2")
	trie.Insert("/api/v2/resource/item1")
	trie.Insert("/api/v2/resource/item2")

	// These will cause soft collapse at depth 2 (under /api/v1/resource/)
	trie.Insert("/api/v1/resource/item3")
	trie.Insert("/api/v1/resource/item4")

	// When pattern limit is exceeded, deeper nodes should collapse first
	// The node at depth 2 (resource level) should collapse before depth 1
	patternCount := trie.PatternCount()
	assert.LessOrEqual(t, patternCount, 5, "pattern count should be enforced")

	result := trie.Insert("/api/v1/resource/item3")
	assert.Equal(t, "/api/v1/resource/*", result)
}

func TestPathTrie_MaxPatterns_NoCandidates(t *testing.T) {
	// Test that if there are no soft-collapsed candidates, we accept over limit
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100, // Very high so nothing soft collapses
		HardMaxCardinality: 200,
		MaxPatterns:        5,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	// Insert paths - none will soft collapse since soft threshold is high
	for i := 0; i < 10; i++ {
		trie.Insert("/api/v" + strconv.Itoa(i))
	}

	// Since no nodes are soft-collapsed, there are no candidates to collapse
	// Pattern count may exceed limit
	assert.Equal(t, 10, trie.PatternCount(), "should have 10 patterns with no collapse candidates")
}

func TestPathTrie_PatternCount(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})
	require.NoError(t, err)

	assert.Equal(t, 0, trie.PatternCount(), "empty trie should have 0 patterns")

	trie.Insert("/a")
	assert.Equal(t, 1, trie.PatternCount(), "one path should create one pattern")

	trie.Insert("/b")
	assert.Equal(t, 2, trie.PatternCount(), "two paths should create two patterns")

	trie.Insert("/a/b")
	// /a becomes a waypoint, /a/b is the leaf
	// /b is still a leaf
	// So we have: /a/b, /b = 2 patterns? No, /a is still a potential endpoint
	// Actually, patterns = leaves, and /a might not be a leaf anymore if it has children
	// Let's check: after /a/b, node "a" has child "b", so "a" is not a leaf
	// Patterns = /a/b (1) + /b (1) = 2
	// Wait, before we had /a as a leaf. Now /a has a child, so it's not a leaf.
	// So pattern count should still be 2: /a/b and /b
	assert.Equal(t, 2, trie.PatternCount(), "pattern count after adding child")
}

// Tests for TTL-based pattern pruning

func TestPathTrie_PruneStale_TTLDisabled(t *testing.T) {
	// When PatternTTL=0, no pruning should happen
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         0, // disabled
	})
	require.NoError(t, err)

	trie.Insert("/api/v1/users")
	trie.Insert("/api/v2/users")
	assert.Equal(t, 2, trie.PatternCount())

	// Pruning should do nothing when TTL is disabled
	pruned := trie.PruneStale(time.Now().Add(time.Hour))
	assert.Equal(t, 0, pruned, "no patterns should be pruned when TTL is disabled")
	assert.Equal(t, 2, trie.PatternCount())
}

func TestPathTrie_PruneStale_FreshPatternsPreserved(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
	})
	require.NoError(t, err)

	trie.Insert("/api/v1/users")
	trie.Insert("/api/v2/users")
	assert.Equal(t, 2, trie.PatternCount())

	// Cutoff in the past - patterns are fresh
	cutoff := time.Now().Add(-time.Hour)
	pruned := trie.PruneStale(cutoff)
	assert.Equal(t, 0, pruned, "fresh patterns should not be pruned")
	assert.Equal(t, 2, trie.PatternCount())
}

func TestPathTrie_PruneStale_StalePatternsRemoved(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
	})
	require.NoError(t, err)

	trie.Insert("/api/v1/users")
	trie.Insert("/api/v2/users")
	assert.Equal(t, 2, trie.PatternCount())

	// Cutoff in the future - all patterns are stale
	cutoff := time.Now().Add(time.Hour)
	pruned := trie.PruneStale(cutoff)
	assert.Equal(t, 2, pruned, "stale patterns should be pruned")
	assert.Equal(t, 0, trie.PatternCount())
}

func TestPathTrie_PruneStale_ParentCleanup(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
	})
	require.NoError(t, err)

	trie.Insert("/api/v1/users/profile")
	trie.Insert("/api/v1/users/settings")
	initialNodeCount := trie.NodeCount()
	assert.Equal(t, 2, trie.PatternCount())

	// Prune all (cutoff in future)
	cutoff := time.Now().Add(time.Hour)
	pruned := trie.PruneStale(cutoff)
	assert.Equal(t, 2, pruned)
	assert.Equal(t, 0, trie.PatternCount())

	// Node count should be reduced (empty parents cleaned up)
	assert.Less(t, trie.NodeCount(), initialNodeCount, "empty parent nodes should be cleaned up")
}

func TestPathTrie_PruneStale_PartialPrune(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
	})
	require.NoError(t, err)

	// Insert first pattern
	trie.Insert("/api/v1/users")
	time.Sleep(10 * time.Millisecond)
	midPoint := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Insert second pattern later
	trie.Insert("/api/v2/users")
	assert.Equal(t, 2, trie.PatternCount())

	// Prune only the first pattern (cutoff between the two inserts)
	pruned := trie.PruneStale(midPoint)
	assert.Equal(t, 1, pruned, "only older pattern should be pruned")
	assert.Equal(t, 1, trie.PatternCount())

	// v2 should still be accessible
	result := trie.Lookup("/api/v2/users")
	assert.Equal(t, "/api/v2/users", result)
}

func TestPathTrie_Stop_TerminatesGoroutine(t *testing.T) {
	goroutinesBefore := runtime.NumGoroutine()

	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
		PruneInterval:      10 * time.Millisecond,
	})
	require.NoError(t, err)

	trie.Start()

	// Goroutine count should have increased
	assert.Greater(t, runtime.NumGoroutine(), goroutinesBefore, "pruning goroutine should have started")

	trie.Insert("/api/v1/users")

	// Stop should terminate the goroutine
	trie.Stop()

	// Goroutine count should be back to original
	assert.Equal(t, goroutinesBefore, runtime.NumGoroutine(), "pruning goroutine should have stopped")

	// Multiple calls to Stop should be safe
	trie.Stop()
}

func TestPathTrie_Stop_NoGoroutine(t *testing.T) {
	// Stop should be safe to call even when no goroutine was started
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         0, // no pruning
	})
	require.NoError(t, err)

	// Should not panic
	trie.Stop()
}

func TestPathTrie_BackgroundPruning(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 100,
		HardMaxCardinality: 200,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         50 * time.Millisecond,
		PruneInterval:      20 * time.Millisecond,
	})
	require.NoError(t, err)
	trie.Start()
	defer trie.Stop()

	trie.Insert("/api/v1/users")
	assert.Equal(t, 1, trie.PatternCount())

	// Wait for TTL to expire and pruning to occur
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, trie.PatternCount(), "pattern should be pruned by background goroutine")
}

func TestPathTrie_ConcurrentInsertAndPrune(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 10,
		HardMaxCardinality: 100,
		MaxPatterns:        0,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
		PatternTTL:         time.Hour,
	})
	require.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrent inserts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				trie.Insert("/api/v" + strconv.Itoa(id) + "/resource/" + strconv.Itoa(j))
			}
		}(i)
	}

	// Concurrent prunes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				trie.PruneStale(time.Now().Add(-time.Second))
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Trie should still be functional
	result := trie.Lookup("/api/v0/resource/0")
	assert.NotEmpty(t, result)
}

// Benchmarks

func BenchmarkPathTrie_Insert(b *testing.B) {
	trie, _ := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 10,
		HardMaxCardinality: 100,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})

	paths := []string{
		"/users/fdklsd/j4elk/23993/job/2",
		"/v1/products/22",
		"/products/1/org/3",
		"/attach?session_id=ddfsdsf&track_id=sjdklnfldsn",
		"GET /user_space/",
		"/api/hello.world",
		"/123/ljgdflgjf",
		"/a/b/c/d/e",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			trie.Insert(path)
		}
	}
}

func BenchmarkPathTrie_Lookup(b *testing.B) {
	trie, _ := NewPathTrie(&TrieConfig{
		SoftMaxCardinality: 10,
		HardMaxCardinality: 100,
		ReplaceWith:        "*",
		Separator:          "/",
		MaxDepth:           20,
	})

	// Pre-populate trie
	paths := []string{
		"/users/fdklsd/j4elk/23993/job/2",
		"/v1/products/22",
		"/products/1/org/3",
		"/api/hello.world",
		"/a/b/c/d/e",
	}
	for _, path := range paths {
		trie.Insert(path)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			trie.Lookup(path)
		}
	}
}

func BenchmarkPathTrie_InsertWithCollapse(b *testing.B) {
	paths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		paths[i] = "/api/v" + strconv.Itoa(i) + "/resource/" + strconv.Itoa(i%100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie, _ := NewPathTrie(&TrieConfig{
			SoftMaxCardinality: 10,
			HardMaxCardinality: 50,
			ReplaceWith:        "*",
			Separator:          "/",
			MaxDepth:           20,
		})
		for _, path := range paths {
			trie.Insert(path)
		}
	}
}
