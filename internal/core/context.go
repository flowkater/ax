package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ContextSnippet is a path + excerpt pair used by 3-layer context loading.
type ContextSnippet struct {
	Path    string
	Excerpt string
}

// ContextLayers represents protocol/task/session context stacks.
type ContextLayers struct {
	Protocol      ContextSnippet
	TaskScoped    []ContextSnippet
	SessionMemory []ContextSnippet
}

// LoadContextLayers collects 3-layer context using deterministic ordering.
func LoadContextLayers(base string, state *State, affectedPrefixes []string) ContextLayers {
	layers := ContextLayers{}

	layers.Protocol = readSnippet(base, filepath.Join(base, ".ax", "context-policy.md"), 18)

	// Layer 2: task-scoped artifacts from context_chain.
	if state != nil {
		for _, rel := range state.ContextChain {
			if len(layers.TaskScoped) >= 5 {
				break
			}
			if !matchesAffectedPath(rel, affectedPrefixes) {
				continue
			}
			full := filepath.Join(base, filepath.FromSlash(rel))
			snippet := readSnippet(base, full, 12)
			if snippet.Path == "" {
				continue
			}
			layers.TaskScoped = append(layers.TaskScoped, snippet)
		}
	}

	// Layer 3: session memory.
	memoryFiles := []string{
		filepath.Join(base, ".ax", "memory", "MEMORY.md"),
		filepath.Join(base, ".ax", "memory", "gotchas.md"),
	}
	for _, m := range memoryFiles {
		snippet := readSnippet(base, m, 8)
		if snippet.Path != "" {
			layers.SessionMemory = append(layers.SessionMemory, snippet)
		}
	}

	return layers
}

func matchesAffectedPath(rel string, affectedPrefixes []string) bool {
	if len(affectedPrefixes) == 0 {
		return true
	}
	rel = filepath.ToSlash(rel)
	for _, prefix := range affectedPrefixes {
		prefix = strings.TrimSpace(filepath.ToSlash(prefix))
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func readSnippet(base, fullPath string, maxLines int) ContextSnippet {
	body, err := os.ReadFile(fullPath)
	if err != nil {
		return ContextSnippet{}
	}
	rel, err := filepath.Rel(base, fullPath)
	if err != nil {
		rel = fullPath
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if maxLines <= 0 || maxLines > len(lines) {
		maxLines = len(lines)
	}
	trimmed := strings.Join(lines[:maxLines], "\n")
	trimmed = strings.TrimSpace(trimmed)
	if len(trimmed) > 400 {
		trimmed = trimmed[:397] + "..."
	}
	return ContextSnippet{
		Path:    filepath.ToSlash(rel),
		Excerpt: trimmed,
	}
}

// FormatContextLayers renders context summary for markdown artifacts.
func FormatContextLayers(layers ContextLayers) string {
	var b strings.Builder
	b.WriteString("## Context Inputs\n\n")

	if layers.Protocol.Path != "" {
		b.WriteString(fmt.Sprintf("- Protocol: `%s`\n", layers.Protocol.Path))
	}
	if len(layers.TaskScoped) == 0 {
		b.WriteString("- Task Scoped: none\n")
	} else {
		b.WriteString("- Task Scoped:\n")
		for _, s := range layers.TaskScoped {
			b.WriteString(fmt.Sprintf("  - `%s`\n", s.Path))
		}
	}
	if len(layers.SessionMemory) == 0 {
		b.WriteString("- Session Memory: none\n")
	} else {
		b.WriteString("- Session Memory:\n")
		for _, s := range layers.SessionMemory {
			b.WriteString(fmt.Sprintf("  - `%s`\n", s.Path))
		}
	}
	b.WriteString("\n")
	return b.String()
}

var pathCandidatePattern = regexp.MustCompile(`([A-Za-z0-9._-]+/[A-Za-z0-9._/-]+)`)

// AnalyzeAffectedDirectories extracts deterministic directory candidates from task text + references.
func AnalyzeAffectedDirectories(taskDescription string, refs ...string) []string {
	seen := map[string]struct{}{}
	add := func(raw string) {
		raw = strings.Trim(raw, " \t\n\r`'\".,:;()[]{}")
		raw = filepath.ToSlash(raw)
		if raw == "" || !strings.Contains(raw, "/") {
			return
		}
		dir := filepath.ToSlash(filepath.Dir(raw))
		if dir == "." || dir == "" {
			return
		}
		seen[dir] = struct{}{}
	}

	for _, m := range pathCandidatePattern.FindAllString(taskDescription, -1) {
		add(m)
	}
	for _, ref := range refs {
		body, err := os.ReadFile(ref)
		if err != nil {
			continue
		}
		for _, m := range pathCandidatePattern.FindAllString(string(body), -1) {
			add(m)
		}
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// BuiltContextLayers is a compact layer shape used by command scaffolding/tests.
type BuiltContextLayers struct {
	Protocol []string
	TaskScoped []string
	SessionMem []string
}

// BuildContextLayers returns deterministic protocol/task/session path stacks.
func BuildContextLayers(base string, contextChain []string, affectedPrefixes []string) BuiltContextLayers {
	layers := BuiltContextLayers{
		Protocol: []string{filepath.ToSlash(filepath.Join(".ax", "context-policy.md"))},
	}

	for _, rel := range SortUniquePaths(contextChain) {
		if !matchesAffectedPath(rel, affectedPrefixes) {
			continue
		}
		layers.TaskScoped = append(layers.TaskScoped, filepath.ToSlash(rel))
	}

	memoryFiles := []string{
		filepath.Join(base, ".ax", "memory", "MEMORY.md"),
		filepath.Join(base, ".ax", "memory", "gotchas.md"),
	}
	for _, full := range memoryFiles {
		if _, err := os.Stat(full); err != nil {
			continue
		}
		rel, err := filepath.Rel(base, full)
		if err != nil {
			rel = full
		}
		layers.SessionMem = append(layers.SessionMem, filepath.ToSlash(rel))
	}

	return layers
}
