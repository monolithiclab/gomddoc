---
title: "Troubleshooting"
description: "What gomddoc serves for each file type and directory, and fixes for the common problems: 403s, 406s, stale content, ports."
tags: ["troubleshooting", "content-types", "directories"]
---

# Troubleshooting

Run `gomddoc doctor` first: it reports every configuration problem and the cheap content problems, each with a
location and a fix (see [Doctor](14-doctor.md)).

```bash
gomddoc doctor ./docs
```

## What gets served

| Content type                   | Handling                                      | Template wrapping |
| ------------------------------ | --------------------------------------------- | ----------------- |
| `.md`, `.markdown`             | Markdown → HTML (or raw with `text/markdown`) | Yes               |
| `.html`, `.htm`                | HTML passthrough                              | Yes               |
| `.css`                         | CSS passthrough                               | No                |
| `.js`, `.mjs`                  | JavaScript passthrough                        | No                |
| `.json`                        | JSON passthrough                              | No                |
| `.png`, `.jpg`, `.gif`, `.svg` | Image passthrough                             | No                |
| `.pdf`                         | PDF passthrough                               | No                |
| All others                     | Binary passthrough                            | No                |

A directory serves its `default_index` file (`README.md` by default). Without one, and with `dir_index: false` (the
default), it redirects (302) to the first page of the navigation tree, or answers 403 Forbidden when there is none.
With `dir_index: true` it renders a generated directory listing.

```bash
curl http://localhost:8080/                          # README.md rendered as HTML
curl http://localhost:8080/docs/guide                # docs/guide.md rendered as HTML
curl -i http://localhost:8080/docs/guide.md          # 301 to /docs/guide
curl -H "Accept: text/markdown" http://localhost:8080/docs/guide   # raw Markdown source
curl http://localhost:8080/assets/style.css          # CSS passthrough
curl "http://localhost:8080/api/search?q=install"    # Full-text search (JSON)
```

## A directory URL redirects or answers 403

A directory without its `default_index` file redirects to the first page, or answers 403 when there is none. Enable
generated listings:

```bash
GOMDDOC_SITE_DIR_INDEX=true gomddoc serve
```

## Images, CSS or JS don't load

Use relative paths in your HTML and Markdown:

```markdown
![Logo](./images/logo.png)

<link rel="stylesheet" href="./assets/style.css">
```

## 406 Not Acceptable

The server can't provide the format the `Accept` header asks for:

```bash
# Fails: a Markdown file cannot be served as JSON
curl -H "Accept: application/json" http://localhost:8080/docs

# */* accepts any format
curl -H "Accept: */*" http://localhost:8080/docs
```

See [HTTP Behavior](12-advanced/01-http-behavior.md) for the negotiation rules.

## The port is already in use

```bash
gomddoc serve -p :8081
gomddoc serve -p :auto   # first free port from 8080
```

## Permission denied

gomddoc needs to read every file and list every directory it serves:

```bash
chmod 644 *.md
chmod 755 $(find . -type d)
```

## Edits don't show up

Outside dev mode, templates and rendered pages are cached. Use dev mode while writing:

```bash
gomddoc preview        # dev mode: render cache off, templates re-parsed on every request
GOMDDOC_SERVER_DEV_MODE=true gomddoc serve
```

There is no file watcher. Dev mode re-reads content and templates on every request, so editing an existing page and
refreshing is enough, but adding, renaming or deleting a file needs a restart: the path resolver, navigation tree,
metadata index and search index are built once at startup.
