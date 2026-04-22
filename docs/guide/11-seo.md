---
title: "SEO"
description: "Search engine optimization features including canonical URLs, sitemap, robots.txt, and Open Graph tags."
author: "nicolasm"
tags: ["seo", "configuration"]
---

# SEO

gomddoc includes built-in technical SEO features that help search engines discover, crawl, and
index your documentation correctly. All SEO features work in both `serve` mode (dynamic) and
`build` mode (static files), so your documentation is optimized regardless of how you deploy it.

Most SEO features activate automatically once you configure a domain for your site. Without a
domain, gomddoc still serves `robots.txt` and renders pages with proper HTML structure — but
canonical URLs, sitemap, and Open Graph tags require a domain to generate absolute URLs.

## Configuring the Domain

Set the `meta.domain` field in your `.gomddoc/config.yml`:

```yaml
meta:
  title: "My Project Docs"
  description: "Documentation for My Project"
  domain: "docs.example.com"
```

Or via environment variable:

```bash
export GOMDDOC_SITE_META_DOMAIN=docs.example.com
```

The domain may include a scheme (`https://docs.example.com`) or not — `https://` is assumed by
default. This single setting enables canonical URLs, sitemap generation, Open Graph tags, and the
`Sitemap:` directive in robots.txt.

## Canonical URLs

When a domain is configured, every page includes a `<link rel="canonical">` tag in the HTML `<head>`:

```html
<link rel="canonical" href="https://docs.example.com/guide/getting-started.md">
```

Canonical URLs tell search engines the authoritative URL for each page. This prevents duplicate
content issues when the same page is accessible via multiple URLs (for example, with and without
a trailing slash, or via different hostnames behind a load balancer).

Default index files (like `README.md`) are stripped from canonical URLs — so a page at
`/guide/README.md` gets a canonical URL of `https://docs.example.com/guide/` rather than including
the filename.

### Template Function

Custom themes can use the `canonicalURL` template function:

```html
{{ $canonical := canonicalURL .Page.Path }}
{{ if $canonical }}
    <link rel="canonical" href="{{ $canonical }}">
{{ end }}
```

This function returns an empty string when no domain is configured, so the tag is safely omitted.

## Sitemap

An XML sitemap is automatically generated at `/sitemap.xml` when a domain is configured. It follows
the [sitemaps.org](https://www.sitemaps.org/) specification and includes all indexed markdown pages
discovered by the metadata index:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://docs.example.com/</loc>
    <lastmod>2025-06-15</lastmod>
  </url>
  <url>
    <loc>https://docs.example.com/guide/getting-started.md</loc>
    <lastmod>2025-06-10</lastmod>
  </url>
</urlset>
```

Each entry includes a `<lastmod>` date derived from the file's modification time. For the
filesystem provider, this is the OS file mtime. For the git provider, it reflects the commit
timestamp. If a file cannot be stat'd, the `<lastmod>` is simply omitted for that entry.

In `serve` mode, the sitemap is generated dynamically on first request and cached. In `build`
mode, `sitemap.xml` is written as a static file in the output directory.

Submitting your sitemap to search engines (via Google Search Console or Bing Webmaster Tools) helps
them discover all your pages efficiently, especially for large documentation sites where not every
page is reachable through links from the homepage.

## Robots.txt

A `robots.txt` file is always served at `/robots.txt`, regardless of whether a domain is configured:

```
User-agent: *
Allow: /

Disallow: /_assets/
Disallow: /api/
Disallow: /debug/
```

The `Disallow` rules prevent search engines from indexing internal assets (theme CSS and JS), API
endpoints (search and tags), and debug endpoints (pprof). These are implementation details that
should not appear in search results.

When a domain is configured, a `Sitemap:` directive is appended:

```
Sitemap: https://docs.example.com/sitemap.xml
```

This tells search engine crawlers where to find your sitemap without requiring manual submission.

In `build` mode, `robots.txt` is always generated in the output directory.

## Open Graph Tags

When a domain is configured, pages include Open Graph and Twitter Card meta tags that control how
your documentation appears when shared on social media, in chat applications, and in link previews:

```html
<meta property="og:url" content="https://docs.example.com/guide.md">
<meta property="og:type" content="article">
<meta property="og:site_name" content="My Project Docs">
<meta property="og:title" content="Getting Started Guide">
<meta property="og:description" content="Learn how to set up and configure the project">

<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Getting Started Guide">
<meta name="twitter:description" content="Learn how to set up and configure the project">
```

These tags are rendered in the `<head>` of every page by the default theme's `head.html.tmpl`
partial, which is inherited by all eight built-in themes.

### Customizing Open Graph per Page

You can control Open Graph behavior through YAML frontmatter on individual pages:

- **`og_type`** — defaults to `article` for all pages. Override with `og_type: website` for landing
  pages or `og_type: profile` for author pages. See the [Open Graph specification](https://ogp.me/)
  for valid type values.
- **`title`** — used as `og:title`. If not set in frontmatter, the page has no explicit title in
  Open Graph tags.
- **`description`** — used as `og:description`. Falls back to the site-level `meta.description` from
  your config file if not set on the page.

Example frontmatter:

```yaml
---
title: "API Reference"
description: "Complete API documentation with examples"
og_type: website
---
```

## Structured Data (JSON-LD)

When a domain is configured, gomddoc injects [Schema.org](https://schema.org/) structured data
into every page as `<script type="application/ld+json">`. This helps search engines understand
your content and can enable rich results (enhanced search snippets).

Three schema types are generated:

### TechArticle (every page)

Every page gets a `TechArticle` schema with fields populated from frontmatter:

```json
{
  "@context": "https://schema.org",
  "@type": "TechArticle",
  "headline": "Setup Guide",
  "description": "How to install and configure the project",
  "author": {"@type": "Person", "name": "Alice"},
  "datePublished": "2025-06-15T00:00:00Z",
  "url": "https://docs.example.com/guide/setup.md"
}
```

Fields are omitted when not present in frontmatter — only `url` and `mainEntityOfPage` are always
included.

### BreadcrumbList (pages with navigation depth)

Pages with more than one breadcrumb get a `BreadcrumbList` schema that mirrors the visible
breadcrumb trail:

```json
{
  "@context": "https://schema.org",
  "@type": "BreadcrumbList",
  "itemListElement": [
    {"@type": "ListItem", "position": 1, "name": "Home", "item": "https://docs.example.com/"},
    {"@type": "ListItem", "position": 2, "name": "Guide", "item": "https://docs.example.com/guide/"}
  ]
}
```

### WebSite (index page only)

The site's index page (root or default index file) includes a `WebSite` schema. When search is
enabled, it includes a `SearchAction` that tells Google about your site's search endpoint:

```json
{
  "@context": "https://schema.org",
  "@type": "WebSite",
  "name": "My Project Docs",
  "url": "https://docs.example.com/",
  "potentialAction": {
    "@type": "SearchAction",
    "target": "https://docs.example.com/api/search?q={search_term_string}",
    "query-input": "required name=search_term_string"
  }
}
```

### Customizing JSON-LD

The JSON-LD output is rendered via an overridable template partial called `jsonld`. To customize
or disable JSON-LD, create a `.gomddoc/partials/jsonld.html.tmpl` file:

```html
{{/* Disable JSON-LD entirely */}}
{{ define "jsonld" }}{{ end }}
```

Or override with custom schemas:

```html
{{ define "jsonld" }}
<script type="application/ld+json">
{"@context": "https://schema.org", "@type": "Organization", "name": "My Company"}
</script>
{{ end }}
```

## Static Site Generation

All SEO features work seamlessly with `gomddoc build`:

```bash
gomddoc build ./docs -o ./public
```

The output includes:

- **`robots.txt`** — always generated in the output root
- **`sitemap.xml`** — generated when `meta.domain` is configured
- **Canonical URLs and Open Graph tags** — embedded in each HTML page's `<head>`

This means your static site has the same SEO capabilities as the live server, with no additional
build steps or plugins required.
