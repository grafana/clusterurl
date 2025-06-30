package clusterurl

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/AlessandroPomponio/go-gibberish/gibberish"
	"github.com/AlessandroPomponio/go-gibberish/structs"
	lru "github.com/hashicorp/golang-lru/v2"
)

type ClusterUrlClassifier struct {
	classifier  *structs.GibberishData
	isSeparator map[byte]bool
	cache       *lru.Cache[string, bool]
	cfg         *Config
}

func NewClusterUrlClassifier(config *Config) (*ClusterUrlClassifier, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("NewClusterUrlClassifier: invalid configuration: %w", err)
	}

	classifier, err := loadKnowledgeBase()
	if err != nil {
		return nil, fmt.Errorf("NewClusterUrlClassifier: unable to load knowledge base: %w", err)
	}

	isSeparator := make(map[byte]bool)
	for _, sep := range config.Separators {
		isSeparator[sep] = true
	}

	cache, err := lru.New[string, bool](config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("NewClusterUrlClassifier: unable to create cache: %w", err)
	}

	return &ClusterUrlClassifier{
		classifier:  classifier,
		isSeparator: isSeparator,
		cache:       cache,
		cfg:         config,
	}, nil
}

// This function takes a path and returns a "clustered" path, where
// all the "IDs" in the path are replaced by a single "*" character.
// For example, the path "/foo/42/baz" would be replaced with "/foo/*/baz".
// The purpose of this function is to allow for a large number of paths
// to be grouped into a smaller number of paths.

//nolint:cyclop
func (csf *ClusterUrlClassifier) ClusterURL(path string) string {
	if path == "" {
		return path
	}

	p := []byte(path)
	sPos := 0
	sFwd := 0

	skip := false
	skipGrace := true
	nSegments := 0
	for _, c := range p {
		char := c
		if csf.isSeparator[c] {
			nSegments++
			if skip {
				p[sPos] = csf.cfg.ReplaceWith
				sPos++
			} else if sFwd > sPos {
				if !csf.okWord(string(p[sPos:sFwd])) {
					p[sPos] = csf.cfg.ReplaceWith
					sPos++
				} else {
					sPos = sFwd
				}
			}

			if nSegments >= csf.cfg.MaxSegments {
				break
			}

			p[sPos] = char
			sPos++
			sFwd = sPos
			skip = false
			skipGrace = true
		} else if !skip {
			p[sFwd] = c
			sFwd++
			if !isAlpha(c) {
				if skipGrace && (sFwd-sPos) == 2 {
					skipGrace = false
					continue
				}
				skip = true
			}
		}
	}

	if skip {
		p[sPos] = csf.cfg.ReplaceWith
		sPos++
	} else if sFwd > sPos {
		if !csf.okWord(string(p[sPos:sFwd])) {
			p[sPos] = csf.cfg.ReplaceWith
			sPos++
		} else {
			sPos = sFwd
		}
	}

	return string(p[:sPos])
}

func (csf *ClusterUrlClassifier) okWord(w string) bool {
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

func loadKnowledgeBase() (*structs.GibberishData, error) {
	content, err := dataFile.ReadFile("model.json")
	if err != nil {
		return nil, fmt.Errorf("LoadKnowledgeBase: unable to read knowledge base content: %w", err)
	}

	var data structs.GibberishData
	err = json.Unmarshal(content, &data)
	if err != nil {
		return nil, fmt.Errorf("LoadKnowledgeBase: unable to unmarshal knowledge base content: %w", err)
	}

	return &data, nil
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_' || c == ' ' || c == '.'
}
