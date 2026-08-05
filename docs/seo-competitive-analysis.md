---
title: "SEO Competitive Analysis"
description: "Technical SEO capabilities across CMS and documentation tools, with feature recommendations for gomddoc."
author: "research"
---

# SEO Competitive Analysis

> **Historical research, not a gap analysis.** This document was written before Phase 9 to decide
> what to build. Most of it has since shipped, so every "gomddoc is missing X" statement below
> describes the state at the time of writing, not today. The competitor survey and the rationale for
> each recommendation are still accurate and still worth reading; the verdicts are not. See
> [Implementation Status](#implementation-status) for what actually landed, and
> [roadmap.md](roadmap.md) Phase 9 for the authoritative record.

## Implementation Status

| #  | Recommendation                     | Status | Where                                                     |
| -- | ---------------------------------- | ------ | --------------------------------------------------------- |
| 1  | Canonical URLs                     | ✅     | `internal/seo`, `canonicalURL` template func               |
| 2  | XML sitemap                        | ✅     | `internal/server/sitemap.go`, `build.go` (+ sitemap index) |
| 3  | robots.txt                         | ✅     | `GET /robots.txt`, `generateSEOFiles`                      |
| 4  | Open Graph / Twitter Card          | ✅     | `partials/head-shared.html.tmpl`                           |
| 5  | JSON-LD structured data            | ✅     | `seo.GenerateJSONLD`, `partials/jsonld.html.tmpl`          |
| 6  | Auto-generated meta description    | ❌     | frontmatter `description` or the site default only         |
| 7  | Git-based timestamps               | 🟡     | sitemap `<lastmod>` and the feed use ModTime; JSON-LD has no `datePublished`/`dateModified` |
| 8  | Social preview image generation    | ❌     | no `og:image`; deliberately deferred (needs image rendering) |
| 9  | Per-page `<meta name="robots">`    | ✅     | frontmatter `robots`, site default `meta.robots`           |
| 10 | Heading anchor slug stability      | 🟡     | goldmark `WithAutoHeadingID` is stable, but the algorithm is neither documented nor pinned by a test |
| 11 | `<html lang>`                      | ✅     | `layouts/default.html.tmpl`, `site.language`               |
| 12 | Related pages via tags             | ✅     | `findRelatedDocs`, `PageContext.RelatedDocs`               |
| 13 | 404 page with navigation           | ✅     | error layout in serve, `404.html` in build                 |
| 14 | Redirect support                   | ✅     | frontmatter `redirect_from`, `URLRedirectMap`              |
| 15 | RSS/Atom feed                      | ✅     | `internal/server/feed.go`, `GET /feed.xml`                 |
| 16 | Preconnect/preload resource hints  | ✅     | `partials/head.html.tmpl`                                  |
| 17 | Image dimension attributes         | ❌     | images pass through unchanged                              |
| 18 | `<link rel="next/prev">`           | ✅     | `partials/head-shared.html.tmpl`                           |

One item from [Features to Skip](#features-to-skip) shipped anyway: **hreflang**, because i18n landed
(`partials/hreflang.html.tmpl`). The rest of that section still stands.

## Executive Summary

This analysis examines the technical SEO mechanisms used by WordPress (with Yoast/RankMath), major
documentation generators (MkDocs Material, Docusaurus, Hugo, VitePress, GitBook), and modern static
site frameworks (Astro, Gatsby, Next.js). The goal is to identify which SEO features gomddoc should
implement to compete effectively for search rankings in the documentation space.

Key findings:

1. **The basics matter most.** Canonical URLs, meta description, Open Graph tags, and XML sitemaps
   account for the largest SEO impact. gomddoc has partial coverage (description meta tag exists,
   sitemap is planned) but is missing canonical URLs, Open Graph, and robots.txt.
   _(All four have since shipped — see [Implementation Status](#implementation-status).)_
2. **Structured data is a differentiator.** JSON-LD markup (Article, TechArticle, BreadcrumbList)
   enables rich snippets in search results. Documentation tools rarely implement this, creating an
   opportunity.
3. **Performance is SEO.** Google's Core Web Vitals are a ranking signal. gomddoc's server-rendered
   HTML with no client-side framework is a natural advantage. The main gaps are image optimization
   and resource hints (preconnect, preload).
4. **Documentation-specific patterns** (clean heading hierarchy, internal cross-references, code
   snippet indexing) are where gomddoc can differentiate from general-purpose CMS tools.

gomddoc's current state: `<meta name="description">` is present, `<title>` uses page + site title,
`base_url`/`domain` config exists. Everything else in this document is net-new.
_(As of Phase 9, 14 of the 18 recommendations below are implemented.)_

## WordPress SEO Deep Dive

### Core Features

WordPress ships several SEO-relevant features in core (no plugins):

- **Permalink structure**: Configurable URL patterns (`/posts/%postname%/`, date-based, numeric).
  Pretty permalinks eliminate query strings. WordPress rewrites URLs via `.htaccess` or Nginx rules.
- **XML sitemaps** (since WP 5.5): Auto-generated at `/wp-sitemap.xml` with sub-sitemaps for posts,
  pages, taxonomies, and authors. Includes `<lastmod>` timestamps. Respects noindex settings.
- **`robots.txt`**: Virtual file generated dynamically. Blocks `wp-admin`, points to sitemap URL.
  Plugins can append rules.
- **Canonical URLs**: `<link rel="canonical">` on every page. Handles pagination (`?page=2`),
  trailing slashes, and HTTP-to-HTTPS. Prevents duplicate content penalties.
- **Title tag**: `<title>` with customizable separator and site name. `wp_title()` filter chain.
- **Excerpt/description**: Manual excerpt or auto-generated from first 55 words.
- **RSS/Atom feeds**: Auto-discovery via `<link rel="alternate" type="application/rss+xml">`.
  Search engines use feeds for faster content discovery.
- **Image alt text**: Media library enforces alt text. Missing alt generates admin warnings.
- **Lazy loading**: Native `loading="lazy"` on images and iframes (since WP 5.5). `fetchpriority="high"`
  on above-the-fold images (since WP 6.3).
- **Responsive images**: `srcset` and `sizes` attributes auto-generated from uploaded image sizes.

### Yoast SEO / RankMath

These plugins dominate WordPress SEO. Their technical features (not content analysis):

**Meta tag management:**
- Per-page `<meta name="description">` with variable substitution (`%%title%%`, `%%sep%%`, `%%sitename%%`)
- `<meta name="robots">` per page: `noindex`, `nofollow`, `noarchive`, `nosnippet`, `max-snippet:-1`
- Canonical URL override (defaults to permalink, editable per page)
- `<meta name="author">` from post author

**Open Graph / Twitter Cards:**
- `<meta property="og:title">`, `og:description`, `og:image`, `og:url`, `og:type`, `og:site_name`
- `og:image` with explicit `og:image:width` and `og:image:height` dimensions
- `<meta name="twitter:card">` (summary_large_image), `twitter:title`, `twitter:description`, `twitter:image`
- Fallback chain: custom OG image > featured image > first image in content > site default
- Per-page override of all OG/Twitter fields

**Structured data (JSON-LD):**
- Organization/Person schema on homepage
- Article/BlogPosting schema on posts (headline, datePublished, dateModified, author, publisher, image)
- BreadcrumbList schema matching the breadcrumb navigation
- WebSite schema with SearchAction (enables sitelinks search box in Google)
- FAQ schema for FAQ-formatted content
- HowTo schema for step-by-step content
- WebPage schema with `mainEntity` references
- Automatic `@graph` construction connecting all schema entities

**Technical SEO:**
- XML sitemap with image entries, news entries, and video entries
- Redirect manager (301/302/307/410 with regex support)
- `robots.txt` editor with per-search-engine rules
- Internal linking suggestions based on cornerstone content
- Breadcrumb markup (both HTML + JSON-LD)
- RSS feed modifications (add canonical link, author attribution)

### Structured Data

WordPress structured data (via plugins) is the most sophisticated of any CMS:

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "WebSite",
      "url": "https://example.com/",
      "name": "Site Name",
      "potentialAction": {
        "@type": "SearchAction",
        "target": "https://example.com/?s={search_term_string}",
        "query-input": "required name=search_term_string"
      }
    },
    {
      "@type": "TechArticle",
      "headline": "Page Title",
      "datePublished": "2025-01-15T10:00:00+00:00",
      "dateModified": "2025-03-01T14:30:00+00:00",
      "author": { "@type": "Person", "name": "Author Name" },
      "description": "Page description",
      "mainEntityOfPage": { "@id": "https://example.com/docs/page/" }
    },
    {
      "@type": "BreadcrumbList",
      "itemListElement": [
        { "@type": "ListItem", "position": 1, "name": "Home", "item": "https://example.com/" },
        { "@type": "ListItem", "position": 2, "name": "Docs", "item": "https://example.com/docs/" },
        { "@type": "ListItem", "position": 3, "name": "Page Title" }
      ]
    }
  ]
}
</script>
```

### Performance and Core Web Vitals

Google uses Core Web Vitals (LCP, FID/INP, CLS) as ranking signals. WordPress addresses this through:

- **Lazy loading**: `loading="lazy"` on below-fold images/iframes. `fetchpriority="high"` on hero images.
- **Image optimization**: WebP/AVIF conversion, responsive `srcset`, explicit `width`/`height` to prevent CLS.
- **Script deferral**: `defer`/`async` on non-critical scripts. `wp_enqueue_script` with `strategy` parameter.
- **Preconnect/preload**: `<link rel="preconnect">` for Google Fonts, CDNs. `<link rel="preload">` for critical CSS.
- **Render-blocking removal**: Move scripts to footer, inline critical CSS.
- **Speculative prerendering**: `<script type="speculationrules">` for instant page transitions (WP 6.4+).

### Key Takeaways for gomddoc

1. Canonical URLs are table stakes -- every page needs `<link rel="canonical">`.
2. Open Graph tags are essential for social sharing and modern search engines.
3. JSON-LD structured data (especially BreadcrumbList and TechArticle) enables rich snippets.
4. `robots.txt` and XML sitemap are the minimum for crawl management.
5. WordPress's complexity (plugin ecosystem, database queries) is a liability for performance;
   gomddoc's server-rendered static HTML is a natural advantage for Core Web Vitals.

## Documentation Tool Landscape

### MkDocs Material

The gold standard for documentation SEO among static site generators.

**Built-in SEO features:**
- **`meta` plugin**: Extracts frontmatter `description` and `title` for `<meta>` tags. Supports
  `meta` key in frontmatter for arbitrary `<meta>` tags (e.g., `robots: noindex`).
- **Social cards**: Auto-generates `og:image` social preview cards as PNG images from page title
  and description. Uses site colors and fonts. No manual image creation needed.
- **`<link rel="canonical">`**: Auto-generated from `site_url` + page path.
- **XML sitemap**: Generated via `sitemap.xml` plugin (enabled by default). Respects `sitemap_exclude`
  frontmatter to exclude pages.
- **`robots.txt`**: Not generated by default; users add it to `docs/` directory.
- **Open Graph**: `og:title`, `og:description`, `og:url`, `og:image`, `og:type` auto-generated.
  Twitter card tags included.
- **Structured data**: No built-in JSON-LD. Some community plugins exist.
- **Clean URLs**: `use_directory_urls: true` produces `/page/index.html` served as `/page/`.
- **Navigation tabs**: Horizontal top-level navigation improves internal linking.
- **Search**: Lunr.js-based full-text search. Generates `search_index.json` at build time.
- **Heading anchors**: Auto-generated IDs with configurable slug function.
- **Performance**: Static HTML, no client-side framework. CSS/JS is minimal. Instant loading via
  `instant` navigation (intercepts clicks, fetches via XHR, swaps content).

**What makes it rank well:**
- Clean semantic HTML output
- Excellent mobile responsiveness
- Fast load times (static HTML, minimal JS)
- Auto-generated social preview images (increases CTR from social/search)

### Docusaurus

Facebook's documentation framework (React-based SSG).

**SEO features:**
- **`@docusaurus/plugin-sitemap`**: Generates `sitemap.xml` with configurable `changefreq` and
  `priority`. Supports `lastmod` from Git commit dates.
- **Meta tags**: `<meta name="description">` from frontmatter `description` or first paragraph.
  `<meta name="keywords">` from frontmatter `keywords` array.
- **Canonical URLs**: Auto-generated `<link rel="canonical">` from `url` + `baseUrl` config.
- **Open Graph**: Full OG tag set (`og:title`, `og:description`, `og:url`, `og:image`) auto-generated.
  Twitter card tags included. Customizable per page via frontmatter.
- **Structured data**: No built-in JSON-LD (open issue). Community plugins available.
- **robots.txt**: Not auto-generated; users create `static/robots.txt`.
- **SEO component**: `<Seo>` React component that manages `<head>` tags per page. Prevents duplicate
  meta tags.
- **`noindex` support**: `frontMatter.draft: true` adds `<meta name="robots" content="noindex">`.
- **Clean URLs**: Trailing slash configurable. `/docs/page` vs `/docs/page/`.
- **Versioned docs**: `/docs/1.0/page` alongside `/docs/2.0/page` with canonical pointing to latest.
- **Search**: Algolia DocSearch integration (free for open source). Generates crawlable content.
- **Last updated**: Shows "Last updated on" from Git dates. Visible to users, not in structured data.

**Performance concerns:**
- React hydration adds JS bundle weight (~200-400KB gzipped).
- Client-side routing improves perceived performance but increases initial load.
- React-based rendering can hurt LCP compared to pure static HTML.

### Hugo

The fastest static site generator. Minimal built-in SEO, relies on themes and templates.

**Built-in SEO features:**
- **Internal templates**: `_internal/opengraph.html`, `_internal/twitter_cards.html`,
  `_internal/schema.html`, `_internal/google_analytics.html`. Called via `{{ template "_internal/opengraph.html" . }}`.
- **Sitemap**: Built-in `sitemap.xml` generation with configurable `changefreq`, `priority`, and
  `filename`. Template-based (fully customizable).
- **robots.txt**: Template-based generation. `enableRobotsTXT = true` in config.
- **Canonical URLs**: Available via `.Permalink` in templates. Theme-dependent.
- **Taxonomies**: Tags, categories, series. Each generates a listing page (good for SEO).
- **`.GitInfo`**: Exposes Git author date and commit hash per page for `dateModified`.
- **Related content**: Content-based recommendations using keyword/tag/date matching.
- **Aliases**: Generates redirect pages (`<meta http-equiv="refresh">`). Not 301s.
- **`_index.md`**: Section pages with custom content (avoids thin content on listing pages).
- **Output formats**: Same content can produce HTML, JSON, RSS, AMP, etc.

**SEO is theme-dependent:**
Hugo's SEO story depends entirely on the theme. Popular themes (e.g., Hugo Book, Docsy, PaperMod)
include OG tags, JSON-LD, and sitemaps. Base Hugo provides the building blocks but not the assembly.

**Performance:**
- Fastest build times of any SSG (sub-second for thousands of pages).
- Pure static HTML output. Zero client-side JS unless theme adds it.
- Image processing pipeline (resize, crop, WebP conversion) at build time.

### VitePress

Vue-powered SSG for documentation. Successor to VuePress.

**SEO features:**
- **`<head>` injection**: Config-level `head` array for adding arbitrary `<meta>`, `<link>`, `<script>` tags.
- **Frontmatter `head`**: Per-page head tag injection.
- **Sitemap**: Built-in `sitemap` config option (since v1.3). Generates `sitemap.xml` from `hostname` + routes.
- **`titleTemplate`**: Configurable `<title>` pattern (e.g., `:title | Site Name`).
- **`description`**: Global and per-page meta descriptions.
- **Open Graph**: Not auto-generated. Must be configured manually via `head` injection or
  `transformHead` build hook.
- **Canonical URLs**: Not auto-generated. Manual via `head` config.
- **Clean URLs**: `cleanUrls: true` removes `.html` extensions.
- **Search**: MiniSearch-based local search. Algolia integration available.
- **Dead link detection**: Build-time validation of internal links (prevents 404s).
- **Last updated**: Git-based timestamp shown per page.

**Performance:**
- Vue 3 hydration is lighter than React but still adds JS weight.
- Vite-based dev server and build. Tree-shaking removes unused framework code.
- Static HTML with selective hydration (islands-like approach for interactive elements).

### GitBook

Commercial documentation platform (SaaS + open source renderer).

**SEO features:**
- **Automatic meta tags**: Title, description, OG tags derived from content.
- **Canonical URLs**: Auto-generated for all pages.
- **Sitemap**: Auto-generated for published spaces.
- **Custom domain**: CNAME support with automatic SSL.
- **`robots.txt`**: Configurable per space (published vs. private).
- **Search engine indexing**: Toggle per space (published spaces are indexed by default).
- **Clean URLs**: Path-based routing from content structure.
- **Crawler-friendly**: Server-side rendered HTML (not SPA for crawlers).

**Limitations:**
- No JSON-LD structured data.
- No OG image customization (uses generic GitBook branding).
- Limited performance control (SaaS, can't optimize server config).
- No direct access to HTML output for SEO customization.

### Others (Astro, Gatsby, Next.js)

**Astro:**
- Zero-JS output by default (islands architecture). Excellent Core Web Vitals.
- `@astrojs/sitemap` integration for automatic sitemap generation.
- `<SEO>` component pattern in Astro themes. Full control over `<head>`.
- View Transitions API for smooth page transitions.
- Image optimization via `<Image>` component (lazy loading, srcset, AVIF/WebP).
- Built-in RSS feed generation.

**Gatsby:**
- `gatsby-plugin-sitemap`: Auto-generates sitemap from page queries.
- `gatsby-plugin-react-helmet`: Per-page `<head>` management.
- GraphQL-based data layer pulls frontmatter into meta tags.
- Image optimization via `gatsby-plugin-image` (blur-up, traced SVG, responsive).
- Pathprefix support for subdirectory hosting.
- Structured data via `gatsby-plugin-schema-org` (community).
- Heavy JS bundle (React + GraphQL runtime) hurts Core Web Vitals.

**Next.js (for docs):**
- `next/head` for per-page meta tags. `metadata` API in App Router.
- `next-sitemap` package for sitemap generation.
- `next/image` for automatic image optimization (WebP, lazy loading, blur placeholder).
- ISR (Incremental Static Regeneration) for near-real-time content updates.
- Automatic `<link rel="canonical">` via `metadata.alternates.canonical`.
- JSON-LD via `metadata` API or `next-seo` package.
- Middleware for redirects and rewrites (301/302 at edge).

## Documentation-Specific SEO Patterns

What makes documentation sites rank well, distinct from blogs or marketing sites:

### 1. Heading Hierarchy and Content Structure

Search engines weight heading hierarchy heavily for documentation queries. Best practices:
- Single `<h1>` per page matching the page's primary topic
- Logical `<h2>`/`<h3>` nesting that mirrors the conceptual structure
- Headings that match search queries (e.g., "How to install" rather than "Getting Started")
- gomddoc already produces clean heading hierarchy from Markdown

### 2. Code Snippet Indexing

Google indexes code snippets and can surface them in search results:
- `<pre><code>` blocks with language class (`language-go`, `language-yaml`)
- Syntax-highlighted code is more crawlable than screenshots
- gomddoc already does this well via goldmark + Chroma highlighting

### 3. Breadcrumb Navigation

Both visual breadcrumbs and BreadcrumbList JSON-LD markup:
- Appears as rich snippet path in search results (e.g., "Docs > Guide > Installation")
- Improves CTR by showing content hierarchy in SERPs
- gomddoc has breadcrumbs in templates but no JSON-LD markup

### 4. Internal Cross-References

Dense internal linking within documentation:
- Table of contents linking to page sections
- "See also" / "Related" sections linking to other pages
- API reference cross-links between types, methods, endpoints
- Navigation sidebars providing site-wide internal links
- gomddoc has TOC and navigation; could add related pages via tags

### 5. Clean URL Structure

Documentation URLs should be human-readable and stable:
- `/docs/installation/` not `/docs/page?id=123`
- No file extensions (`.html`, `.md`) in URLs
- Hierarchical paths matching content organization
- gomddoc produces clean URLs; `index.html` generation supports this

### 6. Canonical URLs for Versioned Docs

When docs exist in multiple versions:
- `<link rel="canonical">` points to latest version
- `<meta name="robots" content="noindex">` on old versions (optional)
- Prevents duplicate content across versions
- Not applicable to gomddoc yet (no versioning), but relevant for Phase 9/10

### 7. Last Modified Dates

`dateModified` signals help search engines prioritize fresh content:
- Git commit dates are the ideal source for documentation
- Displayed to users and included in structured data
- Included in sitemap `<lastmod>`
- gomddoc can extract this from Git provider but doesn't yet

### 8. Long-Tail Keyword Coverage

Documentation naturally targets long-tail queries:
- Each page covers a specific topic ("how to configure X", "error Y troubleshooting")
- FAQ sections generate question-based queries
- API reference pages target method/function names
- This is a content strategy concern, not a technical feature

### 9. Mobile Responsiveness

Google uses mobile-first indexing:
- All documentation must render well on mobile
- Touch-friendly navigation (hamburger menus, collapsible sidebars)
- gomddoc themes already handle this well

### 10. Page Load Performance

Documentation sites with minimal JS consistently outrank framework-heavy alternatives:
- Server-rendered HTML without hydration overhead
- Minimal CSS (inline critical, defer rest)
- No layout shift from late-loading elements
- gomddoc's architecture is ideal here

## Recommended Features for gomddoc

### P0: Critical (Ship Before Public Launch)

#### 1. Canonical URLs

- **What it does:** Adds `<link rel="canonical" href="...">` to every page's `<head>`.
  Constructed from `domain` config + page path.
- **Why it matters:** Prevents duplicate content penalties when the same content is accessible
  via multiple URLs (with/without trailing slash, `README.md` vs `index.html`, HTTP vs HTTPS).
  Every major CMS and SSG does this. Its absence is a red flag to search engines.
- **Implementation complexity:** Low. Compose `domain` + page path in template context. Add to
  all theme templates.
- **Requires:** `domain` config already exists in `MetaConfig`. Need to expose a `canonicalURL`
  value in `TemplateContext` or as a template function.

#### 2. XML Sitemap Generation

- **What it does:** Generates `sitemap.xml` with `<url>`, `<loc>`, and `<lastmod>` for every
  rendered page. In `build` mode, writes to output directory. In `serve` mode, generates on
  request at `/sitemap.xml`.
- **Why it matters:** Primary mechanism for search engines to discover all pages. Required for
  large sites. `<lastmod>` from Git dates helps prioritize re-crawling.
- **Implementation complexity:** Medium. Walk content tree, collect URLs and dates, render XML.
  Git provider can supply commit dates; filesystem provider uses mtime.
- **Already planned** in Phase 7 roadmap.

#### 3. robots.txt

- **What it does:** Serves `/robots.txt` with `Sitemap:` directive and sensible defaults
  (allow all, block `/_assets/`, `/api/`, `/.well-known/`).
- **Why it matters:** Without `robots.txt`, crawlers have no guidance. The `Sitemap:` directive
  is how search engines discover the sitemap URL. Every production site needs one.
- **Implementation complexity:** Low. Static handler with template substitution for sitemap URL.
  In `build` mode, write file. In `serve` mode, serve from handler.

#### 4. Open Graph Meta Tags

- **What it does:** Adds `og:title`, `og:description`, `og:url`, `og:type`, `og:site_name` to
  every page. Optionally `og:image` if configured. Adds corresponding `twitter:card` tags.
- **Why it matters:** Controls how pages appear when shared on social media, Slack, Discord,
  and messaging apps. Also used by search engines for rich results. MkDocs Material, Docusaurus,
  and Hugo all do this automatically.
- **Implementation complexity:** Low. All data is already available (title, description, URL).
  Add meta tags to theme `<head>` section. Image is optional (see P1 social images).
- **Data sources:** `og:title` from page frontmatter `title` or `<h1>`, `og:description` from
  frontmatter `description` or auto-extracted first paragraph, `og:url` from canonical URL,
  `og:type` = "article", `og:site_name` from `Meta.Title`.

### P1: High Priority (Soon After Launch)

#### 5. JSON-LD Structured Data

- **What it does:** Injects `<script type="application/ld+json">` with Schema.org markup:
  - `TechArticle` or `Article` schema per page (headline, datePublished, dateModified, author,
    description)
  - `BreadcrumbList` schema matching the breadcrumb navigation
  - `WebSite` schema on the index page (with `SearchAction` if search is implemented)
- **Why it matters:** Enables rich snippets in Google search results: breadcrumb paths, article
  dates, author info. Documentation sites rarely implement this, so it's a competitive advantage.
  Google explicitly recommends JSON-LD over microdata/RDFa.
- **Implementation complexity:** Medium. Build JSON-LD from existing template context data
  (breadcrumbs, frontmatter, page path). Add as template function or automatic injection.
- **Data sources:** Breadcrumbs already exist. Frontmatter provides `title`, `description`,
  `author`. Git dates provide `datePublished`/`dateModified`. `domain` provides the base URL.

#### 6. Auto-Generated Meta Description

- **What it does:** When frontmatter `description` is missing, auto-extracts the first ~160
  characters of rendered text content (stripping Markdown/HTML) as the meta description.
- **Why it matters:** Pages without descriptions get search engine-generated snippets, which
  are often poor. Auto-extraction ensures every page has a reasonable description without
  requiring authors to write frontmatter.
- **Implementation complexity:** Low. Strip HTML tags from rendered content, truncate at word
  boundary, use as fallback when `description` frontmatter is absent.

#### 7. Git-Based Timestamps

- **What it does:** Exposes `datePublished` (first commit date) and `dateModified` (last commit
  date) for each page. Used in JSON-LD, sitemap `<lastmod>`, and optionally displayed in the UI
  ("Last updated on...").
- **Why it matters:** `dateModified` is a ranking signal. Sitemap `<lastmod>` controls crawl
  priority. Visible "last updated" dates build reader trust. MkDocs Material and Docusaurus
  both extract dates from Git.
- **Implementation complexity:** Medium. Git provider can run `git log` per file. Filesystem
  provider falls back to mtime. Cache results at startup (or lazily) to avoid per-request
  Git operations.

#### 8. Social Preview Image Generation

- **What it does:** Auto-generates OG images (1200x630 PNG) for each page using the page title,
  description, and site branding. Used as `og:image` value.
- **Why it matters:** Pages with custom OG images get 2-3x higher click-through rates on social
  media and in search results. MkDocs Material's social cards plugin is one of its most popular
  features. Without an image, shares look plain and unprofessional.
- **Implementation complexity:** High. Requires image generation in Go (e.g., `fogleman/gg` or
  `golang.org/x/image/draw`). Must handle text layout, colors from theme, caching. Build-time
  only (too expensive for serve mode).

### P2: Medium Priority (Enhances Competitiveness)

#### 9. Configurable `<meta name="robots">` Per Page

- **What it does:** Allows frontmatter `robots: noindex` or `robots: noindex, nofollow` to
  control per-page indexing. Also supports site-wide default via config.
- **Why it matters:** Needed for draft pages, internal docs served publicly, or pages that
  shouldn't pollute search results (e.g., tag listing pages, print views). Yoast/RankMath
  and Docusaurus both support this.
- **Implementation complexity:** Low. Read `robots` from frontmatter, emit `<meta name="robots">`
  in `<head>`. Default to no robots meta tag (allow indexing).

#### 10. Heading Anchor Slug Stability

- **What it does:** Ensures heading anchor IDs are stable across builds and match common
  conventions (GitHub-compatible slugs). Documents the slug algorithm. Supports frontmatter
  `slug` or `id` override per heading.
- **Why it matters:** Search engines index anchor fragments. Links to `#installation` that
  change to `#installing` break inbound links and lose any SEO value accumulated on the
  original anchor. Stable anchors preserve link equity.
- **Implementation complexity:** Low. Document and test the current slug algorithm. Ensure
  it matches GitHub's algorithm for maximum link compatibility.

#### 11. HTML Language Attribute

- **What it does:** Adds `lang` attribute to `<html>` tag (e.g., `<html lang="en">`).
  Configurable via site config (default: `en`).
- **Why it matters:** Search engines use `lang` for language-specific ranking. Screen readers
  use it for pronunciation. Lighthouse flags its absence. Trivial to implement but often
  overlooked.
- **Implementation complexity:** Low. Add `language` field to `SiteConfig`, default to `"en"`,
  expose in template context, add to `<html>` tag in all themes.

#### 12. Related Pages via Tags

- **What it does:** Displays "Related pages" section at the bottom of each page, populated
  from shared tags in frontmatter. Links to other pages with matching tags, ranked by
  number of shared tags.
- **Why it matters:** Internal linking is one of the strongest on-page SEO signals. Related
  pages increase time on site, reduce bounce rate (indirect ranking signals), and help
  search engines understand content relationships. Hugo has this built in.
- **Implementation complexity:** Medium. The metadata index already supports `ByTag()` lookup.
  Need to compute related pages per render and expose via template function or enrichment data.

#### 13. 404 Page with Navigation

- **What it does:** Custom 404 page that includes site navigation, search (when available),
  and suggested pages. Served with proper 404 status code.
- **Why it matters:** A useful 404 page retains visitors who arrive via broken links. Search
  engines use 404 response codes (not content) for de-indexing, so the page content is purely
  for user retention. Reduces bounce rate from dead links.
- **Implementation complexity:** Low. Create a 404 template. Serve it from the error handler
  with navigation context. In `build` mode, output `404.html` (convention for static hosts
  like Netlify, GitHub Pages, Cloudflare Pages).

### P3: Nice to Have (Long-Term)

#### 14. Redirect Support

- **What it does:** Supports frontmatter `redirect_from: [/old-url, /legacy/path]` and/or a
  `_redirects` file. In `serve` mode, responds with 301. In `build` mode, generates redirect
  HTML files (`<meta http-equiv="refresh">`) or a `_redirects` file for Netlify/Cloudflare.
- **Why it matters:** When documentation reorganizes, old URLs lose accumulated link equity
  without redirects. 301 redirects pass ~90-99% of link equity to the new URL. Hugo, Docusaurus,
  and WordPress all support this.
- **Implementation complexity:** Medium. Parse redirect config, register routes in serve mode,
  generate files in build mode.

#### 15. RSS/Atom Feed

- **What it does:** Generates an RSS or Atom feed at `/feed.xml` listing recently modified
  pages with titles, descriptions, and links.
- **Why it matters:** Search engines use feeds for faster content discovery. RSS readers and
  aggregators drive traffic. Minor SEO impact but good for content discoverability.
- **Implementation complexity:** Medium. Walk content tree, sort by date, render XML feed.

#### 16. Preconnect/Preload Resource Hints

- **What it does:** Adds `<link rel="preconnect">` for external domains (Google Fonts, CDNs)
  and `<link rel="preload">` for critical resources (fonts, CSS) in `<head>`.
- **Why it matters:** Reduces connection setup time for external resources. Improves LCP by
  prioritizing critical resources. Part of Core Web Vitals optimization.
- **Implementation complexity:** Low. Add static `<link>` tags to themes that use external
  fonts/CDN resources (KaTeX, Mermaid).

#### 17. Image Dimension Attributes

- **What it does:** Post-processes rendered HTML to add `width` and `height` attributes to
  `<img>` tags that lack them. Reads image dimensions from the content provider.
- **Why it matters:** Prevents Cumulative Layout Shift (CLS), a Core Web Vitals metric.
  Images without dimensions cause layout reflows when they load. Lighthouse flags this.
- **Implementation complexity:** Medium. Requires reading image files to determine dimensions
  during post-processing. Could be expensive for large image counts.

#### 18. `<link rel="next/prev">` for Sequential Pages

- **What it does:** Adds `<link rel="next">` and `<link rel="prev">` for pages that are part
  of a sequence (e.g., multi-page guides). Determined from navigation order or frontmatter.
- **Why it matters:** Helps search engines understand multi-part content. Minor ranking signal
  but aids crawl efficiency. Google deprecated this for web search but Bing still uses it.
- **Implementation complexity:** Low. Derive from navigation tree (previous/next siblings).

## Features to Skip

### Content Analysis / SEO Scoring

Yoast's "green dot" content analysis (keyword density, readability scores, etc.) is a content
strategy tool, not a technical SEO feature. It requires NLP processing, is language-dependent,
and doesn't belong in a documentation viewer. Documentation writers follow style guides, not
SEO keyword recommendations.

### AMP (Accelerated Mobile Pages)

Google has deprecated AMP as a ranking signal and removed the AMP badge from search results.
Not worth implementing.

### Hreflang / Multi-Language

~~gomddoc has explicitly deferred i18n. When/if it ships, `hreflang` tags would be needed, but
it's premature to implement now.~~

**Superseded.** i18n shipped, and with it `hreflang`: on a multi-language site,
`partials/hreflang.html.tmpl` emits one `<link rel="alternate" hreflang="…">` per detected language
plus `x-default`, in both serve and build. The reasoning above was right about the ordering — the
tags followed the feature.

### News/Video Sitemaps

These are for media-heavy sites, not documentation. Standard XML sitemap is sufficient.

### Schema.org FAQ/HowTo Markup

While WordPress plugins auto-detect FAQ content, this requires content structure analysis that
doesn't fit gomddoc's Markdown-to-HTML pipeline. Authors can add JSON-LD manually if needed.

### Internal Link Analysis / Orphan Page Detection

Tools like Screaming Frog and Yoast provide internal link auditing. This is a build-time
analysis tool, not a rendering feature. Could be part of `gomddoc validate` in the future
but is not an SEO feature to implement in the renderer.

### WebP/AVIF Image Conversion

Image format optimization requires a build-time processing pipeline. gomddoc passes through
images unchanged, which is correct for a documentation viewer. Image optimization belongs
in the CI/CD pipeline or content authoring workflow, not the documentation server.

### Speculative Prerendering / Prefetch

Chrome's Speculation Rules API (`<script type="speculationrules">`) enables instant navigations.
Interesting for future consideration but low SEO impact -- it improves user experience metrics
but doesn't directly affect indexing or ranking.
