package trie

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathTrie(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		trie, err := NewPathTrie(nil)
		require.NoError(t, err)
		assert.NotNil(t, trie)
		assert.Equal(t, 10, trie.cfg.SoftMaxCardinality)
		assert.Equal(t, 100, trie.cfg.HardMaxCardinality)
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := &TrieConfig{
			SoftMaxCardinality: 5,
			HardMaxCardinality: 50,
			ReplaceWith:        "?",
			Separator:          "/",
			MaxDepth:           10,
		}
		trie, err := NewPathTrie(cfg)
		require.NoError(t, err)
		assert.Equal(t, 5, trie.cfg.SoftMaxCardinality)
		assert.Equal(t, 50, trie.cfg.HardMaxCardinality)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &TrieConfig{
			SoftMaxCardinality: 0, // invalid
			HardMaxCardinality: 100,
			ReplaceWith:        "*",
			Separator:          "/",
			MaxDepth:           20,
		}
		_, err := NewPathTrie(cfg)
		assert.Error(t, err)
	})

	t.Run("hard less than soft returns error", func(t *testing.T) {
		cfg := &TrieConfig{
			SoftMaxCardinality: 50,
			HardMaxCardinality: 10, // invalid: less than soft
			ReplaceWith:        "*",
			Separator:          "/",
			MaxDepth:           20,
		}
		_, err := NewPathTrie(cfg)
		assert.Error(t, err)
	})
}

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
	assert.Equal(t, "/api/v1/users", trie.Lookup("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Lookup("api/v2/users"))
	assert.Equal(t, "/api/v3/users", trie.Lookup("api/v3/users"))

	// New paths should go to wildcard
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v5/users"))
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v999/users"))
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
	assert.Equal(t, "/api/v1/users", trie.Lookup("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Lookup("api/v2/users"))

	// 6th unique triggers hard collapse - now even v1, v2 become wildcard
	assert.Equal(t, "/api/*/users", trie.Insert("api/v6/users"))

	// Now ALL lookups should return wildcard
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v1/users"))
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v2/users"))
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v999/users"))
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
		result := trie.Lookup("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i))
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

func TestPathTrie_HTTPMethodPrefix(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	// HTTP method should be stripped
	result := trie.Insert("GET /user_space?kernel_space")
	assert.Equal(t, "/user_space", result)

	// Non-HTTP prefix should be preserved
	result = trie.Insert("MET /user_space")
	assert.Equal(t, "/MET /user_space", result)
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
	trie.Insert("root/child3/grandchild4")

	// Trigger hard collapse
	result := trie.Insert("root/child4/grandchild5")
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
