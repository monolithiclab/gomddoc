# Deployment

gomddoc supports two deployment models: **dynamic serving** via `gomddoc serve` (container-based) and **static site generation** via `gomddoc build` (deploy to any static host).

## Static Site Deployment

Generate a static site and deploy to any static host:

```bash
gomddoc build ./docs -o ./public
```

This walks your content directory, renders all markdown through the full template pipeline (with navigation,
breadcrumbs, TOC, and styling), copies non-markdown files as-is, and generates `index.html` files alongside
`README.html` for clean URLs.

The output is a self-contained directory ready for:

- **GitHub Pages**: Push `./public` to a `gh-pages` branch
- **Netlify/Vercel**: Point the build output to `./public`
- **S3 + CloudFront**: Sync `./public` to an S3 bucket
- **Any static file server**: Serve `./public` with nginx, caddy, etc.

## Container Deployment

gomddoc ships with a multi-stage Dockerfile that produces a minimal, secure container image suitable for production deployment.

## Docker Image

### Building the Image

```bash
docker build -t gomddoc .
```

The multi-stage build:

1. Compiles a statically-linked Go binary with stripped debug symbols (`-ldflags="-s -w"`)
2. Copies the binary into a [distroless](https://github.com/GoogleContainerTools/distroless) runtime image running as a non-root user

The final image contains only the gomddoc binary -- no shell, no package manager, no unnecessary system libraries.

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
  gomddoc
```

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
          args: ["-d", "/content"]
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
