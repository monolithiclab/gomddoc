---
title: "Deployment"
description: "Deploy gomddoc as a live server or build static sites for any hosting platform."
author: "nicolasm"
tags: ["deployment", "docker", "kubernetes"]
---

# Deployment

gomddoc supports two deployment models: **dynamic serving** via `gomddoc serve` and **static site
generation** via `gomddoc build` (deploy to any static host).

## Live Server

The `serve` command starts a production HTTP server with control over the listen address, admin port, domain,
Git access, profiling and authentication. Point it at a local directory and your markdown is rendered as styled HTML
with an auto-generated navigation sidebar, table of contents, breadcrumbs, and a search modal accessible via Ctrl+K
(Cmd+K on macOS).

```bash
gomddoc serve
```

Open your browser to `http://localhost:8080` to see your documentation site.

Serve a specific directory on a different port:

```bash
gomddoc serve ./my-docs -p :3000
```

### Serving from Git

You can serve documentation directly from a public repository without cloning it manually. gomddoc clones the
repository at startup (shallow, depth 1, single branch, no tags) and serves that snapshot; restart the server to
pick up new commits:

```bash
gomddoc serve "git+https://github.com/monolithiclab/gomddoc.git"
```

For private repositories, provide an SSH key:

```bash
gomddoc serve "git+ssh://git@github.com/org/private-docs.git" --git-key-file ~/.ssh/id_ed25519
```

See [Content Sources](03-content-sources.md) for more details on Git-based serving, including
disk-based storage and SSH configuration.

## Static Site Deployment

Generate a static site and deploy to any static host:

```bash
gomddoc build ./docs -o ./public
```

`build` takes the same directory argument as `serve` (a local directory or a Git URL) and these flags:
`-o/--output` (default `build/site`, `GOMDDOC_BUILD_OUTPUT`), `-d/--domain`, `--git-key-file` and
`--git-storage-dir`.

This walks your content directory, renders all markdown through the full template pipeline (with navigation,
breadcrumbs, TOC, and styling), and copies non-markdown files as-is. Hidden files and `exclude` patterns are skipped.
Output file names depend on `strip_extensions`:

| Source | Without `strip_extensions` | With `strip_extensions` |
|---|---|---|
| `guide.md` | `guide.html` | `guide/index.html`, plus a redirect stub at `guide.md` pointing to `/guide` |
| `README.md` (default index) | `index.html` | `index.html` |
| `index.md` | `index.html` | `index.html` |

When a directory has both `index.md` and `README.md`, `index.md` becomes `index.html` and the README keeps its own
name (`README.html`, or `README/index.html` with `strip_extensions`).

The output includes:

- Rendered HTML for all markdown files (with navigation, breadcrumbs, TOC, styling)
- Non-markdown files copied as-is (images, PDFs, etc.)
- `robots.txt` — always generated; its `Sitemap:` line is present only when `meta.domain` is set
- `sitemap.xml` — generated when `meta.domain` is configured
- `feed.xml` — Atom 1.0 feed with the 20 most recently modified pages, generated when `meta.domain` is configured
- `tags/index.html` and `tags/{tag}/index.html` — tag index and tag pages
- Redirect stubs for `redirect_from` frontmatter
- `404.html` — error page for static hosts (Netlify, GitHub Pages, Cloudflare Pages)
- `_assets/` — static assets from the site, theme and shared layers (see
  [Theming](05-theming-and-assets.md#static-asset-overlay))
- `.gomddoc-build` — a marker file

`build` deletes and recreates the output directory on each run, but only when it is empty or contains the
`.gomddoc-build` marker from a previous build. It refuses to write into any other non-empty directory. Rendering runs
in parallel, one worker per CPU. The run ends with a `Build complete` log line counting markdown, copied and skipped
files.

A static build has no server behind it, so the dynamic endpoints do not exist: `/api/search`, `/api/tags`, `/_mcp/`,
`/health/*` and `/metrics`. The search modal queries `/api/search`, so search does not work on a static host; the
search button still renders while the `search` feature is on. Disable it for static builds:

```yaml
# .gomddoc/config.yml
theme:
  features:
    search: false
```

Content negotiation is also gone: a static host serves the files as they are, so a request with
`Accept: text/markdown` gets the HTML page.

The output is a self-contained directory ready for:

- **GitHub Pages**: Push `./public` to a `gh-pages` branch
- **Netlify/Vercel**: Point the build output to `./public`
- **S3 + CloudFront**: Sync `./public` to an S3 bucket
- **Any static file server**: Serve `./public` with nginx, caddy, etc.

### Multi-Language Build

When your content includes BCP 47 directories (e.g., `fr-FR/`, `es-ES/`), the build generates per-language output
with separate 404 pages, redirect stubs and, when `meta.domain` is set, sitemaps, feeds and tag pages:

```text
./public/
├── index.html
├── robots.txt
├── sitemap.xml              ← default language
├── feed.xml                 ← default language
├── 404.html                 ← default language
├── sitemap-index.xml        ← references all per-language sitemaps
├── fr-FR/
│   ├── index.html
│   ├── sitemap.xml
│   ├── feed.xml
│   ├── 404.html
│   └── tags/
├── tags/
└── _assets/
```

The `sitemap-index.xml` is generated when `meta.domain` is set and at least one language directory is detected; it is
then the sitemap `robots.txt` names. See [Internationalization](13-internationalization.md) for the full multi-language
guide.

## Container Deployment

Official images are published to GitHub Container Registry by the release workflow (GoReleaser) for `linux/amd64`
and `linux/arm64`, and signed with cosign:

```bash
docker pull ghcr.io/monolithiclab/gomddoc:latest
```

Tags: `{version}` (multi-arch manifest, e.g. `0.1.2`), `{version}-amd64`, `{version}-arm64`, and `latest`, which is
not moved by prereleases. For other installation methods (Homebrew, install script, `go install`, release archives),
see the [Quickstart](01-quickstart.md).

## Docker Image

### Building the Image

The repository's `Dockerfile` is **not** self-contained: GoReleaser cross-compiles the binary per
target platform and drops it into the build context, so the Dockerfile only copies it in. Building
straight from a fresh clone fails, because there is no `gomddoc` binary in the context yet.

To build locally, cross-compile a static Linux binary into the context first. A macOS binary will not run in the
Linux runtime image, and the distroless `static` base has no C library, so build with `CGO_ENABLED=0`:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gomddoc ./cmd/gomddoc
docker build -t gomddoc .
rm gomddoc
```

The runtime image is [distroless](https://github.com/GoogleContainerTools/distroless)
(`gcr.io/distroless/static-debian12:nonroot`) and runs as a non-root user. It contains only the
gomddoc binary at `/gomddoc`: no shell, no package manager, no unnecessary system libraries. Theme assets are
embedded in the binary via `//go:embed`, so nothing else is copied in. The image exposes port 8080 and its
entrypoint is `/gomddoc`.

### Running the Container

Serve Markdown files from a local directory:

```bash
docker run -p 8080:8080 -v /path/to/docs:/content:ro gomddoc serve /content
```

The `:ro` flag mounts the volume as read-only, which is recommended since gomddoc only reads content.

### Configuration

gomddoc accepts configuration via CLI flags and environment variables. Both work inside Docker.

**CLI flags:**

```bash
docker run -p 9000:9000 -v ./docs:/content:ro gomddoc serve /content -p :9000
```

**Environment variables:**

```bash
docker run -p 8080:8080 \
  -e GOMDDOC_SERVER_PORT=:8080 \
  -e GOMDDOC_SERVER_DIR=/content \
  -e GOMDDOC_SITE_META_TITLE="My Documentation" \
  -e GOMDDOC_SITE_DIR_INDEX=true \
  -v ./docs:/content:ro \
  gomddoc serve
```

The subcommand is still required: the image's entrypoint is the bare binary, and `gomddoc` with no
subcommand prints help and exits 1. `GOMDDOC_SERVER_DIR` supplies the directory argument.

See [Configuration](02-configuration.md) for the full list of environment variables and flags.

### Site Configuration File

If your content directory contains a `.gomddoc/config.yml` file, it will be picked up automatically when you mount the
directory:

```bash
docker run -p 8080:8080 -v ./my-site:/content:ro gomddoc serve /content
```

Where `./my-site/.gomddoc/config.yml` might contain:

```yaml
meta:
  title: "My Documentation Site"
  description: "Project documentation"
  domain: "docs.example.com"
theme:
  name: "default"
dir_index: true
```

## Docker Compose

A typical `docker-compose.yml` for running gomddoc:

```yaml
services:
  gomddoc:
    image: ghcr.io/monolithiclab/gomddoc:latest
    command: ["serve"]
    ports:
      - "8080:8080"
    volumes:
      - ./content:/content:ro
    environment:
      GOMDDOC_SERVER_DIR: /content
      GOMDDOC_SITE_META_TITLE: "My Documentation"
    restart: unless-stopped
    read_only: true
```

`command` is required: the image's entrypoint is the bare binary.

To start the service:

```bash
docker compose up -d
```

## Kubernetes

A minimal Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gomddoc
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gomddoc
  template:
    metadata:
      labels:
        app: gomddoc
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
      containers:
        - name: gomddoc
          image: ghcr.io/monolithiclab/gomddoc:latest
          ports:
            - containerPort: 8080
          args: ["serve", "/content"]
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
          volumeMounts:
            - name: content
              mountPath: /content
              readOnly: true
          resources:
            limits:
              memory: "64Mi"
              cpu: "100m"
            requests:
              memory: "32Mi"
              cpu: "50m"
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
      volumes:
        - name: content
          configMap:
            name: docs-content
---
apiVersion: v1
kind: Service
metadata:
  name: gomddoc
spec:
  selector:
    app: gomddoc
  ports:
    - port: 80
      targetPort: 8080
```

## Production Considerations

**Image size.** The image is the distroless base (about 2 MB) plus the gomddoc binary (about 27 MB for linux/amd64),
around 30 MB in total. There is no shell or package manager, which reduces the attack surface.

**Non-root execution.** The container runs as the distroless `nonroot` user (UID 65532). No additional configuration
is needed.

**Read-only filesystem.** `serve` does not write to disk at runtime, except the Git clone when
`--git-storage-dir` is set; give that directory a writable volume. Mount content volumes as read-only and set
`readOnlyRootFilesystem: true` in your container security context.

**Graceful shutdown.** gomddoc handles SIGTERM and SIGINT for graceful shutdown. SIGTERM is the default signal sent by
Docker and Kubernetes when stopping a container. In-flight requests get `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT`
(default 1s) to complete before the process exits.

**Health probes.** Point liveness probes at `/health/live` and readiness probes at `/health/ready`. They stay on the
main port even when `--admin-port` is set, and skip authentication. See [Observability](08-observability.md).

**Admin port.** In production, set `--admin-port` so `/metrics` and pprof leave the main port. A bare `:9090` binds
127.0.0.1, which a Prometheus scraper in another container or pod cannot reach; use `0.0.0.0:9090` there.

> [!WARNING]
> Never expose the admin port publicly. `/metrics` and the health endpoints on it have no authentication, even with
> `--basic-auth-file`. Do not publish it in a Docker `-p` mapping, a Kubernetes `Service` of type `LoadBalancer` or
> `NodePort`, or an ingress; restrict it to the scraper with a network policy or firewall rule.

**Resource limits.** gomddoc keeps the metadata index, search index and navigation tree in memory, plus the whole
repository for a Git source without `--git-storage-dir`. A small site runs in about 35 MB of resident memory. Start
with a 64 MB memory limit and 50-100m CPU, then adjust based on observed usage and content size.
