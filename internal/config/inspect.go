package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/diag"
)

// Inspection is a site's configuration loaded for diagnosis: everything
// NewFromServeArgs would apply, with every problem collected rather than the
// first one returned.
type Inspection struct {
	// Config is never nil. Where the file could not be used, defaults (and
	// env overrides) stand in, and Assumed says so.
	Config *Config
	// Node is config.yml's document node, for callers that need key lines
	// (doctor's unknown-key check). Nil when the file is absent, empty or
	// does not parse.
	Node     *yaml.Node
	Findings []diag.Finding
	Assumed  string
}

var yamlLine = regexp.MustCompile(`line (\d+)`)

// Inspect loads dir's configuration the way NewFromServeArgs does — env,
// dynamic defaults, config.yml, site env again, Normalize — but never stops:
// a parse error, type errors, invalid values, replaced values and unparseable
// env vars all become findings. Unknown keys are not checked here (the loader
// rejects them with KnownFields; doctor walks Node against the schema to
// report all of them with suggestions). Nothing is logged.
func Inspect(dir string) Inspection {
	cfg := New()
	findings := cfg.ApplyEnvOverrides()
	cfg.Server.Dir = dir
	cfg.ComputeDynamicDefaults()
	ins := Inspection{Config: cfg}

	node, fileFindings, usable := readConfigNode(dir)
	findings = append(findings, fileFindings...)
	if !usable {
		ins.Assumed = ConfigFile + " could not be used; defaults and environment variables stand in"
	}
	if node != nil {
		ins.Node = node
		var typeErr *yaml.TypeError
		if err := node.Decode(&cfg.Site); errors.As(err, &typeErr) {
			for _, msg := range typeErr.Errors {
				line := lineOf(msg)
				findings = append(findings, diag.New("config.wrong-type", ConfigFile, line, keyAtLine(node, line),
					strings.TrimPrefix(msg, fmt.Sprintf("line %d: ", line)), "see `gomddoc schema` for the expected type"))
			}
		} else if err != nil {
			findings = append(findings, diag.New("config.parse-error", ConfigFile, lineOf(err.Error()), "", err.Error(), ""))
		}
	}

	findings = append(findings, cfg.Site.ApplyEnvOverrides()...)
	findings = append(findings, cfg.Normalize()...)
	findings = append(findings, cfg.ValidateAll()...)
	ins.Findings = locate(diag.Dedupe(findings), ins.Node)
	return ins
}

// readConfigNode reads config.yml into a document node. usable is false when
// the file exists but cannot be applied (read or parse failure).
func readConfigNode(dir string) (node *yaml.Node, findings []diag.Finding, usable bool) {
	data, err := os.ReadFile(filepath.Join(dir, ConfigDirName, ConfigFileName)) // #nosec G304 -- the site's own config file
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, true
	}
	if err != nil {
		return nil, []diag.Finding{diag.New("target.read-error", ConfigFile, 0, "", err.Error(), "")}, false
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, true // empty or comment-only: defaults, as the loader treats it
		}
		return nil, []diag.Finding{diag.New("config.parse-error", ConfigFile, lineOf(err.Error()), "", err.Error(),
			"fix the YAML syntax at that line")}, false
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		line := extra.Line
		if err != nil {
			line = lineOf(err.Error())
		}
		return nil, []diag.Finding{diag.New("config.parse-error", ConfigFile, line, "",
			"only the first YAML document is read", "remove the `---` separator and merge the documents")}, false
	}
	return &doc, nil, true
}

func lineOf(msg string) int {
	if m := yamlLine.FindStringSubmatch(msg); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// locate fills in the line of config-file findings that name a key but were
// produced without one (Normalize, ValidateAll work on structs, not YAML).
func locate(findings []diag.Finding, node *yaml.Node) []diag.Finding {
	if node == nil {
		return findings
	}
	for i, f := range findings {
		if f.File == ConfigFile && f.Line == 0 && f.Key != "" {
			findings[i].Line = keyLine(node, f.Key)
		}
	}
	return findings
}

// root returns the top-level mapping of a document node, or nil.
func root(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		node = node.Content[0]
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	return node
}

// keyLine is the line of the key at a dotted path, or its deepest existing
// ancestor's line (theme.features.Toc under a flow mapping still points at a
// line), or 0.
func keyLine(node *yaml.Node, key string) int {
	m := root(node)
	line := 0
	for part := range strings.SplitSeq(key, ".") {
		if m == nil || m.Kind != yaml.MappingNode {
			return line
		}
		var next *yaml.Node
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == part {
				line, next = m.Content[i].Line, m.Content[i+1]
				break
			}
		}
		if next == nil {
			return line
		}
		m = next
	}
	return line
}

// keyAtLine is the dotted path of the key whose key or scalar value sits on
// line, for yaml.TypeError messages that only carry a line.
func keyAtLine(node *yaml.Node, line int) string {
	var walk func(m *yaml.Node, prefix string) string
	walk = func(m *yaml.Node, prefix string) string {
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			p := k.Value
			if prefix != "" {
				p = prefix + "." + k.Value
			}
			if v.Kind == yaml.MappingNode {
				if found := walk(v, p); found != "" {
					return found
				}
			}
			if k.Line == line || v.Line == line {
				return p
			}
		}
		return ""
	}
	if m := root(node); m != nil {
		return walk(m, "")
	}
	return ""
}
