package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/grafana/clusterurl/pkg/trie"
)

func main() {
	urlsFile := flag.String("urls", "assets/urls-list.txt", "Path to URLs file")
	maxCard := flag.Int("max-card", 50, "Default max cardinality")
	depth0Card := flag.Int("depth0-card", 0, "Cardinality override for depth 0 (0 = use default)")
	depth1Card := flag.Int("depth1-card", 0, "Cardinality override for depth 1 (0 = use default)")
	depth2Card := flag.Int("depth2-card", 0, "Cardinality override for depth 2 (0 = use default)")
	topN := flag.Int("top", 20, "Number of top clusters to show")
	flag.Parse()

	urls, err := loadURLs(*urlsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading URLs: %v\n", err)
		os.Exit(1)
	}

	depthCards := make(map[int]int)
	if *depth0Card != 0 {
		depthCards[0] = *depth0Card
	}
	if *depth1Card != 0 {
		depthCards[1] = *depth1Card
	}
	if *depth2Card != 0 {
		depthCards[2] = *depth2Card
	}

	cfg := &trie.TrieConfig{
		DefaultMaxCardinality: *maxCard,
		DepthCardinalities:    depthCards,
		ReplaceWith:           "*",
		Separator:             "/",
		MaxDepth:              20,
	}

	t, err := trie.NewPathTrie(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating trie: %v\n", err)
		os.Exit(1)
	}

	// prime the trie.
	for _, url := range urls {
		t.Insert(url)
	}

	clustered := make(map[string]int)
	for _, url := range urls {
		result := t.Insert(url)
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
