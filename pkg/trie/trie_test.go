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
		assert.Equal(t, 100, trie.cfg.DefaultMaxCardinality)
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := &TrieConfig{
			DefaultMaxCardinality: 50,
			ReplaceWith:           "?",
			Separator:             "/",
			MaxDepth:              10,
		}
		trie, err := NewPathTrie(cfg)
		require.NoError(t, err)
		assert.Equal(t, 50, trie.cfg.DefaultMaxCardinality)
		assert.Equal(t, "?", trie.cfg.ReplaceWith)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &TrieConfig{
			DefaultMaxCardinality: 0, // invalid
			ReplaceWith:           "*",
			Separator:             "/",
			MaxDepth:              20,
		}
		_, err := NewPathTrie(cfg)
		assert.Error(t, err)
	})
}

func TestPathTrie_BasicInsertAndLookup(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	})
	require.NoError(t, err)

	// Insert first path
	result := trie.Insert("test/bar-attach-generic-product-apjkmyp/files/multi-test-version-jwbCm/test")
	assert.Equal(t, "/test/bar-attach-generic-product-apjkmyp/files/multi-test-version-jwbCm/test", result)

	// Insert second path with different second segment
	result = trie.Insert("test/apjkmyp/files/jwbCm/test")
	assert.Equal(t, "/test/apjkmyp/files/jwbCm/test", result)

	// Insert third path - should trigger collapse at second segment (cardinality > 2)
	result = trie.Insert("test/xyz/files/abc/test")
	assert.Equal(t, "/test/*/files/*/test", result)

	// Lookup should now return collapsed path
	result = trie.Lookup("test/anything-new/files/something/test")
	assert.Equal(t, "/test/*/files/*/test", result)
}

func TestPathTrie_CardinalityThreshold(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 3,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	})
	require.NoError(t, err)

	// Add paths up to threshold
	assert.Equal(t, "/api/v1/users", trie.Insert("api/v1/users"))
	assert.Equal(t, "/api/v2/users", trie.Insert("api/v2/users"))
	assert.Equal(t, "/api/v3/users", trie.Insert("api/v3/users"))

	// Next insert should trigger collapse
	assert.Equal(t, "/api/*/users", trie.Insert("api/v4/users"))

	// Verify lookup uses collapsed path
	assert.Equal(t, "/api/*/users", trie.Lookup("api/v999/users"))
}

func TestPathTrie_DepthBasedCardinality(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		DepthCardinalities: map[int]int{
			0: 5, // First segment allows 5 unique values
			1: 3, // Second segment allows 3 unique values
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})
	require.NoError(t, err)

	// Add 5 different first segments - should not collapse
	for i := 0; i < 5; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i))
	}

	// 6th first segment should trigger collapse
	result := trie.Insert("segment5/sub/path")
	assert.Equal(t, "/*/sub/path", result)
}

func TestPathTrie_DepthBasedCardinality_NoLimit(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		DepthCardinalities: map[int]int{
			0: -1, // First segment never collapses
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})
	require.NoError(t, err)

	// Add many first segments - should never collapse due to -1
	for i := 0; i < 100; i++ {
		result := trie.Insert("segment" + strconv.Itoa(i) + "/sub/path")
		assert.Contains(t, result, "segment"+strconv.Itoa(i), "depth 0 should never collapse with -1")
	}

	// But second level should still collapse at 2
	trie2, _ := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		DepthCardinalities: map[int]int{
			0: -1, // First segment never collapses
		},
		ReplaceWith: "*",
		Separator:   "/",
		MaxDepth:    20,
	})

	trie2.Insert("api/v1/users")
	trie2.Insert("api/v2/users")
	result := trie2.Insert("api/v3/users")
	assert.Equal(t, "/api/*/users", result, "depth 1 should still collapse at default cardinality")
}

func TestPathTrie_CascadingCollapse(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	})
	require.NoError(t, err)

	// Build tree: /root/child1/grandchild1
	//             /root/child1/grandchild2
	//             /root/child2/grandchild3
	trie.Insert("root/child1/grandchild1")
	trie.Insert("root/child1/grandchild2")
	trie.Insert("root/child2/grandchild3")

	// This should trigger collapse at "child" level
	// which should cascade to grandchildren
	result := trie.Insert("root/child3/grandchild4")

	// After collapse, all should be wildcards
	assert.Equal(t, "/root/*/*", result)
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

func TestPathTrie_TrailingSlash(t *testing.T) {
	trie, err := NewPathTrie(nil)
	require.NoError(t, err)

	result1 := trie.Insert("/api/users/")
	result2 := trie.Insert("/api/users")

	// Both should normalize to the same path
	assert.Equal(t, result1, result2)
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

func TestPathTrie_PreserveExistingPaths(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 2,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	})
	require.NoError(t, err)

	// Insert paths
	trie.Insert("api/users/123")
	trie.Insert("api/users/456")

	// Before collapse, lookups should return exact matches
	assert.Equal(t, "/api/users/123", trie.Lookup("api/users/123"))
	assert.Equal(t, "/api/users/456", trie.Lookup("api/users/456"))

	// Trigger collapse
	trie.Insert("api/users/789")

	// After collapse, all should use wildcard
	assert.Equal(t, "/api/users/*", trie.Lookup("api/users/123"))
	assert.Equal(t, "/api/users/*", trie.Lookup("api/users/999"))
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
		DefaultMaxCardinality: 10,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
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

func TestPathTrie_MaxDepth(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 100,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              3,
	})
	require.NoError(t, err)

	// Path with more segments than MaxDepth
	result := trie.Insert("/a/b/c/d/e/f")
	assert.Equal(t, "/a/b/c/d/e/f", result)

	// Segments beyond MaxDepth are preserved
	result = trie.Lookup("/a/b/c/x/y/z")
	assert.Equal(t, "/a/b/c/x/y/z", result)
}

func TestPathTrie_ComplexPaths(t *testing.T) {
	trie, err := NewPathTrie(&TrieConfig{
		DefaultMaxCardinality: 3,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	})
	require.NoError(t, err)

	paths := []string{
		"bar/test/test/bar-attach-generic-product-apjkmyp/files/multi-test-version-jwbCm/test",
		"bar/test/test/bar-attach-generic-registry-apjkmyp/files/push-metrics-test-OYboK/test",
		"bar/test/test/another-product-xyz/files/version-abc/test",
	}

	for _, path := range paths {
		trie.Insert(path)
	}

	// Should not collapse yet (cardinality = 3, threshold = 3)
	result := trie.Lookup(paths[0])
	assert.Contains(t, result, "bar-attach-generic-product-apjkmyp")

	// Fourth path should trigger collapse
	trie.Insert("bar/test/test/fourth-product/files/version-def/test")

	result = trie.Lookup("bar/test/test/any-product/files/any-version/test")
	assert.Equal(t, "/bar/test/test/*/files/*/test", result)
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
			name: "zero cardinality",
			config: &TrieConfig{
				DefaultMaxCardinality: 0,
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "negative cardinality",
			config: &TrieConfig{
				DefaultMaxCardinality: -1,
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "empty replace with",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				ReplaceWith:           "",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "empty separator",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				ReplaceWith:           "*",
				Separator:             "",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "zero max depth",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              0,
			},
			wantErr: true,
		},
		{
			name: "max depth too large",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              101,
			},
			wantErr: true,
		},
		{
			name: "negative depth in DepthCardinalities",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				DepthCardinalities:    map[int]int{-1: 50},
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "zero cardinality in DepthCardinalities",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				DepthCardinalities:    map[int]int{0: 0},
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "invalid negative cardinality in DepthCardinalities",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				DepthCardinalities:    map[int]int{0: -2},
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: true,
		},
		{
			name: "valid depth cardinalities",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				DepthCardinalities:    map[int]int{0: 50, 1: 30, 2: 20},
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
			},
			wantErr: false,
		},
		{
			name: "-1 for no limit is valid",
			config: &TrieConfig{
				DefaultMaxCardinality: 100,
				DepthCardinalities:    map[int]int{0: -1, 1: 50},
				ReplaceWith:           "*",
				Separator:             "/",
				MaxDepth:              20,
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
		DefaultMaxCardinality: 100,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
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
		DefaultMaxCardinality: 100,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
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
			DefaultMaxCardinality: 10,
			ReplaceWith:           "*",
			Separator:             "/",
			MaxDepth:              20,
		})
		for _, path := range paths {
			trie.Insert(path)
		}
	}
}
