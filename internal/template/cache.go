package template

import (
	"html/template"
	"sync"
)

// TemplateCache is an interface for caching parsed templates
// Implementations: CachedTemplateStore (production), PassthroughTemplateStore (dev mode)
type TemplateCache interface {
	// Get retrieves a template from cache, returns nil if not found
	Get(key string) *template.Template

	// Set stores a template in cache
	Set(key string, tmpl *template.Template)

	// Clear removes all cached templates (used in dev mode hot reload)
	Clear()
}

// CachedTemplateStore caches templates for production use
// Thread-safe implementation using sync.Map
type CachedTemplateStore struct {
	cache sync.Map
}

func (c *CachedTemplateStore) Get(key string) *template.Template {
	if val, ok := c.cache.Load(key); ok {
		return val.(*template.Template)
	}
	return nil
}

func (c *CachedTemplateStore) Set(key string, tmpl *template.Template) {
	c.cache.Store(key, tmpl)
}

func (c *CachedTemplateStore) Clear() {
	c.cache.Range(func(key, value any) bool {
		c.cache.Delete(key)
		return true
	})
}

// PassthroughTemplateStore is a no-op cache for development mode
// Always returns nil on Get, forcing template re-parsing on every request
type PassthroughTemplateStore struct{}

func (p *PassthroughTemplateStore) Get(key string) *template.Template {
	return nil // Always miss - forces re-parsing
}

func (p *PassthroughTemplateStore) Set(key string, tmpl *template.Template) {
	// No-op - don't cache in dev mode
}

func (p *PassthroughTemplateStore) Clear() {
	// No-op - nothing to clear
}

// NewTemplateCache creates appropriate cache implementation based on dev mode
// Factory pattern: devMode=true → PassthroughTemplateStore, devMode=false → CachedTemplateStore
func NewTemplateCache(devMode bool) TemplateCache {
	if devMode {
		return &PassthroughTemplateStore{}
	}
	return &CachedTemplateStore{}
}
