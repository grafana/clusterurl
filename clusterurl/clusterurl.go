package clusterurl

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	// Use the safe version and ignore errors for backward compatibility
	result, _ := csf.ClusterURLSafe(path)
	return result
}

// ClusterURLSafe is a safer version of ClusterURL that returns an error instead of panicking.
// It includes input sanitization and proper bounds checking.
func (csf *ClusterURLClassifier) ClusterURLSafe(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	// Sanitize the input path
	sanitizedPath, err := csf.sanitizePath(path)
	if err != nil {
		return path, fmt.Errorf("failed to sanitize path: %w", err)
	}

	// Validate path length and segment count
	if err := csf.validatePath(sanitizedPath); err != nil {
		return path, fmt.Errorf("path validation failed: %w", err)
	}

	// Process the sanitized path
	result, err := csf.processPath(sanitizedPath)
	if err != nil {
		return path, fmt.Errorf("path processing failed: %w", err)
	}

	return result, nil
}

// IsSanitizationEnabled returns whether path sanitization is enabled
func (csf *ClusterURLClassifier) IsSanitizationEnabled() bool {
	return csf.cfg.EnableSanitization
}

// sanitizePath cleans and normalizes the input path
func (csf *ClusterURLClassifier) sanitizePath(path string) (string, error) {
	if !csf.cfg.EnableSanitization {
		return path, nil
	}

	if len(path) > csf.cfg.MaxPathLength {
		return "", fmt.Errorf("path too long: %d characters (max: %d)", len(path), csf.cfg.MaxPathLength)
	}

	// Remove null bytes and other problematic characters
	cleanPath := make([]byte, 0, len(path))
	for _, c := range []byte(path) {
		if c == 0 { // Skip null bytes
			continue
		}
		cleanPath = append(cleanPath, c)
	}

	return string(cleanPath), nil
}

// validatePath checks if the path is safe to process
func (csf *ClusterURLClassifier) validatePath(path string) error {
	if !csf.cfg.EnableSanitization {
		return nil
	}

	if len(path) == 0 {
		return nil
	}

	// Check segment count
	segmentCount := strings.Count(path, string(csf.cfg.Separator))
	if segmentCount > csf.cfg.MaxSegments {
		return fmt.Errorf("too many segments: %d (max: %d)", segmentCount, csf.cfg.MaxSegments)
	}

	// Check for reasonable path length for processing
	maxProcessingLength := csf.cfg.MaxPathLength / 2
	if len(path) > maxProcessingLength {
		return fmt.Errorf("path too long for processing: %d characters (max: %d)", len(path), maxProcessingLength)
	}

	return nil
}

// processPath safely processes the path without risk of panic
func (csf *ClusterURLClassifier) processPath(path string) (string, error) {
	p := []byte(path)
	sPos := 0
	sFwd := 0

	skip := false
	skipGrace := true
	nSegments := 0

	// Ensure we don't exceed array bounds
	maxLen := len(p)
	if maxLen == 0 {
		return "", nil
	}

	for i := 0; i < maxLen; i++ {
		c := p[i]
		char := c

		// Strip query string and fragment identifiers
		if c == '?' || c == '&' || c == '#' {
			if sPos < maxLen {
				p = p[:sPos]
			}
			break
		}

		if c == csf.cfg.Separator {
			nSegments++

			// Bounds checking for skip logic
			if skip {
				if sPos < maxLen {
					p[sPos] = csf.cfg.ReplaceWith
					sPos++
				}
			} else if sFwd > sPos && sFwd <= maxLen {
				// Safe substring extraction
				if sPos < sFwd && sFwd <= maxLen {
					segment := string(p[sPos:sFwd])
					if !csf.okWord(segment) {
						if sPos < maxLen {
							p[sPos] = csf.cfg.ReplaceWith
							sPos++
						}
					} else {
						sPos = sFwd
					}
				}
			}

			// Check segment limit
			if nSegments >= csf.cfg.MaxSegments {
				break
			}

			// Safe character assignment
			if sPos < maxLen {
				p[sPos] = char
				sPos++
				sFwd = sPos
			}
			skip = false
			skipGrace = true
		} else if !skip {
			// Safe character assignment
			if sFwd < maxLen {
				p[sFwd] = c
				sFwd++
			}

			// Bounds checking for validCharTable access
			if !csf.validCharTable[c] {
				if skipGrace && (sFwd-sPos) == 2 {
					skipGrace = false
					continue
				}
				skip = true
			}
		}
	}

	// Final processing with bounds checking
	if skip {
		if sPos < maxLen {
			p[sPos] = csf.cfg.ReplaceWith
			sPos++
		}
	} else if sFwd > sPos && sFwd <= maxLen {
		if sPos < sFwd && sFwd <= maxLen {
			segment := string(p[sPos:sFwd])
			if !csf.okWord(segment) {
				if sPos < maxLen {
					p[sPos] = csf.cfg.ReplaceWith
					sPos++
				}
			} else {
				sPos = sFwd
			}
		}
	}

	// Safe slice operation
	if sPos > maxLen {
		sPos = maxLen
	}
	if sPos < 0 {
		sPos = 0
	}

	return string(p[:sPos]), nil
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
