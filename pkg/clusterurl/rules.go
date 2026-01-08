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

// ResourceValueRule collapses ANY segment that immediately follows an allowlisted resource type.
// Use this when allowlisted segments represent collection names whose values are high-cardinality.
// E.g., if "books" is allowlisted: /books/harry-potter -> /books/*
// Note: This only works for immediate followers. For broader collapsing, see NonAllowlistedRule.
type ResourceValueRule struct{}

func (r ResourceValueRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	if ctx.PrevSegment == "" {
		return false, false
	}

	lower := strings.ToLower(ctx.PrevSegment)
	if _, ok := ctx.WordList.GlobalAllow[lower]; ok {
		// Previous segment is a resource type, collapse this value
		return true, true
	}

	return false, false
}

// NonAllowlistedRule collapses any segment that is not in the allowlist.
// This treats the allowlist as the complete set of valid path keywords;
// everything else is considered high-cardinality and collapsed.
// E.g., with allowlist {api, v1, books, chapters}:
//
//	/api/v1/books/harry-potter/chapters/intro -> /api/v1/books/*/chapters/*
type NonAllowlistedRule struct{}

func (r NonAllowlistedRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	lower := strings.ToLower(segment)

	// Check if in global allowlist
	if _, ok := ctx.WordList.GlobalAllow[lower]; ok {
		return false, true // preserve
	}

	// Check depth-specific allowlist
	if depthSet, ok := ctx.WordList.DepthAllow[depth]; ok {
		if _, ok := depthSet[lower]; ok {
			return false, true // preserve
		}
	}

	// Not allowlisted -> collapse
	return true, true
}

// LastSegmentRule collapses the final segment of a path (unless it's allowlisted).
// Useful when the last segment is typically a resource ID or name.
// Requires TotalDepth to be set in RuleContext.
// E.g., /api/books/harry-potter -> /api/books/* (if depth matches TotalDepth-1)
type LastSegmentRule struct{}

func (r LastSegmentRule) ShouldCollapse(segment string, depth int, ctx *RuleContext) (collapse, handled bool) {
	// Only act on last segment
	if ctx.TotalDepth < 0 || depth != ctx.TotalDepth-1 {
		return false, false
	}

	lower := strings.ToLower(segment)

	// Don't collapse if allowlisted
	if _, ok := ctx.WordList.GlobalAllow[lower]; ok {
		return false, false
	}
	if depthSet, ok := ctx.WordList.DepthAllow[depth]; ok {
		if _, ok := depthSet[lower]; ok {
			return false, false
		}
	}

	return true, true
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
