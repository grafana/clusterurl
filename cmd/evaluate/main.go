package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/grafana/clusterurl/pkg/trie"
)

func main() {
	urlsFile := flag.String("urls", "assets/urls-list.txt", "Path to URLs file")
	softCard := flag.Int("soft", 10, "Soft max cardinality (preserve first N, then wildcard)")
	hardCard := flag.Int("hard", 100, "Hard max cardinality (collapse all after N)")
	maxPatterns := flag.Int("max-patterns", 1000, "Global max patterns (0 = no limit)")
	depth0Soft := flag.Int("depth0-soft", 0, "Soft cardinality override for depth 0 (0 = use default, -1 = no limit)")
	depth1Soft := flag.Int("depth1-soft", 0, "Soft cardinality override for depth 1 (0 = use default, -1 = no limit)")
	depth0Hard := flag.Int("depth0-hard", 0, "Hard cardinality override for depth 0 (0 = use default, -1 = no limit)")
	depth1Hard := flag.Int("depth1-hard", 0, "Hard cardinality override for depth 1 (0 = use default, -1 = no limit)")
	patternTTL := flag.Duration("pattern-ttl", 0, "Pattern TTL (e.g., 1h, 30m). 0 = no expiration")
	pruneInterval := flag.Duration("prune-interval", time.Minute, "How often to check for stale patterns")
	topN := flag.Int("top", 20, "Number of top clusters to show")
	flag.Parse()

	urls, err := loadURLs(*urlsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading URLs: %v\n", err)
		os.Exit(1)
	}

	depthSoftCards := make(map[int]int)
	if *depth0Soft != 0 {
		depthSoftCards[0] = *depth0Soft
	}
	if *depth1Soft != 0 {
		depthSoftCards[1] = *depth1Soft
	}

	depthHardCards := make(map[int]int)
	if *depth0Hard != 0 {
		depthHardCards[0] = *depth0Hard
	}
	if *depth1Hard != 0 {
		depthHardCards[1] = *depth1Hard
	}

	cfg := &trie.TrieConfig{
		SoftMaxCardinality:     *softCard,
		HardMaxCardinality:     *hardCard,
		MaxPatterns:            *maxPatterns,
		DepthSoftCardinalities: depthSoftCards,
		DepthHardCardinalities: depthHardCards,
		ReplaceWith:            "*",
		Separator:              "/",
		MaxDepth:               20,
		PatternTTL:             *patternTTL,
		PruneInterval:          *pruneInterval,
	}

	t, err := trie.NewPathTrie(cfg, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating trie: %v\n", err)
		os.Exit(1)
	}
	defer t.Stop() // stop background pruner if running

	// Prime the trie
	for _, url := range urls {
		t.Insert(url)
	}

	clustered := make(map[string]int)
	for _, url := range urls {
		result := t.Lookup(url)
		clustered[result]++
	}

	fmt.Println("URLs after collapsing:", len(clustered))

	printTopClusters(clustered, *topN)
}

func loadURLs(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			urls = append(urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

func printTopClusters(clustered map[string]int, n int) {
	type kv struct {
		Key   string
		Value int
	}

	var sorted []kv
	for k, v := range clustered {
		sorted = append(sorted, kv{k, v})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	for i := 0; i < n && i < len(sorted); i++ {
		fmt.Printf("%5d  %s\n", sorted[i].Value, sorted[i].Key)
	}
}
