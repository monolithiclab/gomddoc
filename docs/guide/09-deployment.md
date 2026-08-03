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

The `serve` command starts a production HTTP server with full control over port, caching, and
authentication. Point it at a local directory and your markdown is rendered as styled HTML with an
auto-generated navigation sidebar, table of contents, breadcrumbs, and a search modal accessible
via Ctrl+K (Cmd+K on macOS).

```bash
gomddoc serve
```

Open your browser to `http://localhost:8080` to see your documentation site.

Serve a specific directory on a different port:

```bash
gomddoc serve ./my-docs -p :3000
```

### Serving from Git

You can serve documentation directly from a public repository without cloning it manually. The
repository is cloned lazily on the first request — a shallow clone that minimizes bandwidth and
memory:

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

This walks your content directory, renders all markdown through the full template pipeline (with navigation,
breadcrumbs, TOC, and styling), copies non-markdown files as-is, and generates `index.html` files alongside
`README.html` for clean URLs.

The output includes:

- Rendered HTML for all markdown files (with navigation, breadcrumbs, TOC, styling)
- Non-markdown files copied as-is (images, PDFs, etc.)
- `robots.txt` — always generated
- `sitemap.xml` — generated when `meta.domain` is configured
- `feed.xml` — Atom 1.0 feed with the 20 most recently modified pages
- `404.html` — error page for static hosts (Netlify, GitHub Pages, Cloudflare Pages)
- `_assets/` — theme static assets (CSS, JS, fonts)

The output is a self-contained directory ready for:

- **GitHub Pages**: Push `./public` to a `gh-pages` branch
- **Netlify/Vercel**: Point the build output to `./public`
- **S3 + CloudFront**: Sync `./public` to an S3 bucket
- **Any static file server**: Serve `./public` with nginx, caddy, etc.

### Multi-Language Build

When your content includes BCP 47 directories (e.g., `fr-FR/`, `es-ES/`), the build generates per-language output
with separate sitemaps, feeds, and 404 pages:

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
│   └── 404.html
└── _assets/
```

The `sitemap-index.xml` is only generated when more than one language is detected. See
[Internationalization](13-internationalization.md) for the full multi-language guide.

## Container Deployment

Official images are published to GitHub Container Registry by the release workflow:

```bash
docker pull ghcr.io/monolithiclab/gomddoc:latest
```

## Docker Image

### Building the Image

The repository's `Dockerfile` is **not** self-contained: GoReleaser cross-compiles the binary per
target platform and drops it into the build context, so the Dockerfile only copies it in. Building
straight from a fresh clone fails, because there is no `gomddoc` binary in the context yet.

To build locally, cross-compile a Linux binary into the context first — a macOS binary will not run
in the Linux runtime image:

```bash
GOOS=linux GOARCH=amd64 go build -o gomddoc ./cmd/gomddoc
docker build -t gomddoc .
rm gomddoc
```

The runtime image is [distroless](https://github.com/GoogleContainerTools/distroless)
(`gcr.io/distroless/static-debian12:nonroot`) and runs as a non-root user. It contains only the
gomddoc binary -- no shell, no package manager, no unnecessary system libraries. Theme assets are
embedded in the binary via `//go:embed`, so nothing else is copied in.

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

If your content directory contains a `.gomddoc/config.yml` file, it will be picked up automatically when you mount the directory:

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
    build: .
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
        runAsUser: 65534
      containers:
        - name: gomddoc
          image: gomddoc:latest
          ports:
            - containerPort: 8080
          args: ["serve", "/content"]
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

**Image size.** The distroless base image keeps the final image under 20 MB. There is no shell or package manager, which reduces the attack surface.

**Non-root execution.** The container runs as the `nonroot` user (UID 65534) provided by the distroless image. No additional configuration is needed.

**Read-only filesystem.** gomddoc does not write to disk at runtime. Mount content volumes as read-only and set `readOnlyRootFilesystem: true` in your container security context.

**Graceful shutdown.** gomddoc handles SIGTERM for graceful shutdown, which is the default signal sent by Docker and Kubernetes when stopping a container. In-flight requests are completed before the process exits.

**Resource limits.** gomddoc has a small memory footprint. Start with 32-64 MB memory and 50-100m CPU, then adjust based on observed usage.
