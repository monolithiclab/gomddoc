package config

import (
	"maps"
	"regexp"
	"slices"
)

var featureKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FeatureEnabled returns whether a feature is enabled.
// If the feature key exists in the map, returns its value.
// Otherwise returns true (all features default to enabled).
func FeatureEnabled(name string, features map[string]bool) bool {
	if features == nil {
		return true
	}
	enabled, exists := features[name]
	if !exists {
		return true
	}
	return enabled
}

// ExtractPageFeatures extracts feature toggles from page frontmatter metadata.
// It looks for a "features" key holding a map (map[string]any, or map[any]any
// as goldmark-meta decodes frontmatter) with bool values.
// Non-bool values are silently ignored. Returns nil if no features found.
func ExtractPageFeatures(pageMeta map[string]any) map[string]bool {
	if pageMeta == nil {
		return nil
	}
	featuresRaw, exists := pageMeta["features"]
	if !exists || featuresRaw == nil {
		return nil
	}
	result := make(map[string]bool)
	switch featuresMap := featuresRaw.(type) {
	case map[string]any:
		for key, value := range featuresMap {
			if boolValue, isBool := value.(bool); isBool {
				result[key] = boolValue
			}
		}
	case map[any]any: // goldmark-meta decodes frontmatter with yaml.v2, whose nested maps are keyed by any
		for key, value := range featuresMap {
			name, isString := key.(string)
			if boolValue, isBool := value.(bool); isString && isBool {
				result[name] = boolValue
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// MergeFeatures creates a merged features map from a base map and zero or more
// override maps. Later maps take precedence. Does not mutate any input.
// Returns a non-nil map even if base is nil.
//
// When no overrides contain entries (the common per-request case), base is
// returned directly without cloning. Callers must not mutate the returned map.
func MergeFeatures(base map[string]bool, overrides ...map[string]bool) map[string]bool {
	if !slices.ContainsFunc(overrides, func(m map[string]bool) bool { return len(m) > 0 }) {
		if base == nil {
			return make(map[string]bool)
		}
		return base
	}

	result := maps.Clone(base)
	if result == nil {
		result = make(map[string]bool)
	}
	for _, m := range overrides {
		maps.Copy(result, m)
	}
	return result
}
