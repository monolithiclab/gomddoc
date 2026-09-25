---
title: "MCP Server"
description: "Expose documentation to AI models via the Model Context Protocol (MCP)."
author: "nicolasm"
tags: ["mcp", "ai", "tools"]
---

# MCP Server

gomddoc includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server
that exposes your documentation to AI models. Instead of copy-pasting content into chat windows or
pointing models at URLs they cannot fetch, MCP gives them direct, structured access to your docs
through a standard protocol.

Claude, GPT, Copilot, and any MCP-compatible client can search your documentation, read specific
pages or sections, browse the navigation tree, and discover related content — all programmatically,
with no scraping or browser automation.

## How It Works

The MCP server is a thin adapter over the same internals used by the HTTP server. When you run
`gomddoc mcp`, it loads the site configuration, builds the metadata index, search index and
navigation tree from your markdown files, then exposes them via the MCP protocol over standard
input/output. There is no new parsing or indexing logic — the MCP server calls the same `Provider`,
`MetaIndex`, `SearchIndex`, and navigation components that power `gomddoc serve`.

The indexes are built once at startup. Restart the server to pick up added, removed or retitled
pages; `read_page` and `read_section` read the file on every call.

This means your AI tools see the exact same content, search results, and navigation structure as
your human readers. Frontmatter metadata (titles, descriptions, tags) is used to enrich tool
responses, and the full-text search uses the same TF-IDF ranking with title and description boosts.

## Quick Start

### Running the MCP Server

Start the server over stdin/stdout, pointing it at your documentation directory:

```bash
gomddoc mcp ./docs
```

The server speaks MCP JSON-RPC over stdio and runs until you interrupt it with Ctrl+C. This is the
transport used by local AI clients — the client launches gomddoc as a subprocess and communicates
through its stdin/stdout streams.

### Configuring Claude Desktop

Add gomddoc to your Claude Desktop configuration file (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "my-docs": {
      "command": "gomddoc",
      "args": ["mcp", "/path/to/your/docs"]
    }
  }
}
```

After restarting Claude Desktop, your documentation tools will appear in the tool list. You can
then ask Claude to search your docs, read specific pages, or explain concepts using your
documentation as source material.

### Configuring Cursor / Windsurf

For Cursor, add a `.cursor/mcp.json` file in your project root:

```json
{
  "mcpServers": {
    "docs": {
      "command": "gomddoc",
      "args": ["mcp", "."]
    }
  }
}
```

This gives Cursor's AI assistant direct access to your project documentation while you code.

### Configuring Claude Code

Add to your `.mcp.json` or project settings:

```json
{
  "mcpServers": {
    "docs": {
      "command": "gomddoc",
      "args": ["mcp", "./docs"]
    }
  }
}
```

### Serving from a Git Repository

The MCP server supports the same content sources as `gomddoc serve`, including remote Git
repositories. This is useful for giving AI models access to documentation that lives in a separate
repository:

```bash
gomddoc mcp "git+https://github.com/org/docs.git"
```

For private repositories, use a `git+ssh://` URL and provide an SSH key:

```bash
gomddoc mcp "git+ssh://git@github.com/org/docs.git" --git-key-file ~/.ssh/id_ed25519
```

See [Content Sources](03-content-sources.md) for the URL syntax, branch and subdirectory selection.

## Streamable HTTP Transport

In addition to the stdio transport used by `gomddoc mcp`, the MCP server is also available over
HTTP when running `gomddoc serve` or `gomddoc preview`. The endpoint is mounted at `/_mcp/` and
uses the MCP Streamable HTTP transport — the same protocol, but over HTTP instead of stdin/stdout.

This enables remote MCP clients to connect to a running gomddoc instance without launching a
subprocess. The endpoint is protected by the same authentication middleware as other endpoints
(when `--basic-auth-file` is configured), and request bodies are capped at 1 MiB.

```
POST /_mcp/   → MCP JSON-RPC over Streamable HTTP
```

No additional configuration is needed — the `/_mcp/` endpoint is always available when the HTTP
server is running. It serves the site tools, resources and prompts below, but not the
[self-documentation](#self-documentation) namespace. On a multi-language site it covers the default
language: `list_pages`, `search_docs` and `get_table_of_contents` do not list pages under language
directories, though `read_page` still reads them by path (e.g. `fr-FR/guide.md`).

## Available Tools

gomddoc exposes six site tools through the MCP protocol (plus three self-documentation tools on
stdio, below). All tools are annotated as read-only and idempotent, which signals to MCP clients
that they are safe for automatic approval.

Every tool returns markdown text. A missing page, an empty argument or an unavailable index comes
back as a normal text result (e.g. `Page not found: guide.md`), not as a tool error. Paths are file
paths relative to the content root, with their extension (`guide/configuration.md`, not the clean
URL `/guide/configuration`); a leading `/` is accepted. Hidden and `exclude`d paths answer
`Page not found`, as they do over HTTP. A path, heading ID or tag longer than 1024 bytes is
treated as not found.

### search_docs

Full-text search across all documentation pages, using the same TF-IDF ranking engine as the
web search UI. Returns ranked results with path, title, description, a contextual snippet and a
relevance score.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query text |
| `limit` | int | No | Maximum results to return (default: 20, max: 100) |

Queries use AND semantics — all terms must appear in a document for it to match. Title matches
are boosted 3x and description matches 1.5x over body content.

A `tag:<name>` token filters by frontmatter tag (`tag:deployment kubernetes`). Several `tag:`
tokens must all match. A query made only of `tag:` tokens lists the tagged pages alphabetically by
title, without scores. When `search.index` is `false`, the tool answers `Search index is not
available.`

### read_page

Reads a documentation page and returns its content as clean markdown with YAML frontmatter stripped.
When the page is in the metadata index, a YAML header block (`title`, `description`, `tags`) is
prepended so the model has full context about the page.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path (e.g., `guide/configuration.md`) |

A directory path returns its `default_index` file (`guide/` reads `guide/README.md`).

### read_section

Reads a specific section of a page by heading anchor ID. Instead of loading an entire document, the
model can request only the content under a particular heading. The tool returns everything from the
matched heading to the next heading at the same or higher level (or the end of the file).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path |
| `heading_id` | string | Yes | Heading anchor ID (e.g., `installation`) |

Heading IDs follow the same slugification as HTML anchors: text is lowercased, non-alphanumeric
characters are replaced with hyphens, consecutive hyphens are collapsed, and leading/trailing
hyphens are trimmed. For example, "Getting Started" becomes `getting-started`. Only ATX headings
(`#` to `######`) count, lines inside fenced code blocks are skipped, and when two headings share
an ID the first one wins.

### list_pages

Lists all documentation pages with their title, path, description and tags, or filters by tag.
This gives the model an overview of what documentation is available before it decides which pages
to read.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tag` | string | No | Filter pages by tag (case-insensitive, surrounding spaces ignored) |
| `limit` | int | No | Maximum pages to return (default: 50, max: 200) |

### get_table_of_contents

Returns the site-wide navigation tree showing all pages organized by directory, as a nested
markdown list of labels and paths. This is the same tree that powers the navigation sidebar in the
web UI.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Accepted but ignored: the whole tree is always returned |

### find_related

Finds documentation pages related to a given page based on shared frontmatter tags. Every page
that shares at least one tag is listed once, with the tags it shares. This helps the model discover
connections between topics and suggest additional reading to users.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page path to find related documents for |

A page with no tags answers that related pages cannot be determined.

## Available Resources

In addition to tools, the MCP server exposes four resources using the `docs://` URI scheme.
Resources are data sources that MCP clients can read directly, without invoking a tool.

### Static Resources

| URI | Description |
|-----|-------------|
| `docs://site/index` | JSON array of all pages with `path`, `title`, `description`, `tags`, `date` and `meta` (the remaining frontmatter) |
| `docs://site/tags` | JSON array of all unique tags used across the documentation, sorted |

### Resource Templates

| URI Template | Description |
|-------------|-------------|
| `docs://site/page/{+path}` | Read a page by path — returns clean markdown with frontmatter stripped, without the header block `read_page` adds |
| `docs://site/tag/{tag}` | JSON array of pages tagged with a specific tag, sorted by title |

A page that does not exist (or is hidden or excluded) and a tag no page carries both return an MCP
resource-not-found error.

The `{+path}` syntax uses RFC 6570 reserved expansion, which means paths with forward slashes
(like `guide/configuration.md`) are preserved as-is in the URI.

## Available Prompts

Prompt templates provide pre-built interaction patterns that MCP clients can offer to users. Each
prompt gathers documentation content (search snippets from the top 5 results, or the whole page for
`summarize_page`) and constructs a message that guides the model to use your docs as source
material.

### explain_concept

Searches for documentation related to a concept and asks the model to explain it, citing specific
sections and providing examples from the docs.

| Argument | Required | Description |
|----------|----------|-------------|
| `concept` | Yes | The concept to explain |

### troubleshoot

Searches for documentation related to an issue (and optionally an error message) and asks the model
to provide step-by-step troubleshooting guidance based on what it finds.

| Argument | Required | Description |
|----------|----------|-------------|
| `issue` | Yes | Description of the issue |
| `error_message` | No | Error message if available |

### summarize_page

Reads a documentation page and asks the model for a concise summary including key topics, important
concepts, and actionable takeaways.

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path to the documentation page |

## Self-Documentation

`gomddoc mcp` also describes **gomddoc itself**, so an agent can learn how to configure and use it without external
documentation or network access. These live under the `gomddoc://` scheme and a `gomddoc_` prefix, separate from
your site's `docs://` content, so they never collide with your page names.

| Kind | Name | Returns |
|------|------|---------|
| Resource | `gomddoc://capabilities` | JSON report: every setting (config key, env var, flags, default, description), config precedence, commands, frontmatter fields, the active theme's feature toggles and CSS variables, guide pages |
| Resource | `gomddoc://schema/config` | JSON Schema for `.gomddoc/config.yml` |
| Resource template | `gomddoc://guide/{+path}` | A page of gomddoc's own guide (this guide), as markdown |
| Tool | `gomddoc_capabilities` | Same document as `gomddoc://capabilities` |
| Tool | `gomddoc_guide` | The embedded guide — see modes below |
| Tool | `gomddoc_doctor` | Check the served site's config and content, reloaded on every call; `verbose: true` adds info findings — see [Doctor](14-doctor.md) |
| Prompt | `learn_gomddoc` | Onboarding: overview, how configuration works, commands, where to go deeper |

`gomddoc_guide` picks its mode from the arguments you set:

- no arguments — list the guide's pages (`path`, `title`, `description`)
- `query` — search the guide; returns ranked `path`, `title`, `snippet`. `limit` caps the results (default 10,
  max 50)
- `path` — read a page (e.g. `02-configuration.md`, or its topic name, `configuration`)
- `path` + `section` — read one section by heading anchor (e.g. `priority-order`)

Mistakes come back as tool errors that carry the fix: an unknown page lists the valid paths, an unknown section
lists the page's heading anchors, and setting both `query` and `path`, or `section` without `path`, says which to
drop.

A typical agent flow: get `learn_gomddoc` → read `gomddoc://schema/config` → write `.gomddoc/config.yml` → call
`gomddoc_doctor` and fix what it reports. The same data is on the command line as `gomddoc info --json` and
`gomddoc schema`.

The theme's feature list comes from scanning its templates (`"source": "template-scan"`): names only. Their
descriptions are in [Theming & Assets](05-theming-and-assets.md).

**Only on stdio.** The Streamable HTTP endpoint that `serve` mounts at `/_mcp/` exposes your site to *its
readers'* agents and does not carry gomddoc's own manual.

## CLI Reference

```
Usage: gomddoc mcp [<dir>] [flags]

Start MCP server for AI model integration (stdio). Also serves gomddoc's own
guide and capabilities under gomddoc://.

Arguments:
  [<dir>]    Markdown directory or Git URL ($GOMDDOC_SERVER_DIR).

Flags:
      --git-key-file=""       Path to SSH private key file for Git
                              authentication ($GOMDDOC_SERVER_GIT_SSH_KEY).
      --git-storage-dir=""    Directory for disk-based Git clone
                              storage (default: in-memory)
                              ($GOMDDOC_SERVER_GIT_STORAGE_DIR).
```

The site configuration (`.gomddoc/config.yml` and the site environment variables) applies as it does for `serve`:
`exclude`, `default_index` and `search.index` shape what the tools see. Run `gomddoc info` for the full list of
environment variables.

## Tips for Effective Use

When using gomddoc's MCP server with an AI model, a few strategies help get the most out of the
integration:

**Start with the table of contents.** Before asking about specific topics, use `get_table_of_contents`
to understand how the documentation is organized. This gives the model a map of available content
and helps it make better decisions about which pages to read.

**Use section-level reads for large pages.** The `read_section` tool returns only the content under a
specific heading, which saves context window space compared to reading entire pages. This is
especially valuable for long reference documents where only one section is relevant.

**Search before reading.** The `search_docs` tool uses TF-IDF ranking with title boosts, so it
surfaces the most relevant pages for a query. Searching first and then reading the top results
is more efficient than guessing which page to read.

**Explore connections via tags.** Use `list_pages` with a tag filter to find all pages on a topic,
then `find_related` to discover pages that share tags with a document you have already read. This
helps the model build a broader understanding of a subject across multiple pages.
