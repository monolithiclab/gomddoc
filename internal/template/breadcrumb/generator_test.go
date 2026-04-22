package breadcrumb

import (
	"reflect"
	"testing"
)

type mockProvider struct {
	dirs map[string]bool
}

func (m *mockProvider) IsDir(path string) bool {
	return m.dirs[path]
}

func TestGenerate(t *testing.T) {
	provider := &mockProvider{
		dirs: map[string]bool{
			"/":         true,
			"/docs":     true,
			"/docs/api": true,
			"/images":   true,
		},
	}

	generator := NewGenerator(provider)

	tests := []struct {
		name     string
		path     string
		expected []Breadcrumb
	}{
		{
			name: "root path",
			path: "/",
			expected: []Breadcrumb{
				{Path: "/", Label: "Home"},
			},
		},
		{
			name: "file in root",
			path: "/README.md",
			expected: []Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/README.md", Label: "Readme"},
			},
		},
		{
			name: "directory path",
			path: "/docs",
			expected: []Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/docs/", Label: "Docs"},
			},
		},
		{
			name: "nested file",
			path: "/docs/api/guide.md",
			expected: []Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/docs/", Label: "Docs"},
				{Path: "/docs/api/", Label: "Api"},
				{Path: "/docs/api/guide.md", Label: "Guide"},
			},
		},
		{
			name: "path with dots",
			path: "/docs/./api/../images/logo.png",
			expected: []Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/docs/", Label: "Docs"},
				{Path: "/docs/images/", Label: "Images"},
				{Path: "/docs/images/logo.png", Label: "Logo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.Generate(tt.path)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Generate() = %v, want %v", got, tt.expected)
			}
		})
	}
}
