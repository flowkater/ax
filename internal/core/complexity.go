package core

import (
	"path/filepath"
	"sort"
	"strings"
)

// Depth represents adaptive routing depth.
type Depth string

const (
	DepthQuick   Depth = "quick"
	DepthNormal  Depth = "normal"
	DepthDeep    Depth = "deep"
	DepthExplore Depth = "explore"
)

const (
	DepthSourceUser = "user"
	DepthSourceAuto = "auto"
)

var validDepth = map[Depth]bool{
	DepthQuick: true, DepthNormal: true, DepthDeep: true, DepthExplore: true,
}

// ResolveDepth resolves depth with user override precedence.
func ResolveDepth(taskDescription string, affectedPaths []string, override string) (depth Depth, source string) {
	o := Depth(strings.ToLower(strings.TrimSpace(override)))
	if o != "" && validDepth[o] {
		return o, DepthSourceUser
	}
	return classifyDepth(taskDescription, affectedPaths), DepthSourceAuto
}

// ClassifyDepth classifies task complexity using summary signals.
func ClassifyDepth(taskDescription string, estimatedFiles int, crossModule bool) Depth {
	paths := make([]string, 0, estimatedFiles)
	moduleA := "cmd/ax"
	moduleB := "internal/core"
	for i := 0; i < estimatedFiles; i++ {
		module := moduleA
		if crossModule && i%2 == 1 {
			module = moduleB
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(module, "file-"+strings.TrimSpace(strings.ToLower(string(rune('a'+(i%26)))))+".go")))
	}
	return classifyDepth(taskDescription, paths)
}

func classifyDepth(taskDescription string, affectedPaths []string) Depth {
	text := strings.ToLower(taskDescription)
	if strings.Contains(text, "explore") || strings.Contains(text, "investigate") || strings.Contains(text, "unclear") || strings.Contains(text, "unknown") || strings.Contains(text, "design") {
		return DepthExplore
	}

	if strings.Contains(text, "architecture") || strings.Contains(text, "refactor") {
		return DepthDeep
	}

	if ShouldEscalateQuick(affectedPaths) {
		return DepthDeep
	}

	count := len(affectedPaths)
	switch {
	case count >= 10:
		return DepthDeep
	case count >= 3:
		return DepthNormal
	default:
		if strings.Contains(text, "add") || strings.Contains(text, "feature") || strings.Contains(text, "implement") {
			return DepthNormal
		}
		return DepthQuick
	}
}

// ShouldEscalateQuick determines quick escalation policy.
func ShouldEscalateQuick(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	if len(paths) > 5 {
		return true
	}

	modules := make(map[string]struct{})
	for _, raw := range paths {
		n := strings.TrimSpace(filepath.ToSlash(raw))
		if n == "" {
			continue
		}
		parts := strings.Split(n, "/")
		if len(parts) > 0 {
			modules[parts[0]] = struct{}{}
		}
		if strings.Contains(n, "state") || strings.Contains(n, "codex") || strings.Contains(n, "run") {
			return true
		}
	}

	return len(modules) > 1
}

// ShouldEscalateQuickStats determines quick escalation policy from summarized metrics.
func ShouldEscalateQuickStats(filesChanged int, crossModule bool, coreTouched bool) bool {
	if filesChanged > 5 || crossModule || coreTouched {
		return true
	}
	return false
}

// SortUniquePaths returns deterministic path ordering.
func SortUniquePaths(paths []string) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		n := strings.TrimSpace(filepath.ToSlash(p))
		if n == "" {
			continue
		}
		if _, ok := set[n]; ok {
			continue
		}
		set[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
