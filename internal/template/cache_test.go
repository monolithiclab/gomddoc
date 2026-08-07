package template

import (
	"html/template"
	"sync"
	"testing"
)

func TestCachedTemplateStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation string
		key       string
		template  string
		wantNil   bool
	}{
		{
			name:      "set and get existing template",
			operation: "set_get",
			key:       "test-key",
			template:  "Hello {{.}}",
			wantNil:   false,
		},
		{
			name:      "get non-existent template",
			operation: "get_miss",
			key:       "nonexistent",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := &CachedTemplateStore{}

			switch tt.operation {
			case "set_get":
				tmpl := template.Must(template.New("test").Parse(tt.template))
				cache.Set(tt.key, tmpl)

				retrieved := cache.Get(tt.key)
				if retrieved == nil {
					t.Fatal("Get should return stored template")
				}
				if retrieved != tmpl {
					t.Error("Retrieved template should be the same instance")
				}
			case "get_miss":
				retrieved := cache.Get(tt.key)
				if retrieved != nil {
					t.Error("Get should return nil for cache miss")
				}
			}
		})
	}
}

func TestCachedTemplateStore_Concurrent(t *testing.T) {
	t.Parallel()

	cache := &CachedTemplateStore{}

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create and store template
			tmpl := template.Must(template.New("test").Parse("Hello"))
			key := "key"
			cache.Set(key, tmpl)

			// Retrieve template
			_ = cache.Get(key)
		}(i)
	}

	wg.Wait()

	// Cache should still be functional
	if cache.Get("key") == nil {
		t.Error("Cache should have stored at least one template")
	}
}

func TestPassthroughTemplateStore(t *testing.T) {
	t.Parallel()

	cache := &PassthroughTemplateStore{}
	tmpl := template.Must(template.New("test").Parse("Hello {{.}}"))

	// Set should be no-op
	cache.Set("test-key", tmpl)

	// Get should always return nil (no caching)
	if retrieved := cache.Get("test-key"); retrieved != nil {
		t.Error("PassthroughTemplateStore should always return nil on Get")
	}
}
