package config

import (
	"maps"
	"testing"
)

func TestFeatureEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		feature  string
		features map[string]bool
		want     bool
	}{
		{
			name:     "nil map defaults to enabled",
			feature:  "color_chips",
			features: nil,
			want:     true,
		},
		{
			name:     "empty map defaults to enabled",
			feature:  "heading_anchors",
			features: map[string]bool{},
			want:     true,
		},
		{
			name:     "missing key defaults to enabled",
			feature:  "admonitions",
			features: map[string]bool{"color_chips": false},
			want:     true,
		},
		{
			name:     "explicitly enabled",
			feature:  "color_chips",
			features: map[string]bool{"color_chips": true},
			want:     true,
		},
		{
			name:     "explicitly disabled",
			feature:  "heading_anchors",
			features: map[string]bool{"heading_anchors": false},
			want:     false,
		},
		{
			name:     "multiple features - check enabled one",
			feature:  "toc",
			features: map[string]bool{"color_chips": false, "toc": true, "admonitions": false},
			want:     true,
		},
		{
			name:     "multiple features - check disabled one",
			feature:  "color_chips",
			features: map[string]bool{"color_chips": false, "toc": true, "admonitions": false},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FeatureEnabled(tt.feature, tt.features)
			if got != tt.want {
				t.Errorf("FeatureEnabled(%q, %v) = %v, want %v", tt.feature, tt.features, got, tt.want)
			}
		})
	}
}

func TestExtractPageFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pageMeta map[string]any
		want     map[string]bool
	}{
		{
			name:     "nil pageMeta",
			pageMeta: nil,
			want:     nil,
		},
		{
			name:     "empty pageMeta",
			pageMeta: map[string]any{},
			want:     nil,
		},
		{
			name:     "no features key",
			pageMeta: map[string]any{"title": "Test"},
			want:     nil,
		},
		{
			name:     "features is nil",
			pageMeta: map[string]any{"features": nil},
			want:     nil,
		},
		{
			name:     "features is not a map",
			pageMeta: map[string]any{"features": "not a map"},
			want:     nil,
		},
		{
			name:     "valid bool features",
			pageMeta: map[string]any{"features": map[string]any{"katex": true, "toc": false}},
			want:     map[string]bool{"katex": true, "toc": false},
		},
		{
			name:     "mixed types - non-bool ignored",
			pageMeta: map[string]any{"features": map[string]any{"toc": "yes", "admonitions": 1, "heading_anchors": false}},
			want:     map[string]bool{"heading_anchors": false},
		},
		{
			name:     "all non-bool returns nil",
			pageMeta: map[string]any{"features": map[string]any{"toc": "yes", "admonitions": 1}},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractPageFeatures(tt.pageMeta)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ExtractPageFeatures() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ExtractPageFeatures() = nil, want %v", tt.want)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("ExtractPageFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		base      map[string]bool
		overrides []map[string]bool
		want      map[string]bool
	}{
		{
			name: "nil base no overrides",
			base: nil,
			want: map[string]bool{},
		},
		{
			name: "empty base no overrides",
			base: map[string]bool{},
			want: map[string]bool{},
		},
		{
			name: "base only",
			base: map[string]bool{"color_chips": true, "toc": false},
			want: map[string]bool{"color_chips": true, "toc": false},
		},
		{
			name:      "nil override ignored",
			base:      map[string]bool{"color_chips": true},
			overrides: []map[string]bool{nil},
			want:      map[string]bool{"color_chips": true},
		},
		{
			name:      "single override adds key",
			base:      map[string]bool{"color_chips": true},
			overrides: []map[string]bool{{"toc": false}},
			want:      map[string]bool{"color_chips": true, "toc": false},
		},
		{
			name:      "override replaces key",
			base:      map[string]bool{"color_chips": true, "toc": false},
			overrides: []map[string]bool{{"color_chips": false}},
			want:      map[string]bool{"color_chips": false, "toc": false},
		},
		{
			name:      "multiple overrides - later wins",
			base:      map[string]bool{"toc": true},
			overrides: []map[string]bool{{"toc": false}, {"toc": true, "katex": false}},
			want:      map[string]bool{"toc": true, "katex": false},
		},
		{
			name:      "nil base with override",
			base:      nil,
			overrides: []map[string]bool{{"color_chips": true, "toc": false}},
			want:      map[string]bool{"color_chips": true, "toc": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Store original base for mutation check
			var originalBase map[string]bool
			if tt.base != nil {
				originalBase = maps.Clone(tt.base)
			}

			got := MergeFeatures(tt.base, tt.overrides...)

			if !maps.Equal(got, tt.want) {
				t.Errorf("MergeFeatures() = %v, want %v", got, tt.want)
			}

			// Check that original base was not mutated
			if tt.base != nil && !maps.Equal(tt.base, originalBase) {
				t.Errorf("MergeFeatures() mutated base map: was %v, now %v", originalBase, tt.base)
			}
		})
	}
}

func TestValidateFeatureKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features map[string]bool
		wantErr  bool
	}{
		{
			name:     "nil map is valid",
			features: nil,
			wantErr:  false,
		},
		{
			name:     "empty map is valid",
			features: map[string]bool{},
			wantErr:  false,
		},
		{
			name:     "valid single lowercase key",
			features: map[string]bool{"toc": true},
			wantErr:  false,
		},
		{
			name:     "valid key with underscores",
			features: map[string]bool{"color_chips": true, "heading_anchors": false},
			wantErr:  false,
		},
		{
			name:     "valid key with numbers",
			features: map[string]bool{"h1": true, "h2": false, "h3_heading": true},
			wantErr:  false,
		},
		{
			name:     "valid keys - comprehensive",
			features: map[string]bool{"a": true, "abc": true, "a1": true, "a_b_c": true, "feature123": false, "test_feature_1": true},
			wantErr:  false,
		},
		{
			name:     "invalid - starts with number",
			features: map[string]bool{"1heading": true},
			wantErr:  true,
		},
		{
			name:     "invalid - starts with underscore",
			features: map[string]bool{"_private": true},
			wantErr:  true,
		},
		{
			name:     "invalid - uppercase letter",
			features: map[string]bool{"ColorChips": true},
			wantErr:  true,
		},
		{
			name:     "invalid - contains hyphen",
			features: map[string]bool{"color-chips": true},
			wantErr:  true,
		},
		{
			name:     "invalid - contains space",
			features: map[string]bool{"color chips": true},
			wantErr:  true,
		},
		{
			name:     "invalid - contains special char",
			features: map[string]bool{"color@chips": true},
			wantErr:  true,
		},
		{
			name:     "invalid - empty string",
			features: map[string]bool{"": true},
			wantErr:  true,
		},
		{
			name:     "mixed valid and invalid - should error on first invalid",
			features: map[string]bool{"toc": true, "ColorChips": false, "heading_anchors": true},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFeatureKeys(tt.features)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFeatureKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
