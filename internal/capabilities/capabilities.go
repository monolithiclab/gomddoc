// Package capabilities answers "what can gomddoc do, and how is it
// configured" as one Report: settings (from config struct tags), commands
// (from Kong's model, converted by the caller), special frontmatter keys, the
// active theme's features, and the embedded guide's pages.
//
// Every transport renders the same Report — `gomddoc info` as text, `info
// --json` and the gomddoc://capabilities MCP resource through Report.JSON — so
// none of them may assemble its own. Describe never fails: a config that did
// not load or a theme that is not installed is reported in-band, because the
// situations where an agent asks for this report are exactly the ones where
// something is wrong.
package capabilities

import (
	"cmp"
	"context"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
)

// URIs of the self-description resources. The MCP server serves them; the
// report cites them so an agent reading one knows where the others are.
const (
	CapabilitiesURI = "gomddoc://capabilities"
	SchemaURI       = "gomddoc://schema/config"
	GuideURIPrefix  = "gomddoc://guide/"
)

// Input is everything Describe needs. Config, Assets and Guide may be nil.
type Input struct {
	Version   string
	Dir       string
	Config    *config.Config // nil when it could not be loaded
	ConfigErr error          // why it could not; reported as text
	FileFound bool           // .gomddoc/config.yml exists under Dir
	Commands  []Command
	Assets    fs.FS // asset FS as the renderer sees it; nil reports the theme unavailable
	Guide     fs.FS
}

// Command is one CLI subcommand.
type Command struct {
	Name  string `json:"name"`
	Help  string `json:"help"`
	Args  []Arg  `json:"args,omitempty"`
	Flags []Flag `json:"flags,omitempty"`
}

// Arg is a positional argument.
type Arg struct {
	Name     string `json:"name"`
	Help     string `json:"help"`
	Default  string `json:"default,omitempty"`
	Env      string `json:"env,omitempty"`
	Required bool   `json:"required"`
}

// Flag is a command-line flag.
type Flag struct {
	Name    string `json:"name"`            // "--port"
	Short   string `json:"short,omitempty"` // "-p"
	Help    string `json:"help"`
	Default string `json:"default,omitempty"`
	Env     string `json:"env,omitempty"`
	// Setting is the config key the flag sets when its env var is not that
	// setting's (the Kong `setting:` tag). Empty: joined by Env.
	Setting string `json:"setting,omitempty"`
}

// ConfigInfo says where configuration comes from and whether it loaded.
type ConfigInfo struct {
	Dir        string   `json:"dir"`
	File       string   `json:"file"`
	FileFound  bool     `json:"file_found"`
	FileKeys   string   `json:"file_keys"`
	Precedence []string `json:"precedence"`
	Loaded     bool     `json:"loaded"`
	LoadError  string   `json:"load_error,omitempty"`
	SchemaURI  string   `json:"schema_uri"`
}

// ThemeReport is the active theme's facts plus where they are documented.
type ThemeReport struct {
	template.ThemeInfo
	Docs string `json:"docs"`
}

// GuidePage is one page of the embedded guide.
type GuidePage struct {
	Path        string `json:"path"` // "02-configuration.md", no leading slash
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Report is gomddoc's self-description.
type Report struct {
	Version     string                      `json:"version"`
	Config      ConfigInfo                  `json:"config"`
	Settings    []config.Setting            `json:"settings"`
	Commands    []Command                   `json:"commands"`
	Frontmatter []metadata.FrontmatterField `json:"frontmatter"`
	Theme       ThemeReport                 `json:"theme"`
	Guide       []GuidePage                 `json:"guide"`
	GuideError  string                      `json:"guide_error,omitempty"`
}

// Describe builds the report.
func Describe(in Input) Report {
	r := Report{
		Version:     in.Version,
		Settings:    config.Schema(),
		Commands:    in.Commands,
		Frontmatter: metadata.FrontmatterFieldList(),
		Config: ConfigInfo{
			Dir:        in.Dir,
			File:       config.ConfigDirName + "/" + config.ConfigFileName,
			FileFound:  in.FileFound,
			FileKeys:   config.FileKeysNote,
			Precedence: slices.Clone(config.Precedence),
			Loaded:     in.Config != nil,
			SchemaURI:  SchemaURI,
		},
	}
	if in.ConfigErr != nil {
		r.Config.LoadError = in.ConfigErr.Error()
	}
	joinFlags(r.Settings, in.Commands)

	themeName := config.DefaultThemeName
	if in.Config != nil {
		themeName = in.Config.Site.Theme.Name
	}
	info := template.ThemeInfo{Name: themeName, Source: template.ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	if in.Assets != nil {
		info = template.ThemeFeatures(in.Assets, themeName)
	}
	r.Theme = ThemeReport{ThemeInfo: info, Docs: GuideURIPrefix + "05-theming-and-assets.md"}

	pages, err := guidePages(in.Guide)
	r.Guide = pages
	if err != nil {
		r.GuideError = err.Error()
	}
	return r
}

// joinFlags records, on each setting, the flags and positional args that set
// it: by the flag's Setting when present, else by env var name — the one key
// both a Kong flag and a config field already carry.
func joinFlags(settings []config.Setting, cmds []Command) {
	byEnv, byKey := map[string]int{}, map[string]int{}
	for i, s := range settings {
		byKey[s.Key] = i
		if s.Env != "" {
			byEnv[s.Env] = i
		}
	}
	add := func(i int, name string) {
		if !slices.Contains(settings[i].Flags, name) {
			settings[i].Flags = append(settings[i].Flags, name)
		}
	}
	for _, c := range cmds {
		for _, f := range c.Flags {
			if f.Setting != "" {
				if i, ok := byKey[f.Setting]; ok {
					add(i, f.Name)
				}
			} else if i, ok := byEnv[f.Env]; ok {
				add(i, f.Name)
			}
		}
		for _, a := range c.Args {
			if i, ok := byEnv[a.Env]; ok {
				add(i, strings.ToUpper(a.Name))
			}
		}
	}
	for i := range settings {
		slices.Sort(settings[i].Flags)
	}
}

func guidePages(guide fs.FS) ([]GuidePage, error) {
	pages := []GuidePage{}
	if guide == nil {
		return pages, nil
	}
	idx, err := metadata.BuildIndex(context.Background(), guide, nil)
	if err != nil {
		return pages, err
	}
	for _, p := range idx.AllPages() {
		pages = append(pages, GuidePage{strings.TrimPrefix(p.Path, "/"), p.Title, p.Description})
	}
	slices.SortFunc(pages, func(a, b GuidePage) int { return cmp.Compare(a.Path, b.Path) })
	return pages, nil
}

// JSON is the one serialization of a Report; every transport uses it.
func (r Report) JSON() []byte {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic("capabilities: marshal report: " + err.Error()) // plain data; cannot fail
	}
	return data
}
