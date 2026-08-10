---
title: "Internationalization (i18n)"
description: "Multi-language documentation sites with automatic language detection, translated UI, and per-language SEO."
author: "nicolasm"
tags: ["i18n", "localization", "multilingual"]
---

# Internationalization (i18n)

gomddoc supports multi-language documentation sites with automatic language detection, translated UI strings, a
language switcher, per-language search indexes, and proper SEO (hreflang tags, per-language sitemaps, and Atom feeds).

No plugins or external tools are required. Place content in BCP 47 directories (e.g., `fr-FR/`, `es-ES/`), provide
translation files, and gomddoc handles the rest.

## Content Structure

Multi-language content is organized by placing translated pages in directories named with BCP 47 language tags —
a language subtag plus a script and/or a region (e.g., `fr-FR`, `es-ES`, `ja-JP`, `zh-Hans`, `es-419`, `sr-Latn-RS`).

Content at the root of your documentation directory is served as the **default language** (configured via
`language` in your config, default: `en-US`). Each BCP 47 directory becomes an additional language:

```text
/my-docs
├── .gomddoc/
│   ├── config.yml
│   └── locales/          ← site-level translation overrides
│       └── fr-FR.yml
├── README.md             ← default language (en-US)
├── guide/
│   └── setup.md
├── fr-FR/                ← French content
│   ├── README.md
│   └── guide/
│       └── setup.md
└── es-ES/                ← Spanish content
    ├── README.md
    └── guide/
        └── setup.md
```

### URL Routing

The default language is served at the root without a prefix. Non-default languages are prefixed with their BCP 47 code:

| Content File | URL |
|---|---|
| `README.md` | `/` |
| `guide/setup.md` | `/guide/setup` |
| `fr-FR/README.md` | `/fr-FR/` |
| `fr-FR/guide/setup.md` | `/fr-FR/guide/setup` |
| `es-ES/README.md` | `/es-ES/` |

Extension stripping, clean URLs, and all other URL features work the same way within each language prefix.

### Language Detection

gomddoc detects available languages automatically at startup by scanning the content root. No configuration is needed
beyond creating the directories.

Requirements for a directory to be detected as a language:

- A valid BCP 47 language subtag (2–3 letters), which must be a **real** language — `zz-ZZ` is well-formed but is not
  one, and is ignored
- Followed by a script subtag (`zh-Hans`), a region subtag (`fr-FR`, `es-419`), or both (`sr-Latn-RS`)
- Written in canonical form: lowercase language, title-case script, uppercase region. `en-us` is not detected, so one
  language cannot end up split across two directories

**A bare language subtag such as `en/` or `fr/` is deliberately not detected.** Detection is automatic and runs against
every directory at the content root, and short bare subtags collide with ordinary directory names — `doc`, `api`,
`css`, `bin`, `id`, `is`, `no` and `it` are all real language codes. Mistaking one for a translation tree would take
that content out of the main site, so the extra subtag is required. Use `en-US`, `fr-FR` and so on.

Directories that don't match (e.g., `docs/`, `api/`, `getting-started/`) are treated as regular content directories.

## Translation Files

UI strings (navigation labels, search placeholders, ARIA attributes, button text) are managed through YAML translation
files. gomddoc ships with English (`en-US`) translations built in. To add or override translations for any language,
create locale files.

### Built-in Translation Keys

The default `en-US.yml` provides these 26 keys. `TestBuiltinTranslationKeysAreDocumented`
(`cmd/gomddoc/locale_docs_test.go`) asserts this block and the shipped file name exactly the same
set — a key added to one and not the other fails the build:

```yaml
language_name: English
toc_title: "On this page"
edit_page: "Edit this page"
go_to_homepage: "Go to homepage"
search_placeholder: "Search documentation..."
search_no_results: "No results found"
search_navigate: Navigate
search_open: Open
search_close: Close
copy: Copy
copied: "Copied!"
aria_toggle_nav: "Toggle navigation"
aria_site_nav: "Site navigation"
aria_search: "Search documentation"
aria_toc: "Table of contents"
aria_toggle_toc: "Toggle table of contents"
aria_toggle_theme: "Toggle dark mode"
aria_breadcrumb: Breadcrumb
aria_copy_code: "Copy code to clipboard"
search_shortcut: "Search (Ctrl+K)"
search_tag_tip: "Tip: tag:name filters by tag"
tags_title: Tags
tags_index_title: All tags
tags_tagged_as: "Pages tagged %s"
tags_empty: "No pages tagged %s"
see_also: "See also"
```

`tags_tagged_as` and `tags_empty` carry a single `%s`, which is substituted with the tag name.

The `language_name` key is special — it provides the display name shown in the language switcher (e.g., "Français",
"Español", "日本語").

### Adding Translations

Create locale files in `.gomddoc/locales/` named `{lang}.yml`:

```yaml
# .gomddoc/locales/fr-FR.yml
language_name: Français
toc_title: "Sur cette page"
edit_page: "Modifier cette page"
search_placeholder: "Rechercher..."
copy: Copier
copied: "Copié !"
```

Merging is per key, so this file is deliberately a fragment rather than a copy of all 26 — anything
it omits keeps the built-in English. Start with the strings your readers see most and add the rest
as you go; there is no "incomplete locale" error to avoid.

### Translation Layering

Translations are loaded in a two-level override chain:

1. **Built-in** — the embedded `en-US.yml` provides baseline English strings
2. **Site-level** — `.gomddoc/locales/*.yml` files override any key from the layer above

The second layer uses merge semantics per language: you only need to define the keys you want to
override, and unspecified keys fall back to the built-in file. Themes do **not** ship locale files —
there is no theme-level layer, so a translation cannot vary by theme.

> [!WARNING]
> `.gomddoc/assets/locales/en-US.yml` is a different, sharper thing than `.gomddoc/locales/en-US.yml`.
> The `assets/` path is part of the theme override filesystem, which resolves whole files: putting
> `en-US.yml` there **replaces** the built-in baseline outright instead of merging into it, so every
> key you did not copy across falls through to the key name itself. Use `.gomddoc/locales/`.

### Fallback Chain

When a translation key is requested for a language:

1. Look up the key in the requested language's strings
2. If not found, look up the key in the default language's strings
3. If still not found, return the key itself as a literal string

This means you can add a new language with only `language_name` defined, and all other UI strings will fall back to
English until you translate them.

## Configuration

The `language` setting in `.gomddoc/config.yml` controls the default language for your site:

```yaml
# .gomddoc/config.yml
language: "en-US"
```

Or via environment variable:

```bash
export GOMDDOC_SITE_LANGUAGE=en-US
```

This setting determines:

- The `lang` attribute on the `<html>` tag for default-language pages
- Which language is served at the root URL (without a prefix)
- The fallback language for missing translations

Individual pages can override the language via frontmatter:

```yaml
---
lang: fr-FR
---
```

## Template Functions

Themes access the i18n system through methods on the `TemplateContext`:

### `.T "key"`

Returns the translated string for the current page's language. Falls back to the default language, then to the key
itself.

```html
<span>{{ .T "toc_title" }}</span>
<!-- Output: "Sur cette page" (for fr-FR) -->
<!-- Output: "On this page" (for en-US) -->
```

### `.Lang`

Returns the BCP 47 language code for the current page. Respects the per-page `lang` frontmatter override.

```html
<html lang="{{ .Lang }}">
```

### `.Languages`

Returns the list of all available languages as `LanguageInfo` objects. Used to build the language switcher. Each entry
has:

| Field | Type | Description |
|---|---|---|
| `.Code` | `string` | BCP 47 code (e.g., `"fr-FR"`) |
| `.Name` | `string` | Display name (e.g., `"Français"`) from the `language_name` translation key |
| `.Active` | `bool` | `true` if this is the current page's language |
| `.Default` | `bool` | `true` if this is the site's default language (served without URL prefix) |

The default language is always listed first. The list is only populated when multiple languages are detected.

```html
{{- $langs := .Languages }}
{{- if gt (len $langs) 1 }}
<div class="lang-switcher">
  {{- $path := .Page.Path }}
  {{- range $langs }}
  {{- if .Active }}
  <span class="lang-current">{{ .Name }}</span>
  {{- else }}
  <a href="/{{ .Code }}{{ $path }}">{{ .Name }}</a>
  {{- end }}
  {{- end }}
</div>
{{- end }}
```

## Theme Support

Every theme includes i18n support out of the box:

### Language Switcher

The `lang-switcher` partial renders a language selector when multiple languages are available. It appears in the site
header and shows the display name for each language, with the current language highlighted.

### hreflang Tags

The `hreflang` partial injects `<link rel="alternate" hreflang="...">` tags in the `<head>` for SEO. The default
language gets an unprefixed canonical URL and the `x-default` hreflang. Non-default languages get prefixed URLs:

```html
<link rel="alternate" hreflang="en-US" href="/guide/setup">
<link rel="alternate" hreflang="x-default" href="/guide/setup">
<link rel="alternate" hreflang="fr-FR" href="/fr-FR/guide/setup">
<link rel="alternate" hreflang="es-ES" href="/es-ES/guide/setup">
```

These tags tell search engines which language variants exist for each page, preventing duplicate content issues and
enabling proper language-based search result targeting.

### Creating i18n-Aware Custom Themes

When building a custom theme, use these patterns for i18n support:

1. **Set the HTML lang attribute** using `.Lang`:
   ```html
   <html lang="{{ .Lang }}">
   ```

2. **Use `.T` for all UI strings** instead of hardcoded text:
   ```html
   <button aria-label="{{ .T "aria_toggle_nav" }}">{{ .T "aria_toggle_nav" }}</button>
   ```

3. **Include the language switcher** partial:
   ```html
   {{ template "lang-switcher" . }}
   ```

4. **Include hreflang tags** in the `<head>`:
   ```html
   {{ template "hreflang" . }}
   ```

5. **Pass translations to JavaScript** via data attributes:
   ```html
   <html data-copy-label="{{ .T "copy" }}" data-copied-label="{{ .T "copied" }}">
   ```

## Per-Language Features

Each language gets its own independent:

- **Search index** — full-text search operates within a single language. The search API accepts a `lang` parameter to
  query a specific language's index.
- **Sitemap** — each language has its own `sitemap.xml` (e.g., `/fr-FR/sitemap.xml`), served in both
  modes. When multiple languages exist, a `sitemap-index.xml` referencing them all is written at the
  root — **in `build` mode only**; `serve` has no route for it. `robots.txt` follows: a build that
  writes the index points its `Sitemap:` directive at the index, `serve` always points at
  `/sitemap.xml`.
- **Atom feed** — each language has its own `feed.xml` (e.g., `/fr-FR/feed.xml`).
- **Navigation tree** — sidebar navigation is built from each language's content independently.
- **Metadata index** — tags, related documents, and page metadata are indexed per language.
- **404 page** — each language gets its own `404.html` in build mode.
- **Redirects** — `redirect_from` on a page under `fr-FR/` redirects `/fr-FR/{source}` to that page's
  language-prefixed URL, and never leaks into the default language's redirect table.

The default language indexes **only** the content outside the language directories. A page under
`fr-FR/` appears in `/fr-FR/sitemap.xml`, `/fr-FR/feed.xml`, `/fr-FR/tags/…` and the French sidebar —
never in their default-language counterparts. A tag used only by translated pages therefore has no
default-language tag page.

## Static Site Generation

When building a multi-language site with `gomddoc build`, the output mirrors the URL structure:

```text
build/site/
├── index.html              ← default language root
├── guide/
│   └── setup/
│       └── index.html
├── robots.txt
├── sitemap.xml             ← default language sitemap
├── feed.xml                ← default language feed
├── 404.html                ← default language 404
├── sitemap-index.xml       ← references all per-language sitemaps
├── fr-FR/
│   ├── index.html
│   ├── guide/
│   │   └── setup/
│   │       └── index.html
│   ├── sitemap.xml         ← French sitemap
│   ├── feed.xml            ← French feed
│   ├── 404.html            ← French 404
│   └── tags/               ← French tag pages
└── es-ES/
    ├── index.html
    ├── sitemap.xml         ← Spanish sitemap
    ├── feed.xml            ← Spanish feed
    └── 404.html            ← Spanish 404
```

The `sitemap-index.xml` is only generated when at least one translation directory is detected, so a
single-language site produces a standard `sitemap.xml` at the root and a `robots.txt` naming it.

## Quick Start

To add French to an existing English documentation site:

1. **Create the French content directory:**
   ```bash
   mkdir -p fr-FR/guide
   ```

2. **Add translated content:**
   ```bash
   cp README.md fr-FR/README.md
   cp guide/setup.md fr-FR/guide/setup.md
   # Edit the French files with translated content
   ```

3. **Add French UI translations:**
   ```bash
   mkdir -p .gomddoc/locales
   ```
   Create `.gomddoc/locales/fr-FR.yml` with translated UI strings (see [Adding Translations](#adding-translations)
   above).

4. **Preview:**
   ```bash
   gomddoc preview
   ```
   `preview` binds an auto-assigned port (`--port` defaults to `:auto`) and prints the URL it chose,
   e.g. `Preview: http://localhost:53412`. Visit `/` for English and `/fr-FR/` for French; the
   language switcher appears automatically in the header. Pass `-p :8080` if you want a fixed port,
   or use `gomddoc serve`, which defaults to `:8080`.

5. **Build:**
   ```bash
   gomddoc build -d docs.example.com
   ```
   The output includes both languages with proper hreflang tags, per-language sitemaps, and a sitemap index.
