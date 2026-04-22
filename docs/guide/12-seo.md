---
title: "SEO"
description: "Search engine optimization features including canonical URLs, sitemap, robots.txt, and Open Graph tags."
author: "nicolasm"
---

# SEO

gomddoc includes built-in SEO features that activate when you configure a domain for your site.

## Configuring the Domain

Set the `meta.domain` field in `.gomddoc/config.yml`:

```yaml
meta:
  domain: "docs.example.com"
```

Or via environment variable:

```bash
export GOMDDOC_SITE_META_DOMAIN=docs.example.com
```

The domain may include a scheme (`https://docs.example.com`) or not — `https://` is assumed by default.

## Canonical URLs

When a domain is configured, every page includes a `<link rel="canonical">` tag in the `<head>`:

```html
<link rel="canonical" href="https://docs.example.com/guide/getting-started.md">
```

This tells search engines the authoritative URL for each page, preventing duplicate content issues.

### Template Functions

A template function is available for SEO:

- `{{ canonicalURL .Page.Path }}` — returns the full canonical URL for the page

Returns an empty string if no domain is configured.

## Sitemap

An XML sitemap is automatically generated at `/sitemap.xml` when a domain is configured. It follows the [sitemaps.org](https://www.sitemaps.org/) specification and includes all indexed markdown pages.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://docs.example.com/</loc>
  </url>
  <url>
    <loc>https://docs.example.com/guide/getting-started.md</loc>
  </url>
</urlset>
```

The sitemap is available in both `serve` mode (dynamic endpoint) and `build` mode (generated as `sitemap.xml` in the output directory).

## Robots.txt

A `robots.txt` file is always served at `/robots.txt`, regardless of domain configuration:

```
User-agent: *
Allow: /

Disallow: /_assets/
Disallow: /api/
Disallow: /debug/

Sitemap: https://docs.example.com/sitemap.xml
```

The `Sitemap:` directive is included only when a domain is configured. The `Disallow` rules prevent indexing of internal assets, API endpoints, and debug endpoints.

In `build` mode, `robots.txt` is generated in the output directory.

## Open Graph Tags

When a domain is configured, pages include Open Graph and Twitter Card meta tags:

```html
<meta property="og:url" content="https://docs.example.com/guide.md">
<meta property="og:type" content="article">
<meta property="og:site_name" content="My Documentation">
<meta property="og:title" content="Guide">
<meta property="og:description" content="Getting started with the project">

<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Guide">
<meta name="twitter:description" content="Getting started with the project">
```

### Customizing Open Graph

- **`og:type`** defaults to `article`. Override per-page via frontmatter: `og_type: website`
- **`og:title`** uses the page's frontmatter `title`
- **`og:description`** uses the page's frontmatter `description`, falling back to the site description

## Static Site Generation

All SEO features work with `gomddoc build`:

```bash
gomddoc build ./docs -o ./public
```

The output includes:
- `robots.txt` — always generated
- `sitemap.xml` — generated when domain is configured
- Canonical URLs and Open Graph tags in each HTML page's `<head>`
