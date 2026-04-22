package config

import (
	"fmt"
	"maps"
	"regexp"
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
// It looks for a "features" key containing map[string]any with bool values.
// Non-bool values are silently ignored. Returns nil if no features found.
func ExtractPageFeatures(pageMeta map[string]any) map[string]bool {
	if pageMeta == nil {
		return nil
	}
	featuresRaw, exists := pageMeta["features"]
	if !exists || featuresRaw == nil {
		return nil
	}
	featuresMap, ok := featuresRaw.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]bool, len(featuresMap))
	for key, value := range featuresMap {
		if boolValue, isBool := value.(bool); isBool {
			result[key] = boolValue
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
func MergeFeatures(base map[string]bool, overrides ...map[string]bool) map[string]bool {
	result := maps.Clone(base)
	if result == nil {
		result = make(map[string]bool)
	}
	for _, m := range overrides {
		maps.Copy(result, m)
	}
	return result
}

// ValidateFeatureKeys validates that all feature keys match the pattern ^[a-z][a-z0-9_]*$.
// Returns an error for the first invalid key found.
func ValidateFeatureKeys(features map[string]bool) error {
	if features == nil {
		return nil
	}

	for key := range features {
		if !featureKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid feature key %q: must match pattern ^[a-z][a-z0-9_]*$", key)
		}
	}

	return nil
}
