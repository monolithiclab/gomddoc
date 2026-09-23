package doctor

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

var atxH1 = regexp.MustCompile(`(?m)^#[ \t]+\S`)

// contentChecks walks one pipeline's markdown pages — with that pipeline's own
// exclude list, so a page doctor checks is a page the site serves — and
// checks each page's frontmatter against metadata.FrontmatterFields, its
// feature toggles against the theme, and whether it has a title and a
// description. Paths are reported relative to the site root.
func contentChecks(ctx context.Context, p PipelineView, scan template.ThemeInfo) []diag.Finding {
	var findings []diag.Finding
	_ = fs.WalkDir(p.Root, ".", func(name string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		file := path.Join(p.Lang, name)
		if err != nil {
			findings = append(findings, diag.New("target.read-error", file, 0, "", err.Error(), "check the path's permissions"))
			return nil
		}
		if skip, skipErr := provider.SkipWalkEntry(name, d.Name(), d.IsDir(), p.Exclude); skip {
			return skipErr
		}
		if d.IsDir() || path.Ext(name) != ".md" {
			return nil
		}
		data, err := fs.ReadFile(p.Root, name)
		if err != nil {
			findings = append(findings, diag.New("target.read-error", file, 0, "", err.Error(), "check the file's permissions"))
			return nil
		}
		findings = append(findings, pageChecks(file, data, scan)...)
		return nil
	})
	return findings
}

// frontmatterOffset turns a line in the frontmatter YAML into a file line:
// the opening --- is line 1.
const frontmatterOffset = 1

func pageChecks(file string, data []byte, scan template.ThemeInfo) []diag.Finding {
	var findings []diag.Finding
	fields := map[string]*yaml.Node{}
	if block, ok := metadata.FrontmatterBlock(data); ok {
		var doc yaml.Node
		if err := yaml.Unmarshal(block, &doc); err != nil {
			return []diag.Finding{diag.New("content.frontmatter-invalid", file, diag.YAMLLine(err.Error())+frontmatterOffset, "",
				"frontmatter is not valid YAML, so the page loses its title, tags and other metadata: "+err.Error(),
				"fix the YAML between the --- lines")}
		}
		if len(doc.Content) == 1 && doc.Content[0].Kind == yaml.MappingNode {
			m := doc.Content[0]
			for i := 0; i+1 < len(m.Content); i += 2 {
				fields[m.Content[i].Value] = m.Content[i+1]
			}
		}
	}

	for _, f := range metadata.FrontmatterFields {
		v, ok := fields[f.Key]
		if !ok {
			continue
		}
		if !hasType(v, f.Type) {
			findings = append(findings, diag.New("content.frontmatter-type", file, v.Line+frontmatterOffset, f.Key,
				fmt.Sprintf("%s must be %s", f.Key, typePhrase(f.Type)),
				fmt.Sprintf("write %s as %s", f.Key, typeExample(f.Key, f.Type))))
		}
	}
	if features := fields["features"]; features != nil && features.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(features.Content); i += 2 {
			k := features.Content[i]
			if finding, ok := unknownFeature(k.Value, scan, file, k.Line+frontmatterOffset, "features."+k.Value); ok {
				findings = append(findings, finding)
			}
		}
	}

	if !nonEmptyString(fields["title"]) && !atxH1.Match(text.StripFrontmatter(data)) {
		findings = append(findings, diag.New("content.missing-title", file, 0, "",
			"no title: neither a frontmatter title nor a # heading", "add title: … to the frontmatter"))
	}
	if !nonEmptyString(fields["description"]) {
		findings = append(findings, diag.New("content.missing-description", file, 0, "",
			"no description: the site description is used for meta tags and search snippets",
			"add description: … to the frontmatter"))
	}
	return findings
}

func nonEmptyString(v *yaml.Node) bool {
	return v != nil && v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value != ""
}

// hasType checks a frontmatter value against a FrontmatterField.Type.
func hasType(v *yaml.Node, typ string) bool {
	switch typ {
	case "string":
		return v.Kind == yaml.ScalarNode && v.Tag == "!!str"
	case "[]string":
		if v.Kind != yaml.SequenceNode {
			return false
		}
		for _, item := range v.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
				return false
			}
		}
		return true
	case "date":
		var raw any
		return v.Kind == yaml.ScalarNode && v.Decode(&raw) == nil && !metadata.ParseFrontmatterDate(raw).IsZero()
	case "map[string]bool":
		if v.Kind != yaml.MappingNode {
			return false
		}
		for i := 1; i < len(v.Content); i += 2 {
			if v.Content[i].Kind != yaml.ScalarNode || v.Content[i].Tag != "!!bool" {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func typePhrase(typ string) string {
	switch typ {
	case "[]string":
		return "a list of strings"
	case "date":
		return "a date (YYYY-MM-DD or RFC 3339)"
	case "map[string]bool":
		return "a map of true/false toggles"
	default:
		return "a string"
	}
}

func typeExample(key, typ string) string {
	switch typ {
	case "[]string":
		return key + ": [a, b]"
	case "date":
		return key + ": 2025-06-15"
	case "map[string]bool":
		return key + ": {toc: false}"
	default:
		return key + ": \"…\""
	}
}
