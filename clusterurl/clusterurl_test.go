package clusterurl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterURL(t *testing.T) {
	err := InitAutoClassifier()
	assert.NoError(t, err)
	assert.Equal(t, "", ClusterURL(""))
	assert.Equal(t, "/users/*/j4elk/*/job/*", ClusterURL("/users/fdklsd/j4elk/23993/job/2"))
	assert.Equal(t, "*", ClusterURL("123"))
	assert.Equal(t, "/*", ClusterURL("/123"))
	assert.Equal(t, "*/", ClusterURL("123/"))
	assert.Equal(t, "*/*", ClusterURL("123/ljgdflgjf"))
	assert.Equal(t, "/*", ClusterURL("/**"))
	assert.Equal(t, "/u/*", ClusterURL("/u/2"))
	assert.Equal(t, "/v1/products/*", ClusterURL("/v1/products/2"))
	assert.Equal(t, "/v1/products/*", ClusterURL("/v1/products/22"))
	assert.Equal(t, "/v1/products/*", ClusterURL("/v1/products/22j"))
	assert.Equal(t, "/products/*/org/*", ClusterURL("/products/1/org/3"))
	assert.Equal(t, "/products//org/*", ClusterURL("/products//org/3"))
	assert.Equal(t, "/v1/k6-test-runs/*", ClusterURL("/v1/k6-test-runs/1"))
	assert.Equal(t, "/attach", ClusterURL("/attach"))
	assert.Equal(t, "/usuarios/*/j4elk/*/trabajo/*", ClusterURL("/usuarios/fdklsd/j4elk/23993/trabajo/2"))
	assert.Equal(t, "/Benutzer/*/j4elk/*/Arbeit/*", ClusterURL("/Benutzer/fdklsd/j4elk/23993/Arbeit/2"))
	assert.Equal(t, "/utilisateurs/*/j4elk/*/tache/*", ClusterURL("/utilisateurs/fdklsd/j4elk/23993/tache/2"))
	assert.Equal(t, "/products/", ClusterURL("/products/"))
	assert.Equal(t, "/user-space/", ClusterURL("/user-space/"))
	assert.Equal(t, "/user_space/", ClusterURL("/user_space/"))
	assert.Equal(t, "/api/hello.world", ClusterURL("/api/hello.world"))
	assert.Equal(t, "/api/hello.world.again", ClusterURL("/api/hello.world.again"))
	assert.Equal(t, "/api.backup/hello.world", ClusterURL("/api.backup/hello.world"))
	assert.Equal(t, "GET /user_space/", ClusterURL("GET /user_space/"))
	assert.Equal(t, "POST /user_space/", ClusterURL("POST /user_space/"))
	assert.Equal(t, "PUT /user_space/", ClusterURL("PUT /user_space/"))
	assert.Equal(t, "DELETE /user_space/", ClusterURL("DELETE /user_space/"))
	assert.Equal(t, "OPTIONS /user_space/", ClusterURL("OPTIONS /user_space/"))
	assert.Equal(t, "HEAD /user_space/", ClusterURL("HEAD /user_space/"))
	assert.Equal(t, "PATCH /user_space/", ClusterURL("PATCH /user_space/"))
	assert.Equal(t, "TRACE /user_space/", ClusterURL("TRACE /user_space/"))
	assert.Equal(t, "CONNECT /user_space/", ClusterURL("CONNECT /user_space/"))
	assert.Equal(t, "/attach&session_id=*&track_id=*", ClusterURL("/attach&session_id=ddfsdsf&track_id=sjdklnfldsn"))
}
