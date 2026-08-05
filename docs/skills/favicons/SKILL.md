---
name: adding-favicons
description:
  Use when a website is missing favicons, has only a single favicon.ico, or needs PWA-ready icon
  coverage. Generates the full set of cross-browser/cross-device icons from a single SVG source and
  wires them up correctly in the document head and web manifest.
---

# Adding favicons to a website

The 2024+ minimal-yet-complete set is **8 files** plus 6 head tags. Anything more
(apple-touch-icon-precomposed, browserconfig.xml, msapplication tags, dozens of apple-touch-icon
sizes) is legacy noise — skip it.

Source: <https://dev.to/masakudamatsu/favicon-nightmare-how-to-maintain-sanity-3al7>

## Files to produce

Place in the static asset root (served at `/`).

| File                         | Format        | Size(s)                | Purpose                          |
| ---------------------------- | ------------- | ---------------------- | -------------------------------- |
| `favicon.ico`                | ICO           | 16, 32, 48 (multi-res) | Legacy fallback (Safari ≤11, IE) |
| `favicon.svg`                | SVG           | scalable               | Modern browsers                  |
| `apple-touch-icon.png`       | PNG, no alpha | 180×180                | iOS home screen                  |
| `android-chrome-192x192.png` | PNG           | 192×192                | Android `purpose: any`           |
| `android-chrome-512x512.png` | PNG           | 512×512                | Android `purpose: any`           |
| `maskable-icon-192x192.png`  | PNG           | 192×192                | Android `purpose: maskable`      |
| `maskable-icon-512x512.png`  | PNG           | 512×512                | Android `purpose: maskable`      |
| `site.webmanifest`           | JSON          | —                      | PWA manifest                     |

## Head tags (paste verbatim)

```html
<link rel="icon" href="/favicon.ico" sizes="48x48" />
<link rel="icon" href="/favicon.svg" sizes="any" type="image/svg+xml" />
<link rel="apple-touch-icon" href="/apple-touch-icon.png" />
<link rel="manifest" href="/site.webmanifest" />
<meta
  name="theme-color"
  content="#FFFFFF"
  media="(prefers-color-scheme: light)"
/>
<meta
  name="theme-color"
  content="#000000"
  media="(prefers-color-scheme: dark)"
/>
```

Replace the two `theme-color` values with the page's actual light/dark background colors so the
mobile browser chrome blends with the page.

**Rules that are easy to get wrong:**

- `sizes="48x48"` on the ICO is mandatory — without it, Chromium prefers the ICO over the SVG.
- `sizes="any"` on the SVG signals scalability to current/future browsers.
- `favicon.ico` MUST sit at the URL root — Safari and IE don't honor a different path.
- Skip `<link rel="mask-icon">` unless you specifically need to support Safari < 12.

## site.webmanifest

```json
{
  "name": "Full product name",
  "short_name": "Short",
  "description": "One-line product description.",
  "icons": [
    {
      "src": "/android-chrome-192x192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/android-chrome-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/maskable-icon-192x192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "maskable"
    },
    {
      "src": "/maskable-icon-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "maskable"
    }
  ],
  "theme_color": "#BRAND",
  "background_color": "#PAGE_BG",
  "display": "standalone",
  "start_url": "/"
}
```

`theme_color` here is the PWA splash color (use brand color). The `theme-color` `<meta>` tags above
are the per-page browser chrome color (use page bg).

## Design rules per icon type

| Icon                          | Background                                                           | Padding                                                          | Notes                                                                    |
| ----------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `favicon.svg` / `favicon.ico` | Whatever the brand uses                                              | Brand-natural                                                    | Renders at 16–48px; prefer simple shapes — no fine detail                |
| `apple-touch-icon.png`        | **Solid, no transparency**                                           | 20px around content (140×140 in a 180×180 canvas)                | iOS adds its own rounded mask — don't pre-round corners                  |
| `android-chrome-*` (`any`)    | Brand-natural (rounded square is fine)                               | Brand-natural                                                    | Used as-is on home screen                                                |
| `maskable-icon-*`             | **Solid, fills 100% of canvas, no rounded corners, no transparency** | Content within central **80%** safe zone (10% padding all sides) | Android crops to the device's chosen mask shape (circle, squircle, etc.) |

## SVG dark-mode caveat

If you want the SVG favicon to invert in dark mode, use a `<style>` block with `prefers-color-scheme`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <style>
    @media (prefers-color-scheme: dark) { .fg { fill: #fff } }
  </style>
  <path class="fg" fill="#000" d="..."/>
</svg>
```

**Safari does not honor this** — Safari only renders SVG favicons monochrome with the color from
`<link color="…">`. Either accept that Safari will fall back to `favicon.ico`, or design an SVG that
looks acceptable on both backgrounds without inversion (e.g., a colored badge with high-contrast
glyph).

## Generation recipe

Requires `rsvg-convert` (clean SVG→PNG) and `magick` (ImageMagick, for ICO assembly).
Install on macOS: `brew install librsvg imagemagick`.

Starting from one SVG source `source.svg` with an explicit `viewBox` (e.g. `viewBox="0 0 32 32"`):

```bash
SRC=source.svg
OUT=./static
TMP=$(mktemp -d)

# 1. favicon.svg — copy source, ensure it has a viewBox attribute
cp "$SRC" "$OUT/favicon.svg"

# 2. favicon.ico — multi-resolution 16/32/48
rsvg-convert -w 16 -h 16 "$SRC" -o "$TMP/16.png"
rsvg-convert -w 32 -h 32 "$SRC" -o "$TMP/32.png"
rsvg-convert -w 48 -h 48 "$SRC" -o "$TMP/48.png"
magick "$TMP/16.png" "$TMP/32.png" "$TMP/48.png" "$OUT/favicon.ico"

# 3. android-chrome (purpose: any) — uses source as-is
rsvg-convert -w 192 -h 192 "$SRC" -o "$OUT/android-chrome-192x192.png"
rsvg-convert -w 512 -h 512 "$SRC" -o "$OUT/android-chrome-512x512.png"
```

For the **apple-touch-icon** and **maskable** icons, build a wrapper SVG that:

- Fills the entire canvas with a solid brand color (no rounded corners, no transparency).
- Embeds the source glyph centered with the correct padding.

```bash
# Variables: BRAND=#hex, GLYPH=<path d="..." fill="..."/> from source SVG, GLYPH_VIEWBOX=32 (source viewBox size)

# Apple touch icon: 180×180 canvas, 20px padding → glyph in 140×140
cat > "$TMP/apple.svg" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 180" width="180" height="180">
  <rect width="180" height="180" fill="$BRAND"/>
  <g transform="translate(20 20) scale($(echo "140/$GLYPH_VIEWBOX" | bc -l))">$GLYPH</g>
</svg>
EOF
rsvg-convert -w 180 -h 180 "$TMP/apple.svg" -o "$OUT/apple-touch-icon.png"

# Maskable 192: 10% padding (19.2px) → glyph in 153.6×153.6
cat > "$TMP/m192.svg" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 192 192" width="192" height="192">
  <rect width="192" height="192" fill="$BRAND"/>
  <g transform="translate(19.2 19.2) scale($(echo "153.6/$GLYPH_VIEWBOX" | bc -l))">$GLYPH</g>
</svg>
EOF
rsvg-convert -w 192 -h 192 "$TMP/m192.svg" -o "$OUT/maskable-icon-192x192.png"

# Maskable 512: 10% padding (51.2px) → glyph in 409.6×409.6
cat > "$TMP/m512.svg" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect width="512" height="512" fill="$BRAND"/>
  <g transform="translate(51.2 51.2) scale($(echo "409.6/$GLYPH_VIEWBOX" | bc -l))">$GLYPH</g>
</svg>
EOF
rsvg-convert -w 512 -h 512 "$TMP/m512.svg" -o "$OUT/maskable-icon-512x512.png"
```

## Verification checklist

`scripts/favicon-check.go` asserts every rule on this page against a deployed site: the six head
tags, all eight assets served from the URL root, the ICO's three resolutions, each PNG's dimensions,
the apple-touch-icon's opacity, and the manifest's fields and four icon entries. It needs nothing
beyond the Go toolchain — no `magick`, no `file`, no `python3`.

```bash
go run scripts/favicon-check.go https://your.site
```

One line per requirement; exits non-zero if any fail.

```
[OK  ] <link rel="icon" href="/favicon.ico" sizes="48x48">
[FAIL] /apple-touch-icon.png has no alpha channel (iOS expects opaque) — PNG color type 6 includes alpha

33 checks, 1 failures
```

Two rules it cannot check, because neither is observable over HTTP:

- **Maskable safe zone** — whether the content stays inside the central 80%. Preview the crop at
  <https://maskable.app/editor>.
- **Manifest warnings** — DevTools → Application → Manifest should parse clean and show all four
  icons.

## Common mistakes

| Mistake                                            | Symptom                                                  | Fix                                                    |
| -------------------------------------------------- | -------------------------------------------------------- | ------------------------------------------------------ |
| Single-res `favicon.ico`                           | Blurry favicon at 32px or HiDPI                          | Build with 16/32/48                                    |
| Missing `sizes="48x48"` on ICO link                | Chromium uses ICO instead of SVG (lower quality)         | Add `sizes="48x48"`                                    |
| Apple touch icon has transparent corners           | iOS shows a square hole around the rounded mask          | Use solid background, no rounded corners in source     |
| Maskable icon has rounded corners or padding < 10% | Android crops important content                          | Square fill, content in central 80% only               |
| `theme-color` only set once                        | Status bar mismatches in light or dark mode              | Two `<meta>` with `prefers-color-scheme` media queries |
| `favicon.ico` not at `/` (e.g., in `/assets/`)     | Safari/IE 404 silently                                   | Must be at root URL                                    |
| Manifest references missing icons                  | DevTools manifest tab shows errors, install prompt fails | Run `favicon-check.go` — it probes all eight assets    |
