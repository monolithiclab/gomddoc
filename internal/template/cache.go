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

// CachedTemplateStore caches parsed templates for production use.
//
// Thread-safe: All methods are safe for concurrent access via sync.Map.
//
// IMPORTANT: This cache is unbounded with no eviction policy.
// For typical usage (1-2 themes, <10 templates), memory usage is stable.
//
// For deployments with many themes or dynamic content, consider:
//   - LRU eviction policy
//   - Maximum cache size
//   - TTL-based expiration
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
	c.cache.Clear()
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
