package clusterurl

import (
	"strings"
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

func BenchmarkClusterURLSafeWithCache(b *testing.B) {
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
			_, _ = csf.ClusterURLSafe(testCase)
		}
	}
}

func BenchmarkClusterURLSafeWithoutCache(b *testing.B) {
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
			_, _ = csf.ClusterURLSafe(testCase)
		}
	}
}

func BenchmarkClusterURLSafeSanitizationEnabled(b *testing.B) {
	cfg := DefaultConfig()
	cfg.EnableSanitization = true
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
			_, _ = csf.ClusterURLSafe(testCase)
		}
	}
}

func BenchmarkClusterURLSafeSanitizationDisabled(b *testing.B) {
	cfg := DefaultConfig()
	cfg.EnableSanitization = false
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
			_, _ = csf.ClusterURLSafe(testCase)
		}
	}
}

// TestClusterURLPanic reproduces a slice manipulation bug
func TestClusterURLPanic(t *testing.T) {
	classifier, err := NewClusterURLClassifier(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create classifier: %v", err)
	}

	// This path triggers the "index out of range [1] with length 1" panic
	problematicPath := strings.Repeat("/segment-with-special-chars!@#$%^&*()", 22) + "?param=value"

	defer func() {
		if r := recover(); r != nil {
			t.Logf("panic occurred: %v", r)
			t.Logf("path: %s", problematicPath)
			t.Logf("length: %d, Segments: %d", len(problematicPath), strings.Count(problematicPath, "/"))
			t.Fail()
		} else {
			t.Logf("no panic occurred - bug fixed")
		}
	}()

	_ = classifier.ClusterURL(problematicPath)
}

func TestClusterURLSafeComprehensive(t *testing.T) {
	csf, err := NewClusterURLClassifier(DefaultConfig())
	assert.NoError(t, err)

	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		description string
	}{
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: false,
			description: "should handle empty string",
		},
		{
			name:        "users with dynamic segments",
			input:       "/users/fdklsd/j4elk/23993/job/2",
			expected:    "/users/*/j4elk/*/job/*",
			expectError: false,
			description: "should cluster dynamic segments",
		},
		{
			name:        "numeric only",
			input:       "123",
			expected:    "*",
			expectError: false,
			description: "should cluster numeric-only strings",
		},
		{
			name:        "numeric with leading slash",
			input:       "/123",
			expected:    "/*",
			expectError: false,
			description: "should cluster numeric with leading slash",
		},
		{
			name:        "numeric with trailing slash",
			input:       "123/",
			expected:    "*/",
			expectError: false,
			description: "should cluster numeric with trailing slash",
		},
		{
			name:        "numeric with text",
			input:       "123/ljgdflgjf",
			expected:    "*/*",
			expectError: false,
			description: "should cluster both segments",
		},
		{
			name:        "double wildcard",
			input:       "/**",
			expected:    "/*",
			expectError: false,
			description: "should handle double wildcard",
		},
		{
			name:        "short user path",
			input:       "/u/2",
			expected:    "/u/*",
			expectError: false,
			description: "should cluster short user path",
		},
		{
			name:        "v1 products",
			input:       "/v1/products/2",
			expected:    "/v1/products/*",
			expectError: false,
			description: "should cluster v1 products path",
		},
		{
			name:        "v1 products with longer number",
			input:       "/v1/products/22",
			expected:    "/v1/products/*",
			expectError: false,
			description: "should cluster v1 products with longer number",
		},
		{
			name:        "v1 products with alphanumeric",
			input:       "/v1/products/22j",
			expected:    "/v1/products/*",
			expectError: false,
			description: "should cluster v1 products with alphanumeric",
		},
		{
			name:        "products with org",
			input:       "/products/1/org/3",
			expected:    "/products/*/org/*",
			expectError: false,
			description: "should cluster products with org",
		},
		{
			name:        "products with double slash",
			input:       "/products//org/3",
			expected:    "/products//org/*",
			expectError: false,
			description: "should handle double slash",
		},
		{
			name:        "k6 test runs",
			input:       "/v1/k6-test-runs/1",
			expected:    "/v1/k6-test-runs/*",
			expectError: false,
			description: "should cluster k6 test runs",
		},
		{
			name:        "attach endpoint",
			input:       "/attach",
			expected:    "/attach",
			expectError: false,
			description: "should preserve static attach endpoint",
		},
		{
			name:        "Spanish users",
			input:       "/usuarios/fdklsd/j4elk/23993/trabajo/2",
			expected:    "/usuarios/*/j4elk/*/trabajo/*",
			expectError: false,
			description: "should cluster Spanish user paths",
		},
		{
			name:        "German users",
			input:       "/Benutzer/fdklsd/j4elk/23993/Arbeit/2",
			expected:    "/Benutzer/*/j4elk/*/Arbeit/*",
			expectError: false,
			description: "should cluster German user paths",
		},
		{
			name:        "French users",
			input:       "/utilisateurs/fdklsd/j4elk/23993/tache/2",
			expected:    "/utilisateurs/*/j4elk/*/tache/*",
			expectError: false,
			description: "should cluster French user paths",
		},
		{
			name:        "products trailing slash",
			input:       "/products/",
			expected:    "/products/",
			expectError: false,
			description: "should preserve products with trailing slash",
		},
		{
			name:        "user space with dash",
			input:       "/user-space/",
			expected:    "/user-space/",
			expectError: false,
			description: "should preserve user-space with dash",
		},
		{
			name:        "user space with underscore",
			input:       "/user_space/",
			expected:    "/user_space/",
			expectError: false,
			description: "should preserve user_space with underscore",
		},
		{
			name:        "api with dots",
			input:       "/api/hello.world",
			expected:    "/api/hello.world",
			expectError: false,
			description: "should preserve api with dots",
		},
		{
			name:        "api with multiple dots",
			input:       "/api/hello.world.again",
			expected:    "/api/hello.world.again",
			expectError: false,
			description: "should preserve api with multiple dots",
		},
		{
			name:        "api backup with dots",
			input:       "/api.backup/hello.world",
			expected:    "/api.backup/hello.world",
			expectError: false,
			description: "should preserve api.backup with dots",
		},
		{
			name:        "GET request",
			input:       "GET /user_space/",
			expected:    "GET /user_space/",
			expectError: false,
			description: "should preserve GET request",
		},
		{
			name:        "POST request",
			input:       "POST /user_space/",
			expected:    "POST /user_space/",
			expectError: false,
			description: "should preserve POST request",
		},
		{
			name:        "PUT request",
			input:       "PUT /user_space/",
			expected:    "PUT /user_space/",
			expectError: false,
			description: "should preserve PUT request",
		},
		{
			name:        "DELETE request",
			input:       "DELETE /user_space/",
			expected:    "DELETE /user_space/",
			expectError: false,
			description: "should preserve DELETE request",
		},
		{
			name:        "OPTIONS request",
			input:       "OPTIONS /user_space/",
			expected:    "OPTIONS /user_space/",
			expectError: false,
			description: "should preserve OPTIONS request",
		},
		{
			name:        "HEAD request",
			input:       "HEAD /user_space/",
			expected:    "HEAD /user_space/",
			expectError: false,
			description: "should preserve HEAD request",
		},
		{
			name:        "PATCH request",
			input:       "PATCH /user_space/",
			expected:    "PATCH /user_space/",
			expectError: false,
			description: "should preserve PATCH request",
		},
		{
			name:        "TRACE request",
			input:       "TRACE /user_space/",
			expected:    "TRACE /user_space/",
			expectError: false,
			description: "should preserve TRACE request",
		},
		{
			name:        "CONNECT request",
			input:       "CONNECT /user_space/",
			expected:    "CONNECT /user_space/",
			expectError: false,
			description: "should preserve CONNECT request",
		},
		{
			name:        "attach with query params",
			input:       "/attach?session_id=ddfsdsf&track_id=sjdklnfldsn",
			expected:    "/attach",
			expectError: false,
			description: "should strip query parameters",
		},
		{
			name:        "attach with fragment",
			input:       "/attach#section-1",
			expected:    "/attach",
			expectError: false,
			description: "should strip fragment",
		},
		{
			name:        "HTTP GET method",
			input:       "HTTP GET",
			expected:    "HTTP GET",
			expectError: false,
			description: "should preserve HTTP GET method",
		},
		{
			name:        "GET with complex query",
			input:       "GET /api/cart?sessionId=55f4e5ea-5d6d-482a-80c4-799e3c72dfb0&currencyCode=USD",
			expected:    "GET /api/cart",
			expectError: false,
			description: "should strip complex query parameters",
		},
		{
			name:        "getquote endpoint",
			input:       "/getquote",
			expected:    "/getquote",
			expectError: false,
			description: "should preserve getquote endpoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := csf.ClusterURLSafe(tc.input)

			if tc.expectError {
				assert.Error(t, err, "Expected error for: %s", tc.input)
			} else {
				assert.NoError(t, err, "Unexpected error for: %s", tc.input)
				assert.Equal(t, tc.expected, result, "Mismatch for: %s", tc.input)
			}
		})
	}
}

func TestClusterURLSafePanic(t *testing.T) {
	classifier, err := NewClusterURLClassifier(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create classifier: %v", err)
	}

	// This path triggered the "index out of range [1] with length 1" panic
	problematicPath := strings.Repeat("/segment-with-special-chars!@#$%^&*()", 22) + "?param=value"

	s, err := classifier.ClusterURLSafe(problematicPath)
	assert.Equal(t, problematicPath, s)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path validation failed: too many segments: 22 (max: 10)")
}
