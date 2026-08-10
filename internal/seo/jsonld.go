package seo

import (
	"encoding/json"
	"time"
)

// JSONLDConfig holds site-level configuration for JSON-LD generation.
type JSONLDConfig struct {
	Domain       string
	SiteName     string
	DefaultIndex string
	HasSearch    bool
}

// JSONLDPage holds page-level data for JSON-LD generation.
type JSONLDPage struct {
	Path        string
	Title       string
	Description string
	Author      string

	// Date is the frontmatter `date` — when the page was published.
	Date time.Time
	// Modified is the source file's mtime; zero when unknown, see LastModified.
	Modified time.Time

	Breadcrumbs []BreadcrumbItem
	IsIndex     bool
}

// BreadcrumbItem represents a single breadcrumb for JSON-LD.
type BreadcrumbItem struct {
	Name string
	URL  string
}

// GenerateJSONLD produces JSON-LD structured data for a page.
// Returns raw JSON array string (without <script> wrapper).
// Returns "" when domain is empty.
func GenerateJSONLD(cfg JSONLDConfig, page JSONLDPage) string {
	if cfg.Domain == "" {
		return ""
	}

	pageURL := PageURL(cfg.Domain, page.Path, cfg.DefaultIndex)
	if pageURL == "" {
		return ""
	}

	var schemas []map[string]any

	// TechArticle schema
	article := map[string]any{
		"@context": "https://schema.org",
		"@type":    "TechArticle",
		"url":      pageURL,
		"mainEntityOfPage": map[string]any{
			"@type": "WebPage",
			"@id":   pageURL,
		},
	}
	if page.Title != "" {
		article["headline"] = page.Title
	}
	if page.Description != "" {
		article["description"] = page.Description
	}
	if page.Author != "" {
		article["author"] = map[string]any{
			"@type": "Person",
			"name":  page.Author,
		}
	}
	if !page.Date.IsZero() {
		article["datePublished"] = page.Date.UTC().Format(time.RFC3339)
	}
	if modified := LastModified(page.Modified, page.Date); !modified.IsZero() {
		article["dateModified"] = modified.Format(time.RFC3339)
	}
	schemas = append(schemas, article)

	// BreadcrumbList schema (only if >1 breadcrumb)
	if len(page.Breadcrumbs) > 1 {
		items := make([]map[string]any, len(page.Breadcrumbs))
		for i, bc := range page.Breadcrumbs {
			items[i] = map[string]any{
				"@type":    "ListItem",
				"position": i + 1,
				"name":     bc.Name,
				"item":     bc.URL,
			}
		}
		schemas = append(schemas, map[string]any{
			"@context":        "https://schema.org",
			"@type":           "BreadcrumbList",
			"itemListElement": items,
		})
	}

	// WebSite schema (index page only)
	if page.IsIndex {
		website := map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"url":      PageURL(cfg.Domain, "/", cfg.DefaultIndex),
		}
		if cfg.SiteName != "" {
			website["name"] = cfg.SiteName
		}
		if cfg.HasSearch {
			searchBase := PageURL(cfg.Domain, "/api/search", cfg.DefaultIndex)
			website["potentialAction"] = map[string]any{
				"@type":       "SearchAction",
				"target":      searchBase + "?q={search_term_string}",
				"query-input": "required name=search_term_string",
			}
		}
		schemas = append(schemas, website)
	}

	data, err := json.Marshal(schemas)
	if err != nil {
		return ""
	}
	return string(data)
}
