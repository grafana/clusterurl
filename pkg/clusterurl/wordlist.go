package clusterurl

// WordList provides static allowlist/denylist for path segments.
// These are shipped with clusterurl and do not change at runtime.
type WordList struct {
	// Segments that are always preserved regardless of position
	GlobalAllow map[string]struct{}

	// Segments that are always collapsed regardless of position
	GlobalDeny map[string]struct{}

	// Depth-specific allowlists (0-indexed depth -> set of words)
	// Depth 0 is first segment after leading /
	DepthAllow map[int]map[string]struct{}

	// Depth-specific denylists
	DepthDeny map[int]map[string]struct{}
}

// DefaultWordList returns the built-in static wordlist.
func DefaultWordList() *WordList {
	return &WordList{
		GlobalAllow: toSet([]string{
			// Common API prefixes
			"api", "v1", "v2", "v3", "rest", "graphql", "grpc",
			// Common resource types
			"users", "user", "accounts", "account", "orgs", "org",
			"projects", "project", "teams", "team", "groups", "group",
			"services", "service", "apps", "app", "instances", "instance",
			"pods", "pod", "nodes", "node", "namespaces", "namespace",
			"clusters", "cluster", "deployments", "deployment",
			"jobs", "job", "tasks", "task", "workflows", "workflow",
			"alerts", "alert", "rules", "rule", "dashboards", "dashboard",
			"metrics", "logs", "traces", "events", "audit",
			// Actions
			"search", "query", "list", "get", "create", "update", "delete",
			"status", "health", "healthz", "ready", "readyz", "live", "livez",
			"config", "settings", "preferences",
		}),
		GlobalDeny:  toSet([]string{}),
		DepthAllow:  map[int]map[string]struct{}{},
		DepthDeny:   map[int]map[string]struct{}{},
	}
}

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}

