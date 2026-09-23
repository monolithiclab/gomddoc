package config

import (
	"reflect"
	"strings"
	"time"
)

// Setting describes one configuration value. Schema derives every Setting from
// the config structs' tags (env, yaml, doc, max, default_doc), which makes those
// tags the single source for everything that describes configuration: the JSON
// Schema, `gomddoc info` and the MCP capabilities report. Document a field on
// the field.
type Setting struct {
	Key         string   `json:"key"`                    // "site.theme.name", "server.http.write_timeout"
	FileKey     string   `json:"file_key,omitempty"`     // key inside config.yml ("theme.name"); "" for server.*
	Env         string   `json:"env,omitempty"`          // "GOMDDOC_SITE_THEME_NAME"; "" when none
	Flags       []string `json:"flags,omitempty"`        // CLI flags and args that set it; filled by the caller
	Type        string   `json:"type"`                   // string, bool, int, duration, []string, map[string]string, map[string]bool
	Default     any      `json:"default"`                // durations as strings ("30s"); nil when DefaultNote is set
	DefaultNote string   `json:"default_note,omitempty"` // describes a computed default
	Description string   `json:"description"`
	Max         string   `json:"max,omitempty"`
}

// Precedence is the order in which configuration sources override each other,
// highest first. server.* settings have no file source.
var Precedence = []string{"flag", "env", "file", "default"}

// FileKeysNote states how Setting.Key maps onto config.yml. The JSON Schema and
// the capabilities report both quote it, so they say it the same way.
const FileKeysNote = "config.yml holds the site.* settings with the `site.` prefix dropped " +
	"(site.theme.name is `theme: {name: …}`); server.* settings are set by flag or environment variable only."

// Schema returns every leaf configuration setting in struct order. It builds a
// fresh slice on each call, so callers may fill Flags without copying.
func Schema() []Setting {
	var out []Setting
	walkSchema(reflect.ValueOf(New()).Elem(), "", "GOMDDOC", &out)
	return out
}

func walkSchema(v reflect.Value, keyPrefix, envPrefix string, out *[]Setting) {
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		key := settingName(sf)
		if keyPrefix != "" {
			key = keyPrefix + "." + key
		}
		env := ""
		if tag := sf.Tag.Get("env"); tag != "" {
			env = envPrefix + "_" + tag
		}
		if sf.Type.Kind() == reflect.Struct {
			childPrefix := env
			if childPrefix == "" {
				childPrefix = envPrefix
			}
			walkSchema(v.Field(i), key, childPrefix, out)
			continue
		}
		*out = append(*out, newSetting(sf, v.Field(i), key, env))
	}
}

// settingName is the yaml key when the field has one, else its env segment
// lowercased: server.* fields are not file-settable and carry no yaml tag.
func settingName(sf reflect.StructField) string {
	if name, _, _ := strings.Cut(sf.Tag.Get("yaml"), ","); name != "" {
		return name
	}
	return strings.ToLower(sf.Tag.Get("env"))
}

func newSetting(sf reflect.StructField, fv reflect.Value, key, env string) Setting {
	s := Setting{
		Key:         key,
		Env:         env,
		Type:        settingType(sf.Type),
		Default:     settingDefault(fv),
		DefaultNote: sf.Tag.Get("default_doc"),
		Description: sf.Tag.Get("doc"),
		Max:         sf.Tag.Get("max"),
	}
	if rest, ok := strings.CutPrefix(key, "site."); ok {
		s.FileKey = rest
	}
	// Map-valued env overrides are read per key (walkStruct scans for the
	// prefix), so the variable an agent sets is the prefix plus the key.
	if env != "" && sf.Type.Kind() == reflect.Map {
		s.Env = env + "_<KEY>"
	}
	if s.DefaultNote != "" {
		s.Default = nil
	}
	return s
}

func settingType(t reflect.Type) string {
	if t == reflect.TypeFor[time.Duration]() {
		return "duration"
	}
	return t.String()
}

func settingDefault(fv reflect.Value) any {
	if d, ok := fv.Interface().(time.Duration); ok {
		return d.String()
	}
	if fv.Kind() == reflect.Map && fv.IsNil() {
		return nil
	}
	return fv.Interface()
}
