package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/seo"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

// errTemplateNotFound is returned when a theme layout file does not exist.
// This is used to distinguish "not found" (fallback to default) from parse errors (surface immediately).
var errTemplateNotFound = errors.New("theme template not found")

// Renderer defines the interface for template rendering
type Renderer interface {
	// Render renders a template with the given data
	// The context can be used for cancellation, timeouts, and request-scoped values
	Render(ctx context.Context, templateName string, data any) ([]byte, error)

	// HasTemplate checks if a layout template exists for the current theme
	HasTemplate(name string) bool

	// Synthetic tag pages — server-rendered listings derived from the metadata index.
	// RenderTagPage renders a synthetic /tags/{tag} page with pages sorted by the caller.
	RenderTagPage(ctx context.Context, lang string, tFunc func(string) string, tag string, pages []metadata.PageInfo) ([]byte, error)

	// RenderTagsIndex renders a synthetic /tags/ page with all tags and their counts.
	// Tags should already be sorted alphabetically by the caller.
	RenderTagsIndex(ctx context.Context, lang string, tFunc func(string) string, tags []TagCount) ([]byte, error)
}

// LanguageInfo holds display information for a language.
type LanguageInfo struct {
	Code    string // BCP 47 code, e.g. "fr-FR"
	Name    string // Display name, e.g. "Français"
	Active  bool   // Whether this is the current page's language
	Default bool   // Whether this is the site's default language (served without URL prefix)
}

// LanguageNamer resolves BCP 47 codes to display names.
type LanguageNamer interface {
	LanguageName(lang string) string
}

// BuildLanguageInfos builds the full list of LanguageInfo entries for the
// language switcher. The default language is listed first, followed by
// non-default languages in the order they appear.
func BuildLanguageInfos(namer LanguageNamer, defaultLang string, langs []string) []LanguageInfo {
	infos := make([]LanguageInfo, 0, len(langs)+1)
	infos = append(infos, LanguageInfo{
		Code:    defaultLang,
		Name:    namer.LanguageName(defaultLang),
		Default: true,
	})
	for _, lang := range langs {
		infos = append(infos, LanguageInfo{
			Code: lang,
			Name: namer.LanguageName(lang),
		})
	}
	return infos
}

// WithActiveLang returns a copy of infos with the Active flag set for the
// matching language code. Returns nil if infos is empty.
func WithActiveLang(infos []LanguageInfo, activeLang string) []LanguageInfo {
	if len(infos) == 0 {
		return nil
	}
	out := make([]LanguageInfo, len(infos))
	copy(out, infos)
	for i := range out {
		out[i].Active = out[i].Code == activeLang
	}
	return out
}

// TemplateContext holds the data passed to templates
type TemplateContext struct {
	// Site configuration (public, safe to expose)
	Site *config.SiteConfig

	// Page-specific data (namespaced for extensibility)
	Page PageContext

	// i18n support (set by the server/build when creating the context)
	tFunc     func(string) string // translation function bound to current language
	lang      string              // BCP 47 language code for this request
	languages []LanguageInfo      // all available languages
}

type PageContext struct {
	Content     template.HTML
	Path        string             // Current request path
	Meta        map[string]any     // Extracted metadata (e.g., front matter)
	Features    map[string]bool    // Pre-merged feature toggles (site defaults + page overrides)
	TOC         *enricher.TOCNode  // Table of Contents
	Navigation  *enricher.NavTree  // Navigation tree (populated by enricher)
	PrevPage    *enricher.PageLink // Previous page in navigation order
	NextPage    *enricher.PageLink // Next page in navigation order
	RelatedDocs []enricher.RelatedDoc // Pages sharing frontmatter tags with this page
}

// Feature returns whether a named feature is enabled for this page.
// Uses pre-merged features (site defaults + page overrides), defaulting to true.
func (tc *TemplateContext) Feature(name string) bool {
	return config.FeatureEnabled(name, tc.Page.Features)
}

// T returns the translated string for the given key in the current language.
func (tc *TemplateContext) T(key string) string {
	if tc.tFunc != nil {
		return tc.tFunc(key)
	}
	return key
}

// Lang returns the BCP 47 language code for the current page.
// Page-level frontmatter lang overrides the request-level language.
func (tc *TemplateContext) Lang() string {
	if lang, ok := tc.Page.Meta["lang"].(string); ok && lang != "" {
		return lang
	}
	if tc.lang != "" {
		return tc.lang
	}
	return tc.Site.Language
}

// Languages returns all available languages for the language switcher.
func (tc *TemplateContext) Languages() []LanguageInfo {
	return tc.languages
}

// WithI18n returns the context with i18n fields set.
func (tc *TemplateContext) WithI18n(lang string, tFunc func(string) string, languages []LanguageInfo) *TemplateContext {
	tc.lang = lang
	tc.tFunc = tFunc
	tc.languages = languages
	return tc
}

// bufferPool is a sync.Pool for reusing bytes.Buffer objects
// This reduces GC pressure in high-traffic scenarios by reusing buffers
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// HTMLRenderer implements Renderer for HTML templates.
//
// Thread-safe: Render() is safe for concurrent calls. The siteConfig and assetsFS
// fields are read-only after construction. Thread-safety of Render() depends on
// the injected TemplateCache implementation (CachedTemplateStore uses sync.Map,
// PassthroughTemplateStore is stateless).
//
// IMPORTANT: Do not mutate siteConfig after construction if using concurrently.
type HTMLRenderer struct {
	assetsFS       fs.FS
	siteConfig     *config.SiteConfig    // For theme name (NOT full Config - security)
	cache          TemplateCache         // Injected dependency (strategy pattern)
	parseGroup     singleflight.Group    // Coalesces concurrent cache-miss parses
	breadcrumbGen  breadcrumb.Generator  // Optional breadcrumb generator
	resolver       *resolve.PathResolver // Optional path resolver for clean URLs
	themeVars      themeVarsCache        // Cached CSS custom properties from theme config
	hasSearchIndex bool                  // Whether a search index was successfully built
	loggedMissing  sync.Map              // Tracks template names already warned about
	assetCache     sync.Map              // asset name → []byte
	cacheAssets    bool                  // Set by WithCache; off in dev mode
}

// RendererOption is a functional option for configuring HTMLRenderer
type RendererOption func(*HTMLRenderer)

// WithCache enables template and inline-asset caching. Without this option
// the renderer re-parses templates and re-reads assets on every render
// (preview/dev mode behavior).
func WithCache(cache TemplateCache) RendererOption {
	return func(r *HTMLRenderer) {
		r.cache = cache
		r.cacheAssets = true
	}
}

// WithBreadcrumbGenerator sets the breadcrumb generator for the renderer
func WithBreadcrumbGenerator(gen breadcrumb.Generator) RendererOption {
	return func(r *HTMLRenderer) {
		r.breadcrumbGen = gen
	}
}

// WithResolver sets the path resolver for clean URL generation in templates
func WithResolver(resolver *resolve.PathResolver) RendererOption {
	return func(r *HTMLRenderer) {
		r.resolver = resolver
	}
}

// WithSearchIndex indicates that a search index was successfully built.
// This controls JSON-LD SearchAction output, independent of theme feature flags.
func WithSearchIndex() RendererOption {
	return func(r *HTMLRenderer) {
		r.hasSearchIndex = true
	}
}

// NewHTMLRenderer creates a new HTML template renderer
// Required parameters: siteConfig and assetsFS (cannot operate without them)
// Optional parameters: cache (defaults to PassthroughTemplateStore)
// Only SiteConfig is stored (NOT full Config) to prevent leaking operational settings to templates
func NewHTMLRenderer(siteConfig *config.SiteConfig, assetsFS fs.FS, opts ...RendererOption) *HTMLRenderer {
	r := &HTMLRenderer{
		siteConfig: siteConfig,
		assetsFS:   assetsFS,
		cache:      &PassthroughTemplateStore{}, // Default: no caching
	}

	// Apply optional configurations
	r.Configure(opts...)

	return r
}

// Configure configures an HTMLRenderer instance with additional options
func (h *HTMLRenderer) Configure(opts ...RendererOption) {
	for _, opt := range opts {
		opt(h)
	}
}

// Render renders an HTML template with the given data
// Cache behavior is determined by the injected TemplateCache implementation
// No conditional logic needed - PassthroughTemplateStore always returns nil (cache miss)
// The context is checked before expensive operations for cancellation support
func (h *HTMLRenderer) Render(ctx context.Context, templateName string, data any) ([]byte, error) {
	cacheKey := "assets/themes/" + h.siteConfig.Theme.Name + "/layouts/" + templateName

	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try cache first (returns nil if PassthroughTemplateStore)
	tmpl := h.cache.Get(cacheKey)

	var err error
	if tmpl == nil {
		// Check context again before expensive template parsing
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Cache miss — use singleflight to coalesce concurrent parses
		// for the same template. Without this, N concurrent requests on a
		// cold cache all trigger independent parseTemplate calls.
		v, sfErr, _ := h.parseGroup.Do(cacheKey, func() (any, error) {
			// Double-check cache: another flight may have populated it
			if cached := h.cache.Get(cacheKey); cached != nil {
				return cached, nil
			}
			parsed, parseErr := h.parseTemplate(templateName)
			if parseErr != nil {
				return nil, parseErr
			}
			h.cache.Set(cacheKey, parsed)
			return parsed, nil
		})
		if sfErr != nil {
			return nil, fmt.Errorf("parse template: %w", sfErr)
		}
		tmpl = v.(*template.Template)
	}

	// Execute template with pooled buffer to reduce GC pressure
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		if buf.Cap() <= 65536 {
			bufferPool.Put(buf)
		}
	}()

	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}

	// Copy bytes since buffer will be reused
	return slices.Clone(buf.Bytes()), nil
}

// RenderTagPage renders the body of a /tags/{tag} page through the standard
// theme layout. lang is the active BCP-47 code ("" for default language).
// tFunc is the translator scoped to lang. Pages should already be sorted
// by the caller.
func (h *HTMLRenderer) RenderTagPage(ctx context.Context, lang string, tFunc func(string) string, tag string, pages []metadata.PageInfo) ([]byte, error) {
	if tFunc == nil {
		tFunc = func(k string) string { return k }
	}
	body, err := h.executePartial("tags-list", tagPageData{
		Tag:   tag,
		Pages: pages,
		Lang:  lang,
		T:     tFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("render tags-list: %w", err)
	}

	page := PageContext{
		Path:    tagURL(h.siteConfig.Language, lang, tag),
		Content: template.HTML(body), // #nosec G203 -- partial output is trusted
		Meta:    map[string]any{"title": tag},
	}
	tc := &TemplateContext{Site: h.siteConfig, Page: page}
	tc.WithI18n(lang, tFunc, nil)
	return h.Render(ctx, "default.html.tmpl", tc)
}

// tagPageData is the data passed to the tags-list partial.
type tagPageData struct {
	Tag   string
	Pages []metadata.PageInfo
	Lang  string
	T     func(string) string
}

// TagCount is one entry in the tag index.
type TagCount struct {
	Tag   string
	Count int
}

// RenderTagsIndex renders the body of /tags/ (the index of all tags) through
// the standard theme layout. tags should already be sorted alphabetically.
func (h *HTMLRenderer) RenderTagsIndex(ctx context.Context, lang string, tFunc func(string) string, tags []TagCount) ([]byte, error) {
	if tFunc == nil {
		tFunc = func(k string) string { return k }
	}
	body, err := h.executePartial("tags-index", tagsIndexData{
		Tags: tags,
		Lang: lang,
		T:    tFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("render tags-index: %w", err)
	}

	pagePath := "/tags/"
	if lang != "" {
		pagePath = "/" + lang + "/tags/"
	}
	page := PageContext{
		Path:    pagePath,
		Content: template.HTML(body), // #nosec G203 -- partial output is trusted
	}
	tc := &TemplateContext{Site: h.siteConfig, Page: page}
	tc.WithI18n(lang, tFunc, nil)
	return h.Render(ctx, "default.html.tmpl", tc)
}

type tagsIndexData struct {
	Tags []TagCount
	Lang string
	T    func(string) string
}

// executePartial runs a single named partial against data and returns the
// rendered bytes. Used for server-side composition of synthetic pages
// (tag listings, tag index) where the body is pre-built then passed
// through the standard layout via Page.Content.
func (h *HTMLRenderer) executePartial(name string, data any) ([]byte, error) {
	partialPath := path.Join("assets", "themes", h.siteConfig.Theme.Name, "partials", name+".html.tmpl")
	tmpl, err := template.New(name+".html.tmpl").Funcs(h.funcMap()).ParseFS(h.assetsFS, partialPath)
	if err != nil {
		// Fall back to default theme partial.
		fallback := path.Join("assets", "themes", config.DefaultThemeName, "partials", name+".html.tmpl")
		tmpl, err = template.New(name+".html.tmpl").Funcs(h.funcMap()).ParseFS(h.assetsFS, fallback)
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parseTemplate parses a layout template with its partials, with automatic fallback to default theme.
// Partials are discovered via glob in the theme's partials/ directory and parsed together
// with the layout so that {{ template "partial-name" . }} calls work.
func (h *HTMLRenderer) parseTemplate(templateName string) (*template.Template, error) {
	tmpl, err := h.parseThemeTemplate(templateName, h.siteConfig.Theme.Name)
	if err != nil && h.siteConfig.Theme.Name != config.DefaultThemeName {
		// Only fall back to default theme if the layout file is missing.
		// If the file exists but fails to compile, surface the error for debugging.
		if !errors.Is(err, errTemplateNotFound) {
			return nil, err
		}
		if _, loaded := h.loggedMissing.LoadOrStore(templateName, struct{}{}); !loaded {
			slog.Warn("Theme template not found, falling back to default",
				slog.String("theme", h.siteConfig.Theme.Name),
				slog.String("template", templateName))
		}
		tmpl, err = h.parseThemeTemplate(templateName, config.DefaultThemeName)
	}
	return tmpl, err
}

// parseThemeTemplate parses a layout and its partials for a specific theme.
// The layout is loaded from assets/themes/{theme}/layouts/{templateName}.
// Any partials in assets/themes/{theme}/partials/*.html.tmpl are parsed alongside it.
//
// Partial resolution order (last parsed wins):
//  1. Default theme partials (if theme != default) — baseline definitions
//  2. Theme partials — theme-specific overrides
//  3. Site-level partials at partials/*.html.tmpl — user overrides from .gomddoc/partials/
func (h *HTMLRenderer) parseThemeTemplate(templateName, theme string) (*template.Template, error) {
	layoutPath := path.Join("assets", "themes", theme, "layouts", templateName)
	themePartialsGlob := path.Join("assets", "themes", theme, "partials", "*.html.tmpl")

	// Check if the layout file exists before attempting to parse it.
	// This distinguishes "not found" (safe to fallback) from "found but broken" (must surface).
	if _, err := fs.Stat(h.assetsFS, layoutPath); err != nil {
		return nil, fmt.Errorf("%w: %s in theme %q", errTemplateNotFound, templateName, theme)
	}

	// Start with layout only; partials are layered in priority order below.
	tmpl, err := template.New(templateName).Funcs(h.funcMap()).ParseFS(h.assetsFS, layoutPath)
	if err != nil {
		return nil, err
	}

	// For non-default themes, load default theme partials as a baseline
	// so the theme only needs to override the partials it changes.
	if theme != config.DefaultThemeName {
		defaultPartialsGlob := path.Join("assets", "themes", config.DefaultThemeName, "partials", "*.html.tmpl")
		if _, err := h.parseGlob(tmpl, defaultPartialsGlob); err != nil {
			return nil, fmt.Errorf("parse default theme partials: %w", err)
		}
	}

	// Theme partials override default partials.
	if _, err := h.parseGlob(tmpl, themePartialsGlob); err != nil {
		return nil, fmt.Errorf("parse theme partials: %w", err)
	}

	// Site-level partials override everything. These live at partials/*.html.tmpl
	// in the overlay FS, mapping to .gomddoc/partials/ on disk.
	sitePartialsGlob := path.Join("partials", "*.html.tmpl")
	if _, err := h.parseGlob(tmpl, sitePartialsGlob); err != nil {
		return nil, fmt.Errorf("parse site partials: %w", err)
	}

	return tmpl, nil
}

// parseGlob parses files matching glob into tmpl if any matches exist.
// Returns true if files were parsed, false if no matches found.
func (h *HTMLRenderer) parseGlob(tmpl *template.Template, glob string) (bool, error) {
	if matches, _ := fs.Glob(h.assetsFS, glob); len(matches) > 0 {
		if _, err := tmpl.ParseFS(h.assetsFS, glob); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// funcMap returns the map of functions available in templates
func (h *HTMLRenderer) funcMap() template.FuncMap {
	return template.FuncMap{
		"breadcrumbs":  h.generateBreadcrumbs,
		"toc":          filterTOC,
		"editURL":      h.generateEditURL,
		"themeVarsCSS": h.generateThemeVarsCSS,
		"canonicalURL": func(pagePath string) string {
			return seo.PageURL(h.siteConfig.Meta.Domain, pagePath, h.siteConfig.DefaultIndex)
		},
		"jsonLD": func(page PageContext) template.JS {
			return h.generateJSONLD(page)
		},
		"assetURL": func(name string) string {
			return "/_assets/" + name
		},
		"contentURL":      h.contentURL,
		"inlineJSAsset":   h.inlineJSAsset,
		"inlineCSSAsset":  h.inlineCSSAsset,
		"inlineHTMLAsset": h.inlineHTMLAsset,
		"tagURL": func(lang, tag string) string {
			return tagURL(h.siteConfig.Language, lang, tag)
		},
		"pageTags": pageTags,
	}
}

// generateJSONLD produces JSON-LD structured data for a page.
func (h *HTMLRenderer) generateJSONLD(page PageContext) template.JS {
	cfg := seo.JSONLDConfig{
		Domain:       h.siteConfig.Meta.Domain,
		SiteName:     h.siteConfig.Meta.Title,
		DefaultIndex: h.siteConfig.DefaultIndex,
		HasSearch:    h.hasSearchIndex,
	}

	// Build breadcrumbs with full URLs
	var breadcrumbs []seo.BreadcrumbItem
	if h.breadcrumbGen != nil {
		for _, bc := range h.breadcrumbGen.Generate(page.Path) {
			breadcrumbs = append(breadcrumbs, seo.BreadcrumbItem{
				Name: bc.Label,
				URL:  seo.PageURL(cfg.Domain, bc.Path, cfg.DefaultIndex),
			})
		}
	}

	// Detect index page
	isIndex := page.Path == "/" || path.Base(page.Path) == h.siteConfig.DefaultIndex

	p := seo.JSONLDPage{
		Path:        page.Path,
		Breadcrumbs: breadcrumbs,
		IsIndex:     isIndex,
	}

	// Extract metadata fields
	if title, ok := page.Meta["title"].(string); ok {
		p.Title = title
	}
	if desc, ok := page.Meta["description"].(string); ok {
		p.Description = desc
	}
	if author, ok := page.Meta["author"].(string); ok {
		p.Author = author
	}

	raw := seo.GenerateJSONLD(cfg, p)
	if raw == "" {
		return ""
	}
	return template.JS(raw) // #nosec G203 -- trusted JSON-LD output
}

// generateEditURL generates the full edit URL for a page by combining
// the configured EditURL base with the page path. Returns an empty string
// if EditURL is not configured, which templates use to conditionally hide the link.
func (h *HTMLRenderer) generateEditURL(pagePath string) string {
	if h.siteConfig.EditURL == "" {
		return ""
	}
	base := strings.TrimRight(h.siteConfig.EditURL, "/")
	if !strings.HasPrefix(pagePath, "/") {
		pagePath = "/" + pagePath
	}
	return base + pagePath
}

// generateBreadcrumbs generates breadcrumbs for the given path
func (h *HTMLRenderer) generateBreadcrumbs(path string) []breadcrumb.Breadcrumb {
	if h.breadcrumbGen == nil {
		return nil
	}
	return h.breadcrumbGen.Generate(path)
}

// filterTOC returns a filtered list of TOCNode children for template rendering.
// Nodes outside the min/max level range are pruned: nodes below min are traversed
// transparently (their children promoted), nodes above max are dropped entirely.
// Default levels: 1–2.
func filterTOC(toc *enricher.TOCNode, levels ...int) []*enricher.TOCNode {
	if toc == nil || len(toc.Children) == 0 {
		return nil
	}

	minLevel := 1
	maxLevel := 2
	if len(levels) > 0 {
		minLevel = levels[0]
	}
	if len(levels) > 1 {
		maxLevel = levels[1]
	}

	return filterTOCNodes(toc.Children, minLevel, maxLevel)
}

// filterTOCNodes recursively filters a slice of TOCNode by heading level range.
// Nodes within [min, max] are kept with their children filtered recursively.
// Nodes below min are skipped but their children are promoted (transparent traversal).
// Nodes above max are dropped entirely.
func filterTOCNodes(nodes []*enricher.TOCNode, min, max int) []*enricher.TOCNode {
	var result []*enricher.TOCNode
	for _, node := range nodes {
		if node.Level > max {
			continue
		}
		if node.Level >= min {
			result = append(result, &enricher.TOCNode{
				Level:    node.Level,
				Text:     node.Text,
				ID:       node.ID,
				Children: filterTOCNodes(node.Children, min, max),
			})
		} else {
			// Below min level — promote children (transparent traversal)
			result = append(result, filterTOCNodes(node.Children, min, max)...)
		}
	}
	return result
}

// ResolveLayout determines the template name to use based on frontmatter metadata.
// If the metadata contains a "layout" field and the corresponding template exists,
// it returns "{layout}.html.tmpl". Otherwise, it falls back to "default.html.tmpl".
func ResolveLayout(r Renderer, metadata map[string]any) string {
	const defaultTemplate = "default.html.tmpl"
	layout, ok := metadata["layout"].(string)
	if !ok || layout == "" {
		return defaultTemplate
	}
	candidate := layout + ".html.tmpl"
	if r.HasTemplate(candidate) {
		return candidate
	}
	return defaultTemplate
}

// contentURL returns the absolute URL path (without scheme or domain) for a content file.
// If extension stripping is active and the file has a clean path, the extensionless form is returned.
// Default index files (e.g., README.md) are stripped to their directory path.
func (h *HTMLRenderer) contentURL(filePath string) string {
	// Normalize: strip leading slash for resolver lookup
	p := strings.TrimPrefix(filePath, "/")

	// Default index files map to their directory path, not an extensionless path.
	// The resolver would produce "docs/README" but the correct URL is "docs/".
	if path.Base(p) == h.siteConfig.DefaultIndex {
		p = path.Dir(p)
		if p == "." {
			p = ""
		}
		return "/" + p
	}

	// Try resolver for clean (extensionless) path
	if h.resolver != nil {
		if clean, ok := h.resolver.CleanPath(p); ok {
			p = clean
		}
	}

	return "/" + p
}

// tagURL builds the URL for a tag's listing page, scoped to the active
// language. The default language uses /tags/{tag}; other languages use
// /{lang}/tags/{tag}. Tag values are percent-encoded for URL safety.
func tagURL(defaultLang, lang, tag string) string {
	if lang == "" || lang == defaultLang {
		return "/tags/" + url.PathEscape(tag)
	}
	return "/" + lang + "/tags/" + url.PathEscape(tag)
}

// pageTags normalizes the YAML-decoded value of frontmatter "tags" into a
// []string. Returns nil when the key is absent, the value is the wrong type,
// or the list contains no strings.
func pageTags(meta map[string]any) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta["tags"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// HasTemplate checks if a layout template exists for the current theme.
// It checks only the configured theme directory (not the default fallback),
// since parseTemplate already handles theme-to-default fallback during rendering.
func (h *HTMLRenderer) HasTemplate(name string) bool {
	layoutPath := path.Join("assets", "themes", h.siteConfig.Theme.Name, "layouts", name)
	_, err := fs.Stat(h.assetsFS, layoutPath)
	return err == nil
}

// ClearCache clears the template and inline-asset caches (used in dev mode hot reload).
// Delegates to cache implementation (no-op for PassthroughTemplateStore).
func (h *HTMLRenderer) ClearCache() {
	h.cache.Clear()
	h.assetCache.Clear()
	slog.Info("[DEV] Template cache cleared")
}

// ValidateDefaultTheme checks that the default theme exists in the asset filesystem
// This is a fatal error if missing, as the application cannot function without it
func (h *HTMLRenderer) ValidateDefaultTheme() error {
	defaultTemplate := path.Join("assets", "themes", config.DefaultThemeName, "layouts", "default.html.tmpl")
	f, err := h.assetsFS.Open(defaultTemplate)
	if err != nil {
		return fmt.Errorf("default theme not found: %w (this is a fatal error)", err)
	}
	_ = f.Close()
	slog.Debug("Default theme validated", slog.String("template", defaultTemplate))
	return nil
}
