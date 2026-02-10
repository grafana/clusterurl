package trie

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds Prometheus metrics for a PathTrie.
type Metrics struct {
	PatternCount         prometheus.Gauge
	TotalWrites          prometheus.Counter
	TotalWritesWithChanges prometheus.Counter
}
