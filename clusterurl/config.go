package clusterurl

import "fmt"

type Config struct {
	// MaxSegments is the maximum number of segments in a path.
	MaxSegments int `json:"max_segments"`
	// Separators is a list of characters that are considered as separators in a path.
	Separators []byte `json:"separators"`
	// ReplaceWith is the character that will replace the segments in a path.
	ReplaceWith byte `json:"replace_with"`
	// CacheSize is the size of the cache for the classifier.
	CacheSize int `json:"cache_size"`
}

func DefaultConfig() *Config {
	return &Config{
		MaxSegments: 10,
		Separators:  []byte{'/', '&', '?', '='},
		ReplaceWith: '*',
		CacheSize:   8192,
	}
}
func (c *Config) Validate() error {
	if c.MaxSegments <= 0 {
		return fmt.Errorf("field MaxSegments must be greater than 0")
	}
	if len(c.Separators) == 0 {
		return fmt.Errorf("field Separators cannot be empty")
	}
	if c.ReplaceWith == 0 {
		return fmt.Errorf("field ReplaceWith cannot be zero")
	}
	if c.CacheSize <= 0 {
		return fmt.Errorf("field CacheSize must be greater than 0")
	}
	if len(c.Separators) > 255 {
		return fmt.Errorf("field Separators cannot have more than 255 characters")
	}
	if c.MaxSegments > 100 {
		return fmt.Errorf("field MaxSegments cannot be greater than 100")
	}
	return nil
}
