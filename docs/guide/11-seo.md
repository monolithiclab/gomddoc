---
title: "SEO"
description: "Search engine optimization features including canonical URLs, sitemap, robots.txt, and Open Graph tags."
author: "nicolasm"
tags: ["seo", "configuration"]
---

# SEO

gomddoc includes built-in technical SEO features that help search engines discover, crawl, and
index your documentation. All SEO features work in both `serve` mode (generated on request) and
`build` mode (static files).

Most SEO features need absolute URLs, so they activate once you configure a domain for your site.
Without a domain, gomddoc still serves `robots.txt` and renders the description, Open Graph and
Twitter Card tags that need no URL. Canonical URLs, `og:url`, the sitemap, the Atom feed and JSON-LD
require a domain.

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

Or per run with `--domain` (`-d`) on `serve`, `preview` and `build`, which overrides the config file.

The domain is a bare host: `docs.example.com`, not `https://docs.example.com/`. A scheme or a path is
rejected at startup (`domain should not include protocol`), and `gomddoc doctor` reports it as
`config.invalid-value`. Generated URLs always use `https://`. This one setting enables canonical URLs,
`og:url`, the sitemap, the Atom feed, JSON-LD, and the `Sitemap:` directive in robots.txt.

## Canonical URLs

When a domain is configured, every page includes a `<link rel="canonical">` tag in the HTML `<head>`:

```html
<link rel="canonical" href="https://docs.example.com/guide/getting-started">
```

Canonical URLs tell search engines the authoritative URL for each page. This prevents duplicate
content issues when the same page is accessible via multiple URLs, such as different hostnames behind a
load balancer.

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: in `serve`, the canonical URL follows the request path, so `/guides` and
> `/guides/` give a directory index two different canonical URLs. `build` always emits one form.

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

An XML sitemap is generated at `/sitemap.xml` when a domain is configured. It follows
the [sitemaps.org](https://www.sitemaps.org/) specification and lists every markdown page in the
metadata index except those whose `robots` frontmatter contains `noindex`, followed by the `/tags/`
index and one entry per tag page (tag entries carry no `<lastmod>`):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://docs.example.com/</loc>
    <lastmod>2025-06-15</lastmod>
  </url>
  <url>
    <loc>https://docs.example.com/guide/getting-started</loc>
    <lastmod>2025-06-10</lastmod>
  </url>
</urlset>
```

Each page entry includes a `<lastmod>` date (`YYYY-MM-DD`) derived from the file's modification time. For the
filesystem provider, this is the OS file mtime. For the git provider, it reflects the commit
timestamp. If a file cannot be stat'd, `<lastmod>` falls back to the frontmatter `date`, and is
omitted only when the page has neither.

That fallback is one rule — `seo.LastModified` — shared with `feed.xml`'s `<updated>` and JSON-LD's
`dateModified`, so the three documents cannot describe the same page's freshness differently. It
also normalizes to UTC, which is why every generated timestamp ends in `Z` regardless of the
serving machine's time zone.

In `serve` mode, the sitemap is generated dynamically on first request and cached. In `build`
mode, `sitemap.xml` is written as a static file in the output directory.

Submitting your sitemap to search engines (via Google Search Console or Bing Webmaster Tools) helps
them discover all your pages efficiently, especially for large documentation sites where not every
page is reachable through links from the homepage.

### Multi-Language Sitemaps

When multiple languages are detected, each language gets its own sitemap:

- `/sitemap.xml` — default language pages
- `/fr-FR/sitemap.xml` — French pages
- `/es-ES/sitemap.xml` — Spanish pages

**In `build` mode only**, a `sitemap-index.xml` referencing all per-language sitemaps is written to
the output root. `serve` registers no route for it, so on a live server that URL is a 404: the
per-language sitemaps above are served, but nothing indexes them. Submit them individually, or serve
the built site from a static host:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://docs.example.com/sitemap.xml</loc>
  </sitemap>
  <sitemap>
    <loc>https://docs.example.com/fr-FR/sitemap.xml</loc>
  </sitemap>
</sitemapindex>
```

Submit the sitemap index URL to search engines — they will discover all per-language sitemaps from it.

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

When a domain is configured, a blank line and a `Sitemap:` directive are appended:

```
Sitemap: https://docs.example.com/sitemap.xml
```

This tells search engine crawlers where to find your sitemap without requiring manual submission.

In `build` mode, `robots.txt` is always generated in the output directory. When the build detects
translation directories it also writes a `sitemap-index.xml`, and the directive names that instead:

```
Sitemap: https://docs.example.com/sitemap-index.xml
```

The directive always names a sitemap that was actually published: build derives it from writing the
index, and `serve` from registering the route. Naming `/sitemap.xml` when an index exists would send
crawlers to the default-language sitemap, which by design lists no translated page. `serve` writes no
index — it has no build step to write one during — so its directive names `/sitemap.xml`, and omits
the `Sitemap:` line entirely when no sitemap is served at all.

## Atom Feed

gomddoc generates an Atom 1.0 XML feed at `/feed.xml` when a domain is configured. The feed includes the 20 most
recently modified pages, newest first, dated by the same `seo.LastModified` rule as the sitemap. Pages whose `robots`
frontmatter contains `noindex` are excluded.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>My Project Docs</title>
  <id>https://docs.example.com/</id>
  <updated>2025-06-15T10:30:00Z</updated>
  <link rel="self" href="https://docs.example.com/feed.xml" type="application/atom+xml"></link>
  <link rel="alternate" href="https://docs.example.com/"></link>
  <entry>
    <title>Setup Guide</title>
    <id>https://docs.example.com/guide/setup</id>
    <updated>2025-06-15T10:30:00Z</updated>
    <link rel="alternate" href="https://docs.example.com/guide/setup"></link>
    <summary>How to install and configure the project</summary>
  </entry>
</feed>
```

Each entry includes the page title, URL (also used as its `<id>`), modification time, and description (from
frontmatter). The feed title is `meta.title`. In `serve`, the feed is generated on first request and cached for the
lifetime of the process.

### Multi-Language Feeds

When multiple languages are detected, each language gets its own feed:

- `/feed.xml` — default language
- `/fr-FR/feed.xml` — French content
- `/es-ES/feed.xml` — Spanish content

When a domain is configured, the default theme's `<head>` includes
`<link rel="alternate" type="application/atom+xml">` pointing at `/feed.xml`, so feed readers and browsers can
discover the feed. Translated pages point at the same root feed, not their language's feed.

## hreflang Tags

When multiple languages are detected, gomddoc injects `<link rel="alternate" hreflang="...">` tags in the `<head>` of
every page. These tags tell search engines which language variants exist for each page:

```html
<link rel="alternate" hreflang="en-US" href="/guide/setup">
<link rel="alternate" hreflang="x-default" href="/guide/setup">
<link rel="alternate" hreflang="fr-FR" href="/fr-FR/guide/setup">
```

The default language pages get an unprefixed URL and the `x-default` hreflang value (which tells search engines to use
this variant as the fallback for unsupported languages). Non-default languages get prefixed URLs. The `href` values
are root-relative paths, not absolute URLs, and a tag is emitted for every detected language whether or not that
page has a translation.

hreflang tags prevent search engines from treating translated pages as duplicate content and enable them to serve the
correct language variant in search results based on the user's locale.

See [Internationalization](13-internationalization.md) for the full multi-language setup guide.

## Open Graph Tags

Pages include Open Graph and Twitter Card meta tags that control how your documentation appears when
shared on social media, in chat applications, and in link previews. `og:url` requires a domain; the
others are emitted without one, each only when its value is set:

```html
<meta property="og:url" content="https://docs.example.com/guide">
<meta property="og:type" content="article">
<meta property="og:site_name" content="My Project Docs">
<meta property="og:title" content="Getting Started Guide">
<meta property="og:description" content="Learn how to set up and configure the project">

<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Getting Started Guide">
<meta name="twitter:description" content="Learn how to set up and configure the project">

<meta name="description" content="Learn how to set up and configure the project">
```

These tags are rendered by the `head-meta` template in the default theme's `partials/head-shared.html.tmpl`, which
each theme's `head` partial calls. The same template emits the canonical link, the feed link, `rel="prev"` and
`rel="next"` links (with a domain), and the hreflang tags. A page's `<meta name="robots">` comes from its `robots`
frontmatter, or from the site-level `meta.robots` when the page sets none.

### Customizing Open Graph per Page

You can control Open Graph behavior through YAML frontmatter on individual pages:

- **`og_type`** — defaults to `article` for all pages. Override with `og_type: website` for landing
  pages or `og_type: profile` for author pages. See the [Open Graph specification](https://ogp.me/)
  for valid type values.
- **`title`** — used as `og:title`. If not set in frontmatter, the page has no explicit title in
  Open Graph tags.
- **`description`** — used as `og:description`, `twitter:description` **and** the plain
  `<meta name="description">` that search engines read for the result snippet. All three come from
  one value, falling back to the site-level `meta.description` from your config file when the page
  does not set one. If neither is set, no description tag is emitted — an empty `content=""` is
  worse than no tag.

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
into every page as `<script type="application/ld+json">`, holding a JSON array of schemas. This
helps search engines understand your content and can enable rich results (enhanced search snippets).
Without a domain, no JSON-LD is emitted.

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
  "dateModified": "2025-09-02T11:20:00Z",
  "url": "https://docs.example.com/guide/setup"
}
```

`headline`, `description` and `author` come from the frontmatter `title`, `description` and `author`, and are
omitted when not set. Only `url` and `mainEntityOfPage` are always included.

The two dates come from different places:

| Field           | Source                                                                        |
| --------------- | ----------------------------------------------------------------------------- |
| `datePublished` | Frontmatter `date`. A bare `2025-06-15` and a quoted RFC 3339 timestamp are both accepted. |
| `dateModified`  | The source file's modification time, falling back to `date` when unavailable — `seo.LastModified`, the same rule `sitemap.xml`'s `<lastmod>` and `feed.xml`'s `<updated>` use. |

With the Git provider, file modification times are the commit timestamp of the revision being
served, so `dateModified` is per-repository rather than per-page. Set frontmatter `date` on pages
where that distinction matters.

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

The site's index page (root or default index file) includes a `WebSite` schema. When the search
index was built (`serve` and `preview` with `search.index: true`), it includes a `SearchAction` pointing at the
JSON search endpoint. `build` has no search index, so built pages omit it:

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
or disable JSON-LD, create a `.gomddoc/partials/jsonld.html.tmpl` file. Site-level partials in
`.gomddoc/partials/` override the theme's partials of the same name:

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

## Clean URLs and SEO

gomddoc serves extensionless (clean) URLs by default, which has several SEO benefits:

- **301 redirects preserve link equity.** Requests to `/guide/setup.md` receive a `301 Moved
  Permanently` redirect to `/guide/setup`. The permanent redirect tells search engines to transfer
  all ranking signals from the old URL to the canonical extensionless form.
- **Extensionless URLs are the canonical form.** Canonical `<link>` tags, Open Graph URLs, and
  structured data URLs all use the clean path. Search engines index the extensionless version,
  avoiding duplicate content issues between `/page.md` and `/page`.
- **Sitemap and feed URLs use clean paths**, with default index files folded into their directory
  (`/guide/README.md` is listed as `/guide/`).
- **Migration-safe.** If you are migrating an existing site that previously used `.md` URLs, all
  old extension-based links (from external sites, bookmarks, or cached search results) are properly
  redirected. No manual redirect rules are needed.

For details on how URL resolution works, see
[HTTP Behavior](12-advanced/01-http-behavior.md#url-resolution).

## Static Site Generation

The same SEO output is written by `gomddoc build`:

```bash
gomddoc build ./docs -o ./public -d docs.example.com
```

The output includes:

- **`robots.txt`**: always generated in the output root
- **`sitemap.xml`**: generated when `meta.domain` is configured (per-language in multi-language sites)
- **`sitemap-index.xml`**: generated when `meta.domain` is configured and translation directories are detected
- **`feed.xml`**: Atom 1.0 feed, generated when `meta.domain` is configured (per-language in multi-language sites)
- **Canonical URLs, Open Graph tags, JSON-LD and hreflang tags**: embedded in each HTML page's `<head>`

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: on translated pages, the canonical URL, `og:url` and JSON-LD `url` name the
> default-language path (`https://docs.example.com/guide/setup` for `fr-FR/guide/setup.md`) in both `serve` and
> `build`, which tells search engines the translation is a duplicate of the default-language page.
