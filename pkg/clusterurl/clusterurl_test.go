package clusterurl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterURL(t *testing.T) {
	csf, err := NewClusterURLClassifier(DefaultConfig())
	assert.NoError(t, err)
	assert.Equal(t, "", csf.ClusterURL(""))
	assert.Equal(t, "/users/*/j4elk/*/job/*", csf.ClusterURL("/users/fdklsd/j4elk/23993/job/2"))
	assert.Equal(t, "*", csf.ClusterURL("123"))
	assert.Equal(t, "/*", csf.ClusterURL("/123"))
	assert.Equal(t, "*/", csf.ClusterURL("123/"))
	assert.Equal(t, "*/*", csf.ClusterURL("123/ljgdflgjf"))
	assert.Equal(t, "/*", csf.ClusterURL("/**"))
	assert.Equal(t, "/u/*", csf.ClusterURL("/u/2"))
	assert.Equal(t, "/v1/products/*", csf.ClusterURL("/v1/products/2"))
	assert.Equal(t, "/v1/products/*", csf.ClusterURL("/v1/products/22"))
	assert.Equal(t, "/v1/products/*", csf.ClusterURL("/v1/products/22j"))
	assert.Equal(t, "/products/*/org/*", csf.ClusterURL("/products/1/org/3"))
	assert.Equal(t, "/products//org/*", csf.ClusterURL("/products//org/3"))
	assert.Equal(t, "/v1/k6-test-runs/*", csf.ClusterURL("/v1/k6-test-runs/1"))
	assert.Equal(t, "/attach", csf.ClusterURL("/attach"))
	assert.Equal(t, "/usuarios/*/j4elk/*/trabajo/*", csf.ClusterURL("/usuarios/fdklsd/j4elk/23993/trabajo/2"))
	assert.Equal(t, "/Benutzer/*/j4elk/*/Arbeit/*", csf.ClusterURL("/Benutzer/fdklsd/j4elk/23993/Arbeit/2"))
	assert.Equal(t, "/utilisateurs/*/j4elk/*/tache/*", csf.ClusterURL("/utilisateurs/fdklsd/j4elk/23993/tache/2"))
	assert.Equal(t, "/products/", csf.ClusterURL("/products/"))
	assert.Equal(t, "/user-space/", csf.ClusterURL("/user-space/"))
	assert.Equal(t, "/user_space/", csf.ClusterURL("/user_space/"))
	assert.Equal(t, "/api/hello.world", csf.ClusterURL("/api/hello.world"))
	assert.Equal(t, "/api/hello.world.again", csf.ClusterURL("/api/hello.world.again"))
	assert.Equal(t, "/api.backup/hello.world", csf.ClusterURL("/api.backup/hello.world"))
	assert.Equal(t, "GET /user_space/", csf.ClusterURL("GET /user_space/"))
	assert.Equal(t, "POST /user_space/", csf.ClusterURL("POST /user_space/"))
	assert.Equal(t, "PUT /user_space/", csf.ClusterURL("PUT /user_space/"))
	assert.Equal(t, "DELETE /user_space/", csf.ClusterURL("DELETE /user_space/"))
	assert.Equal(t, "OPTIONS /user_space/", csf.ClusterURL("OPTIONS /user_space/"))
	assert.Equal(t, "HEAD /user_space/", csf.ClusterURL("HEAD /user_space/"))
	assert.Equal(t, "PATCH /user_space/", csf.ClusterURL("PATCH /user_space/"))
	assert.Equal(t, "TRACE /user_space/", csf.ClusterURL("TRACE /user_space/"))
	assert.Equal(t, "CONNECT /user_space/", csf.ClusterURL("CONNECT /user_space/"))
	assert.Equal(t, "/attach", csf.ClusterURL("/attach?session_id=ddfsdsf&track_id=sjdklnfldsn"))
	assert.Equal(t, "/attach", csf.ClusterURL("/attach#section-1"))
	assert.Equal(t, "HTTP GET", csf.ClusterURL("HTTP GET"))
	assert.Equal(t, "GET /api/cart", csf.ClusterURL("GET /api/cart?sessionId=55f4e5ea-5d6d-482a-80c4-799e3c72dfb0&currencyCode=USD"))
	assert.Equal(t, "/getquote", csf.ClusterURL("/getquote"))
	assert.Equal(t, "/w/index.php/*", csf.ClusterURL("/w/index.php/*"))
	assert.Equal(t, "", csf.ClusterURL("?"))
	assert.Equal(t, "*", csf.ClusterURL("attach12?"))
	assert.Equal(t, "*", csf.ClusterURL("1?"))
	assert.Equal(t, "*", csf.ClusterURL("*&"))
	assert.Equal(t, "*", csf.ClusterURL("12#"))
	assert.Equal(t, "/a", csf.ClusterURL("/a#"))
	assert.Equal(t, "/*", csf.ClusterURL("/1#"))
	assert.Equal(t, "a", csf.ClusterURL("a#"))
	assert.Equal(t, "/a/b/c/d/e/f/g/h/i", csf.ClusterURL("/a/b/c/d/e/f/g/h/i/j"))
}

func TestClusterURLWithWordRules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableWordRules = true
	csf, err := NewClusterURLClassifier(cfg)
	assert.NoError(t, err)

	// Allowlisted segments should be preserved
	assert.Equal(t, "/users/*", csf.ClusterURL("/users/123"))
	assert.Equal(t, "/api/users/*", csf.ClusterURL("/api/users/abc123"))

	// Segments after allowlisted resources starting with digit should collapse
	assert.Equal(t, "/users/*", csf.ClusterURL("/users/42"))
	assert.Equal(t, "/accounts/*/settings", csf.ClusterURL("/accounts/12345/settings"))

	// K8s-style names with trailing hash (2+ hyphens, 5-10 char alphanumeric suffix) should collapse
	assert.Equal(t, "/deployments/*", csf.ClusterURL("/deployments/my-app-abc12def"))
	assert.Equal(t, "/pods/*", csf.ClusterURL("/pods/nginx-deployment-5d4f7c8b9"))

	// Hex strings of specific lengths should collapse (7, 8, 12, 40 chars)
	assert.Equal(t, "/commits/*", csf.ClusterURL("/commits/abc1234"))      // 7 hex chars
	assert.Equal(t, "/commits/*", csf.ClusterURL("/commits/abc12345"))     // 8 hex chars
	assert.Equal(t, "/commits/*", csf.ClusterURL("/commits/abc123456789")) // 12 hex chars

	// Too long segments should collapse
	longSegment := "this-is-a-very-long-segment-name-that-exceeds-the-limit"
	assert.Equal(t, "/api/*", csf.ClusterURL("/api/"+longSegment))

	// Non-gibberish, non-allowlisted words should still be preserved by gibberish check
	assert.Equal(t, "/books/harry-potter", csf.ClusterURL("/books/harry-potter"))
	assert.Equal(t, "/movies/star-wars", csf.ClusterURL("/movies/star-wars"))
}

func TestClusterURLWordRulesCustomWordList(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableWordRules = true
	cfg.WordList = &WordList{
		GlobalAllow: toSet([]string{"books", "movies", "authors"}),
		GlobalDeny:  toSet([]string{"internal", "debug"}),
		DepthAllow:  map[int]map[string]struct{}{},
		DepthDeny:   map[int]map[string]struct{}{},
	}
	csf, err := NewClusterURLClassifier(cfg)
	assert.NoError(t, err)

	// Custom allowlist
	assert.Equal(t, "/books/*", csf.ClusterURL("/books/123"))
	assert.Equal(t, "/authors/*", csf.ClusterURL("/authors/456"))

	// Custom denylist - "internal" always collapses
	assert.Equal(t, "/api/*", csf.ClusterURL("/api/internal"))
	assert.Equal(t, "/*/data", csf.ClusterURL("/debug/data"))
}

func TestResourceValueRule(t *testing.T) {
	// This test demonstrates collapsing ANY value immediately after a resource type
	// e.g., /books/harry-potter -> /books/*
	cfg := DefaultConfig()
	cfg.EnableWordRules = true
	cfg.WordList = &WordList{
		GlobalAllow: toSet([]string{"books", "movies", "authors", "genres"}),
		GlobalDeny:  toSet([]string{}),
		DepthAllow:  map[int]map[string]struct{}{},
		DepthDeny:   map[int]map[string]struct{}{},
	}
	// Use ResourceValueRule to collapse anything immediately after allowlisted segments
	cfg.Rules = []CollapseRule{
		DenylistRule{},
		AllowlistRule{},
		ResourceValueRule{}, // This is the key - collapses any value after resource type
		LengthRule{MaxLength: 32},
		PatternRule{},
	}

	csf, err := NewClusterURLClassifier(cfg)
	assert.NoError(t, err)

	// Word values after resource types should collapse
	assert.Equal(t, "/books/*", csf.ClusterURL("/books/harry-potter"))
	assert.Equal(t, "/books/*", csf.ClusterURL("/books/jason-bourne"))
	assert.Equal(t, "/books/*", csf.ClusterURL("/books/the-great-gatsby"))
	assert.Equal(t, "/movies/*", csf.ClusterURL("/movies/star-wars"))
	assert.Equal(t, "/movies/*", csf.ClusterURL("/movies/the-godfather"))

	// Nested resources work too
	assert.Equal(t, "/authors/*/books/*", csf.ClusterURL("/authors/tolkien/books/hobbit"))
	assert.Equal(t, "/books/*/genres/*", csf.ClusterURL("/books/dune/genres/sci-fi"))

	// Resource types themselves are preserved
	assert.Equal(t, "/books/", csf.ClusterURL("/books/"))
	assert.Equal(t, "/api/books/*", csf.ClusterURL("/api/books/something"))
}

func TestNonAllowlistedRule(t *testing.T) {
	// NonAllowlistedRule treats allowlist as the complete vocabulary.
	// Anything NOT in the allowlist collapses, regardless of position.
	cfg := DefaultConfig()
	cfg.EnableWordRules = true
	cfg.WordList = &WordList{
		GlobalAllow: toSet([]string{"api", "v1", "books", "chapters", "authors", "reviews"}),
		GlobalDeny:  toSet([]string{}),
		DepthAllow:  map[int]map[string]struct{}{},
		DepthDeny:   map[int]map[string]struct{}{},
	}
	cfg.Rules = []CollapseRule{
		DenylistRule{},
		NonAllowlistedRule{}, // Key: if not in allowlist, collapse
	}

	csf, err := NewClusterURLClassifier(cfg)
	assert.NoError(t, err)

	// Known vocabulary preserved, everything else collapsed
	assert.Equal(t, "/api/v1/books/*", csf.ClusterURL("/api/v1/books/harry-potter"))
	assert.Equal(t, "/api/v1/books/*", csf.ClusterURL("/api/v1/books/the-great-gatsby"))
	assert.Equal(t, "/api/v1/books/*/chapters/*", csf.ClusterURL("/api/v1/books/dune/chapters/intro"))
	assert.Equal(t, "/books/*/authors/*", csf.ClusterURL("/books/1984/authors/orwell"))
	assert.Equal(t, "/books/*/reviews", csf.ClusterURL("/books/something/reviews"))

	// Multiple unknown segments in a row
	assert.Equal(t, "/api/*/*/books/*", csf.ClusterURL("/api/unknown/stuff/books/title"))

	// All known -> nothing collapses
	assert.Equal(t, "/api/v1/books", csf.ClusterURL("/api/v1/books"))
	assert.Equal(t, "/books/chapters", csf.ClusterURL("/books/chapters"))
}

func TestWordRulesDisabledByDefault(t *testing.T) {
	// Default config should NOT enable word rules
	cfg := DefaultConfig()
	assert.False(t, cfg.EnableWordRules)

	csf, err := NewClusterURLClassifier(cfg)
	assert.NoError(t, err)

	// Without word rules, behavior should match original
	// (gibberish detection only)
	assert.Equal(t, "/users/*/j4elk/*/job/*", csf.ClusterURL("/users/fdklsd/j4elk/23993/job/2"))
}

func BenchmarkClusterURLWithCache(b *testing.B) {
	cfg := DefaultConfig()
	cfg.CacheSize = 1000
	csf, err := NewClusterURLClassifier(cfg)
	if err != nil {
		b.Fatal(err)
	}

	// Test cases representing different scenarios
	testCases := []string{
		"/users/fdklsd/j4elk/23993/job/2",
		"/v1/products/22",
		"/products/1/org/3",
		"/attach?session_id=ddfsdsf&track_id=sjdklnfldsn",
		"GET /user_space/",
		"/api/hello.world",
		"123/ljgdflgjf",
		"",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			_ = csf.ClusterURL(testCase)
		}
	}
}

// Fuzz test to catch panics in ClusterURL
func FuzzClusterURL(f *testing.F) {
	csf, err := NewClusterURLClassifier(DefaultConfig())
	if err != nil {
		f.Fatalf("failed to create classifier: %v", err)
	}

	// Add some interesting seed inputs
	f.Add("")
	f.Add("&?*#")
	f.Add("/users/123/job/456")
	f.Add("123/ljgdflgjf")
	f.Add("/a/b/c/d/e/f/g/h/i/j")
	f.Add("\x00\xff\xfe\xfd") // binary junk
	f.Add("GET /api/cart?sessionId=55f4e5ea-5d6d-482a-80c4-799e3c72dfb0&currencyCode=USD")

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic for input %q: %v", input, r)
			}
		}()
		_ = csf.ClusterURL(input)
	})
}

func BenchmarkClusterURLWithoutCache(b *testing.B) {
	cfg := DefaultConfig()
	cfg.CacheSize = 1
	csf, err := NewClusterURLClassifier(cfg)
	if err != nil {
		b.Fatal(err)
	}

	// Test cases representing different scenarios
	testCases := []string{
		"/users/fdklsd/j4elk/23993/job/2",
		"/v1/products/22",
		"/products/1/org/3",
		"/attach?session_id=ddfsdsf&track_id=sjdklnfldsn",
		"GET /user_space/",
		"/api/hello.world",
		"123/ljgdflgjf",
		"",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, testCase := range testCases {
			_ = csf.ClusterURL(testCase)
		}
	}
}
