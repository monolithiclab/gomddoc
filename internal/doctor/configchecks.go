package doctor

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
)

// keyTree is config.yml's key structure, derived from config.Schema: the
// children allowed under each parent path ("" is the root), and the map-typed
// settings whose children are the author's (theme.vars, theme.features).
type keyTree struct {
	children map[string][]string
	open     map[string]bool
}

func newKeyTree() keyTree {
	t := keyTree{children: map[string][]string{}, open: map[string]bool{}}
	for _, s := range config.Schema() {
		if s.FileKey == "" {
			continue
		}
		parts := strings.Split(s.FileKey, ".")
		for i := range parts {
			parent := strings.Join(parts[:i], ".")
			if !slices.Contains(t.children[parent], parts[i]) {
				t.children[parent] = append(t.children[parent], parts[i])
			}
		}
		if strings.HasPrefix(s.Type, "map[") {
			t.open[s.FileKey] = true
		}
	}
	for k := range maps.Keys(t.children) {
		slices.Sort(t.children[k])
	}
	return t
}

// unknownKeys reports every config.yml key the schema does not define, with a
// suggestion. The loader's KnownFields stops at the first; this is all of them.
func unknownKeys(node *yaml.Node) []diag.Finding {
	if node == nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	tree := newKeyTree()
	var findings []diag.Finding
	var walk func(m *yaml.Node, parent string)
	walk = func(m *yaml.Node, parent string) {
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			full := k.Value
			if parent != "" {
				full = parent + "." + k.Value
			}
			if !slices.Contains(tree.children[parent], k.Value) {
				findings = append(findings, diag.New("config.unknown-key", config.ConfigFile, k.Line, full,
					unknownKeyMessage(k.Value, parent), unknownKeyFix(k.Value, parent, tree)))
				continue
			}
			if v.Kind == yaml.MappingNode && !tree.open[full] && len(tree.children[full]) > 0 {
				walk(v, full)
			}
		}
	}
	walk(node.Content[0], "")
	return findings
}

func unknownKeyMessage(key, parent string) string {
	if parent == "" {
		return fmt.Sprintf("unknown key `%s` at the top level", key)
	}
	return fmt.Sprintf("unknown key `%s` in `%s`", key, parent)
}

func unknownKeyFix(key, parent string, tree keyTree) string {
	if parent == "" && key == "site" {
		return "config.yml holds the site settings directly: remove the `site:` wrapper and un-indent its keys"
	}
	if parent == "" {
		if s, ok := serverSetting(key); ok {
			return fmt.Sprintf("server settings are not read from config.yml: set %s or the command's flag", s.Env)
		}
	}
	siblings := tree.children[parent]
	if best, ok := closest(key, siblings, 2); ok {
		return fmt.Sprintf("did you mean `%s`?", best)
	}
	return "valid keys here: " + strings.Join(siblings, ", ")
}

// serverSetting finds the server.* setting a misplaced top-level key names.
func serverSetting(key string) (config.Setting, bool) {
	for _, s := range config.Schema() {
		if s.Key == "server."+key || s.Key == "server.http."+key {
			return s, true
		}
	}
	return config.Setting{}, false
}

// unknownEnv reports GOMDDOC_* variables gomddoc does not read. known holds
// exact names and, ending in "_", the prefixes of map settings.
func unknownEnv(environ, known []string) []diag.Finding {
	var exact, prefixes []string
	for _, k := range known {
		if strings.HasSuffix(k, "_") {
			prefixes = append(prefixes, k)
		} else {
			exact = append(exact, k)
		}
	}
	var findings []diag.Finding
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, "GOMDDOC_") || slices.Contains(exact, name) ||
			slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(name, p) }) {
			continue
		}
		fix := "see `gomddoc info` for the variables gomddoc reads"
		if best, ok := closest(name, exact, 6); ok {
			fix = fmt.Sprintf("did you mean %s?", best)
		}
		findings = append(findings, diag.New("env.unknown", "", 0, name,
			fmt.Sprintf("%s is set but gomddoc does not read it", name), fix))
	}
	return findings
}

// closest is the candidate nearest to s within maxDist edits; ties go to the
// first in candidates' order.
func closest(s string, candidates []string, maxDist int) (string, bool) {
	best, bestDist := "", maxDist+1
	for _, c := range candidates {
		if d := levenshtein(s, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= maxDist
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
