package clusterurl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterURL(t *testing.T) {
	testcases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/users/fdklsd/j4elk/23993/job/2", "/users/*/j4elk/*/job/*"},
		{"123", "*"},
		{"/123", "/*"},
		{"123/", "*/"},
		{"123/ljgdflgjf", "*/*"},
		{"/**", "/*"},
		{"/u/2", "/u/*"},
		{"/v1/products/2", "/v1/products/*"},
		{"/v1/products/22", "/v1/products/*"},
		{"/v1/products/22j", "/v1/products/*"},
		{"/products/1/org/3", "/products/*/org/*"},
		{"/products//org/3", "/products//org/*"},
		{"/v1/k6-test-runs/1", "/v1/k6-test-runs/*"},
		{"/attach", "/attach"},
		{"/usuarios/fdklsd/j4elk/23993/trabajo/2", "/usuarios/*/j4elk/*/trabajo/*"},
		{"/Benutzer/fdklsd/j4elk/23993/Arbeit/2", "/Benutzer/*/j4elk/*/Arbeit/*"},
		{"/utilisateurs/fdklsd/j4elk/23993/tache/2", "/utilisateurs/*/j4elk/*/tache/*"},
		{"/products/", "/products/"},
		{"/user-space/", "/user-space/"},
		{"/user_space/", "/user_space/"},
		{"/my-page#/route/products/1234/comments", "/my-page#/route/products/<id>/comments"},
	}

	err := InitAutoClassifier()
	assert.NoError(t, err)

	for _, tc := range testcases {
		assert.Equal(t, tc.expected, ClusterURL(tc.input))
	}
}
