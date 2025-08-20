package clusterurl

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/AlessandroPomponio/go-gibberish/gibberish"
	"github.com/AlessandroPomponio/go-gibberish/structs"
	lru "github.com/hashicorp/golang-lru/v2"
)

type ClusterURLClassifier struct {
	classifier     *structs.GibberishData
	cache          *lru.Cache[string, bool]
	cfg            *Config
	validCharTable [256]bool
}

func NewClusterURLClassifier(config *Config) (*ClusterURLClassifier, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("NewClusterURLClassifier: invalid configuration: %w", err)
	}

	classifier, err := loadKnowledgeBase(config.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("NewClusterURLClassifier: unable to load knowledge base: %w", err)
	}

	cache, err := lru.New[string, bool](config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("NewClusterURLClassifier: unable to create cache: %w", err)
	}

	// Initialize lookup table for valid characters
	var validCharTable [256]bool
	for c := byte('a'); c <= 'z'; c++ {
		validCharTable[c] = true
	}
	for c := byte('A'); c <= 'Z'; c++ {
		validCharTable[c] = true
	}
	for _, c := range config.AdditionalValidChars {
		validCharTable[c] = true
	}

	return &ClusterURLClassifier{
		classifier:     classifier,
		cache:          cache,
		cfg:            config,
		validCharTable: validCharTable,
	}, nil
}

// This function takes a path and returns a "clustered" path, where
// all the "IDs" in the path are replaced by a single "*" character.
// For example, the path "/foo/42/baz" would be replaced with "/foo/*/baz".
// The purpose of this function is to allow for a large number of paths
// to be grouped into a smaller number of paths.

//nolint:cyclop
func (csf *ClusterURLClassifier) ClusterURL(path string) string {
	if path == "" {
		return path
	}

	// we will be resizing copies of this slice
	p := make([]byte, len(path))
	copy(p, []byte(path))
	sPos := 0
	sFwd := 0

	// ensureCapacity helps resize the slice if needed
	ensureCapacity := func(requiredIndex int) {
		if requiredIndex >= len(p) {
			newSize := len(p) * 2
			if newSize <= requiredIndex {
				newSize = requiredIndex + 1
			}
			newP := make([]byte, newSize)
			copy(newP, p)
			p = newP
		}
	}

	skip := false
	skipGrace := true
	nSegments := 0
	inQuery := false
	for _, c := range []byte(path) {
		char := c
		if c == '?' || c == '#' || (c == '&' && inQuery) {
			if c == '?' {
				inQuery = true
			}
			break
		}

		if c == csf.cfg.Separator {
			nSegments++
			if skip {
				ensureCapacity(sPos)
				p[sPos] = csf.cfg.ReplaceWith
				sPos++
			} else if sFwd > sPos {
				if !csf.okWord(string(p[sPos:sFwd])) {
					ensureCapacity(sPos)
					p[sPos] = csf.cfg.ReplaceWith
					sPos++
				} else {
					sPos = sFwd
				}
			}

			if nSegments >= csf.cfg.MaxSegments {
				break
			}

			ensureCapacity(sPos)
			p[sPos] = char
			sPos++
			sFwd = sPos
			skip = false
			skipGrace = true
		} else if !skip {
			ensureCapacity(sFwd)
			p[sFwd] = c
			sFwd++
			if !csf.validCharTable[c] {
				if skipGrace && (sFwd-sPos) == 2 {
					skipGrace = false
					continue
				}
				skip = true
			}
		}
	}

	if skip {
		ensureCapacity(sPos)
		p[sPos] = csf.cfg.ReplaceWith
		sPos++
	} else if sFwd > sPos {
		if !csf.okWord(string(p[sPos:sFwd])) {
			ensureCapacity(sPos)
			p[sPos] = csf.cfg.ReplaceWith
			sPos++
		} else {
			sPos = sFwd
		}
	}

	return string(p[:sPos])
}

func (csf *ClusterURLClassifier) okWord(w string) bool {
	_, ok := csf.cache.Get(w)
	if ok {
		return ok
	}
	if gibberish.IsGibberish(w, csf.classifier) {
		return false
	}

	csf.cache.Add(w, true)
	return true
}

//go:embed model.json
var dataFile embed.FS

func loadKnowledgeBase(path string) (*structs.GibberishData, error) {
	var content []byte
	var err error
	if path != "" {
		content, err = os.ReadFile(path)
	} else {
		content, err = dataFile.ReadFile("model.json")
	}

	if err != nil {
		return nil, fmt.Errorf("loadKnowledgeBase: unable to read knowledge base content: %w", err)
	}

	var data structs.GibberishData
	err = json.Unmarshal(content, &data)
	if err != nil {
		return nil, fmt.Errorf("loadKnowledgeBase: unable to unmarshal knowledge base content: %w", err)
	}

	return &data, nil
}
