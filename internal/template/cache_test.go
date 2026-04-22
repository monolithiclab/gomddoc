package template

import (
	"html/template"
	"sync"
	"testing"
)

func TestCachedTemplateStore_GetSet(t *testing.T) {
	cache := &CachedTemplateStore{}

	// Create a test template
	tmpl := template.Must(template.New("test").Parse("Hello {{.}}"))

	// Store template
	cache.Set("test-key", tmpl)

	// Retrieve template
	retrieved := cache.Get("test-key")
	if retrieved == nil {
		t.Fatal("Get should return stored template")
	}

	if retrieved != tmpl {
		t.Error("Retrieved template should be the same instance")
	}
}

func TestCachedTemplateStore_GetMiss(t *testing.T) {
	cache := &CachedTemplateStore{}

	// Try to get non-existent template
	retrieved := cache.Get("nonexistent")
	if retrieved != nil {
		t.Error("Get should return nil for cache miss")
	}
}

func TestCachedTemplateStore_Clear(t *testing.T) {
	cache := &CachedTemplateStore{}

	// Add multiple templates
	tmpl1 := template.Must(template.New("test1").Parse("Template 1"))
	tmpl2 := template.Must(template.New("test2").Parse("Template 2"))
	cache.Set("key1", tmpl1)
	cache.Set("key2", tmpl2)

	// Verify they're stored
	if cache.Get("key1") == nil || cache.Get("key2") == nil {
		t.Fatal("Templates should be stored")
	}

	// Clear cache
	cache.Clear()

	// Verify they're gone
	if cache.Get("key1") != nil {
		t.Error("Cache should be cleared for key1")
	}
	if cache.Get("key2") != nil {
		t.Error("Cache should be cleared for key2")
	}
}

func TestCachedTemplateStore_Concurrent(t *testing.T) {
	cache := &CachedTemplateStore{}

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
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

func TestPassthroughTemplateStore_AlwaysMiss(t *testing.T) {
	cache := &PassthroughTemplateStore{}

	// Create a test template
	tmpl := template.Must(template.New("test").Parse("Hello {{.}}"))

	// Store template (should be no-op)
	cache.Set("test-key", tmpl)

	// Try to get - should always return nil
	retrieved := cache.Get("test-key")
	if retrieved != nil {
		t.Error("PassthroughTemplateStore should always return nil on Get")
	}
}

func TestPassthroughTemplateStore_SetNoOp(t *testing.T) {
	cache := &PassthroughTemplateStore{}

	// Create a test template
	tmpl := template.Must(template.New("test").Parse("Hello {{.}}"))

	// Set should not panic (no-op)
	cache.Set("test-key", tmpl)

	// Verify still returns nil
	if cache.Get("test-key") != nil {
		t.Error("PassthroughTemplateStore should not cache templates")
	}
}

func TestPassthroughTemplateStore_ClearNoOp(t *testing.T) {
	cache := &PassthroughTemplateStore{}

	// Clear should not panic (no-op)
	cache.Clear()
}

func TestNewTemplateCache_DevMode(t *testing.T) {
	cache := NewTemplateCache(true)

	// Should return PassthroughTemplateStore
	if _, ok := cache.(*PassthroughTemplateStore); !ok {
		t.Errorf("NewTemplateCache(true) should return PassthroughTemplateStore, got %T", cache)
	}
}

func TestNewTemplateCache_ProductionMode(t *testing.T) {
	cache := NewTemplateCache(false)

	// Should return CachedTemplateStore
	if _, ok := cache.(*CachedTemplateStore); !ok {
		t.Errorf("NewTemplateCache(false) should return CachedTemplateStore, got %T", cache)
	}
}
