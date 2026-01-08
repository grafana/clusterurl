package clusterurl

import "strings"

// CollapseRule represents a deterministic collapsing heuristic.
// Rules are evaluated in order; first match wins.
type CollapseRule interface {
	// ShouldCollapse returns (collapse bool, handled bool)
	// If handled=false, continue to next rule
	// If handled=true and collapse=true, replace with *
	// If handled=true and collapse=false, preserve segment
	ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool)
}

// RuleContext provides context for rule evaluation
type RuleContext struct {
	WordList    *WordList
	PrevSegment string // previous segment (empty if first)
	TotalDepth  int    // total segments in path (if known, else -1)
}

// AllowlistRule checks global and depth-specific allowlists
type AllowlistRule struct{}

func (r AllowlistRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	lower := strings.ToLower(segment)

	// Check global allow
	if _, ok := ctx.WordList.GlobalAllow[lower]; ok {
		return false, true
	}

	// Check depth-specific allow
	if depthSet, ok := ctx.WordList.DepthAllow[depth]; ok {
		if _, ok := depthSet[lower]; ok {
			return false, true
		}
	}

	return false, false
}

// DenylistRule checks global and depth-specific denylists
type DenylistRule struct{}

func (r DenylistRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	lower := strings.ToLower(segment)

	// Check global deny
	if _, ok := ctx.WordList.GlobalDeny[lower]; ok {
		return true, true
	}

	// Check depth-specific deny
	if depthSet, ok := ctx.WordList.DepthDeny[depth]; ok {
		if _, ok := depthSet[lower]; ok {
			return true, true
		}
	}

	return false, false
}

// NumericPrefixRule collapses segments that start with digits
// after a known resource type (e.g., /users/123, /pods/pod-12345)
type NumericPrefixRule struct{}

func (r NumericPrefixRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	if len(segment) == 0 {
		return false, false
	}

	// If previous segment is in allowlist (likely a resource type)
	// and this segment starts with digit -> collapse
	if ctx.PrevSegment != "" {
		lower := strings.ToLower(ctx.PrevSegment)
		if _, ok := ctx.WordList.GlobalAllow[lower]; ok {
			if segment[0] >= '0' && segment[0] <= '9' {
				return true, true
			}
		}
	}

	return false, false
}

// LengthRule collapses segments that are too long to be meaningful keywords
type LengthRule struct {
	MaxLength int // default 32
}

func (r LengthRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	maxLen := r.MaxLength
	if maxLen == 0 {
		maxLen = 32
	}
	if len(segment) > maxLen {
		return true, true
	}
	return false, false
}

// PatternRule collapses segments matching known ID patterns
type PatternRule struct{}

func (r PatternRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	// Heuristics for common ID patterns (pure function, no regex for perf)

	// Kubernetes-style: contains multiple hyphens with trailing hash
	// e.g., "my-deployment-5d4f7c8b9f"
	if looksLikeK8sName(segment) {
		return true, true
	}

	// Hex strings of certain lengths (short hashes, commit SHAs)
	if isHexString(segment) && (len(segment) == 7 || len(segment) == 8 || len(segment) == 12 || len(segment) == 40) {
		return true, true
	}

	return false, false
}

func looksLikeK8sName(s string) bool {
	// Pattern: word-word-...-randomhash
	// Has at least 2 hyphens and ends with lowercase alphanumeric
	hyphenCount := 0
	for _, c := range s {
		if c == '-' {
			hyphenCount++
		}
	}
	if hyphenCount < 2 {
		return false
	}
	// Check if last segment after final hyphen looks like random suffix
	lastHyphen := strings.LastIndex(s, "-")
	if lastHyphen == -1 || lastHyphen >= len(s)-1 {
		return false
	}
	suffix := s[lastHyphen+1:]
	if len(suffix) >= 5 && len(suffix) <= 10 && isAlphanumLower(suffix) {
		return true
	}
	return false
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

func isAlphanumLower(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}

// DefaultRules returns the default rule chain for collapsing.
// Order matters: deny > allow > length > pattern > numeric prefix.
// Gibberish detection happens after all rules as fallback.
func DefaultRules() []CollapseRule {
	return []CollapseRule{
		DenylistRule{},
		AllowlistRule{},
		LengthRule{MaxLength: 32},
		PatternRule{},
		NumericPrefixRule{},
	}
}

