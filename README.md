# Gomddoc

A production-ready HTTP server and static site generator for Markdown documentation, with content negotiation,
full-text search, SEO output and a built-in MCP server.

## Status

Released and in use: v0.1.3 (2026-09-25) is on GitHub Releases, the Homebrew tap (`monolithiclab/tap`) and
`ghcr.io/monolithiclab/gomddoc`, and it publishes Monolithic Lab's own sites. Pre-1.0: configuration keys can still
change between minor versions (since v0.1.2 an unknown key in `.gomddoc/config.yml` is a load error, so check a site
with `gomddoc doctor` before bumping a pinned version). Open bugs and the ranked backlog are in [ROADMAP.md](ROADMAP.md).

## Features

- **Universal Content Serving**: Renders Markdown to HTML inside a theme; serves HTML, CSS, JS, images and every
  other file type with its detected MIME type
- **Content Negotiation**: `Accept` header support; `Accept: text/markdown` returns the raw Markdown source, an
  unsatisfiable `Accept` gets `406`
- **Clean URLs**: `.md` is stripped from URLs by default (`/guide` serves `guide.md`; `/guide.md` redirects with
  `301`)
- **Git-Native Content**: Serve a local directory or a `git+https://` / `git+ssh://` repository URL (in-memory or
  on-disk clone)
- **Static Site Generation**: `gomddoc build` writes a static site with clean-URL directories, `404.html`, sitemap,
  feed and redirect stubs
- **Full-Text Search**: TF-IDF inverted index built at startup, `tag:` filter syntax, `GET /api/search`, and a
  Ctrl+K search dialog in every theme
- **SEO**: Canonical URLs, `sitemap.xml`, `robots.txt`, Atom `feed.xml`, Open Graph and Twitter tags, JSON-LD,
  per-page `robots`, `redirect_from` and `rel="prev/next"`, enabled by setting a domain
- **Tags**: Frontmatter tags rendered as chips, `/tags/` and `/tags/{tag}` pages, a see-also section and
  `/api/tags`
- **Internationalization**: BCP 47 language directories (`fr-FR/`) served under `/{lang}/`, translated UI strings,
  a language switcher and `hreflang` tags
- **MCP Server**: `gomddoc mcp` (stdio) and `/_mcp/` (Streamable HTTP) expose read-only search, page, section and
  navigation tools to AI agents
- **Themes**: One bundled theme (`default`) plus seven more in
  [gomddoc-themes](https://github.com/monolithiclab/gomddoc-themes); feature toggles and CSS variables in config
- **Self-Documenting**: `gomddoc help`, `gomddoc info`, `gomddoc schema` and `gomddoc doctor` read the embedded
  guide, list every setting, print the config JSON Schema and check a site for problems
- **Secure Defaults**: Path jail, hidden-file and `exclude` blocking (404), GET/HEAD-only content routes,
  security headers, optional htpasswd Basic Auth
- **Operations**: Graceful shutdown, configurable timeouts, gzip, ETags, Prometheus metrics, health endpoints,
  optional pprof, and a separate admin listener

## Install

```bash
# Install script (Linux / macOS, amd64 / arm64): checksum-verified, cosign-verified when cosign is installed
curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh | sh

# Homebrew (macOS / Linux)
brew install monolithiclab/tap/gomddoc

# Go toolchain
go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest

# Docker (multi-arch image on GitHub Container Registry)
docker run --rm -p 8080:8080 -v "$PWD:/site" ghcr.io/monolithiclab/gomddoc serve /site
```

Release archives carry an SPDX SBOM each and a Sigstore-signed `SHA256SUMS`; the
[Quick Start](docs/guide/01-quickstart.md) explains the install script's options and how to verify a release by hand.

## Use

```bash
gomddoc serve ./docs                       # production server on :8080 (also git+https:// and git+ssh:// URLs)
gomddoc preview ./docs --open              # dev mode, first free port from 8080
gomddoc build ./docs -d docs.example.com   # static site into build/site
gomddoc doctor ./docs                      # every configuration and content problem, with fixes
gomddoc help                               # the user guide, embedded in the binary
```

## Documentation

The user guide is [docs/guide/](docs/guide/README.md), the same pages `gomddoc help` and the `gomddoc mcp` server
serve:

| Topic | Page |
| --- | --- |
| Install, first site, verifying a release | [Quick Start](docs/guide/01-quickstart.md) |
| Every flag, environment variable and config key | [Configuration](docs/guide/02-configuration.md) |
| Local directories and Git sources | [Content Sources](docs/guide/03-content-sources.md) |
| AI agents over MCP | [MCP Server](docs/guide/04-mcp.md) |
| Themes, templates, web components | [Theming & Assets](docs/guide/05-theming-and-assets.md) |
| Authentication, path jail, headers | [Security](docs/guide/07-security.md) |
| Health, metrics, pprof | [Observability](docs/guide/08-observability.md) |
| Docker, Kubernetes, static hosting | [Deployment](docs/guide/09-deployment.md) |
| Search, SEO, i18n | [Search](docs/guide/10-search.md), [SEO](docs/guide/11-seo.md), [Internationalization](docs/guide/13-internationalization.md) |
| HTTP endpoints and headers, custom renderers | [Advanced Topics](docs/guide/12-advanced/README.md) |
| Problems and fixes | [Doctor](docs/guide/14-doctor.md), [Troubleshooting](docs/guide/15-troubleshooting.md) |

For contributors: [docs/architecture.md](docs/architecture.md) (how the pieces fit),
[docs/decisions.md](docs/decisions.md) (why, D-numbered), `docs/superpowers/specs/` and `plans/` (feature designs and
their execution notes), [docs/reviews/](docs/reviews/) (review rounds) and [ROADMAP.md](ROADMAP.md).

## Development

Requirements: Go (the exact toolchain is pinned by `go.mod`'s `toolchain` line and fetched automatically) and Make.
Lint and benchmark tools are pinned in `tools/go.mod`; nothing needs installing globally.

```bash
make help   # every command
make ci     # the gate CI runs: golangci-lint, govulncheck, go mod tidy -diff, action/image pin checks,
            # then tests with -race and 87% minimum coverage (needs network for govulncheck)
make run    # serve ./testsite locally
```

Contributions: branch, `make lint-fix`, `make ci`, then a pull request. Coverage stays at 87% or above.

## License

gomddoc is **dual-licensed**:

- **Noncommercial use** (personal projects, hobby use, education, research, and other noncommercial
  purposes) is free under the [PolyForm Noncommercial License 1.0.0](LICENSE).
- **Commercial use** — any use that is not a noncommercial purpose, including use in or for a
  for-profit business, product, or service — requires a separate commercial license. See
  [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md).

Release notes for each version are on [GitHub Releases](https://github.com/monolithiclab/gomddoc/releases),
generated by GoReleaser from conventional commit messages.
