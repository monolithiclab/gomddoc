---
title: "MCP Server"
description: "Expose documentation to AI models via the Model Context Protocol (MCP)."
author: "nicolasm"
tags: ["mcp", "ai", "tools"]
---

# MCP Server

gomddoc includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server
that exposes your documentation to AI models. Claude, GPT, Copilot, and other MCP-compatible clients
can directly search, read, navigate, and query your docs without scraping HTML.

## What is MCP?

The Model Context Protocol is an open standard for connecting AI models to external data sources and
tools. Instead of copy-pasting documentation into a chat window, MCP lets AI models programmatically
access your content through a structured interface.

gomddoc's MCP server is a thin adapter over the same internals used by the HTTP server — the same
provider, metadata index, search index, and navigation system. No new parsing or indexing is needed.

## Quick Start

### stdio Transport (Local)

Run the MCP server over stdin/stdout for local AI clients:

```bash
gomddoc mcp ./docs
```

This starts an MCP server that reads documentation from `./docs` and speaks the MCP protocol over
stdio. The server runs until interrupted (Ctrl+C).

### Configure Claude Desktop

Add gomddoc to your `claude_desktop_config.json`:

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

Restart Claude Desktop. Your documentation tools will appear in the tool list.

### Configure Cursor / Windsurf

For Cursor, add to `.cursor/mcp.json` in your project:

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

### Configure Claude Code

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

gomddoc MCP works with remote Git repositories, just like `gomddoc serve`:

```bash
gomddoc mcp https://github.com/org/docs.git
```

For private repositories, provide an SSH key:

```bash
gomddoc mcp git@github.com:org/docs.git --git-key-file ~/.ssh/id_ed25519
```

## Available Tools

All tools are read-only and idempotent — safe for AI auto-approval.

### search_docs

Full-text search across all documentation pages. Returns ranked results with snippets.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query text |
| `limit` | int | No | Maximum results (default: 20, max: 100) |

**Example prompt:** "Search the docs for how to configure themes"

### read_page

Read a documentation page with metadata. Returns clean markdown with frontmatter stripped and
metadata (title, description, tags) prepended.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path (e.g., `guide/configuration.md`) |

**Example prompt:** "Read the deployment guide"

### read_section

Read a specific section of a page by heading anchor ID. Returns markdown from the heading to the
next heading at the same or higher level. This enables targeted retrieval without loading entire
documents.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path |
| `heading_id` | string | Yes | Heading anchor ID (e.g., `installation`) |

The heading ID follows the same slugification as HTML anchors: lowercase, non-alphanumeric
characters replaced with hyphens, consecutive hyphens collapsed.

**Example prompt:** "Read just the Installation section from the quickstart guide"

### list_pages

List documentation pages with metadata. Optionally filter by tag.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tag` | string | No | Filter by tag |
| `limit` | int | No | Maximum pages (default: 50, max: 200) |

**Example prompt:** "List all pages tagged with 'deployment'"

### get_table_of_contents

Get the site-wide navigation tree showing all pages organized by directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Subtree root path (default: entire site) |

**Example prompt:** "Show me the table of contents for this documentation"

### find_related

Find documentation pages related to a given page based on shared tags.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page path to find related documents for |

**Example prompt:** "What pages are related to the configuration guide?"

## Available Resources

Resources use the `docs://` URI scheme for semantic identification.

### Static Resources

| URI | Description |
|-----|-------------|
| `docs://site/index` | JSON array of all pages with path, title, description, and tags |
| `docs://site/tags` | JSON array of all unique tags |

### Resource Templates

| URI Template | Description |
|-------------|-------------|
| `docs://site/page/{+path}` | Read a page by path (clean markdown, frontmatter stripped) |
| `docs://site/tag/{tag}` | JSON array of pages with a specific tag |

## Available Prompts

Prompt templates provide pre-built interaction patterns.

### explain_concept

Searches the documentation for relevant content and asks the model to explain a concept using the
docs as source material.

| Argument | Required | Description |
|----------|----------|-------------|
| `concept` | Yes | The concept to explain |

### troubleshoot

Searches for relevant documentation and guides the model through step-by-step troubleshooting.

| Argument | Required | Description |
|----------|----------|-------------|
| `issue` | Yes | Description of the issue |
| `error_message` | No | Error message if available |

### summarize_page

Reads a documentation page and asks the model for a concise summary with key topics and actionable
takeaways.

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path to the documentation page |

## CLI Reference

```
Usage: gomddoc mcp [<dir>] [flags]

Start MCP server for AI model integration (stdio transport).

Arguments:
  [<dir>]    Markdown directory or Git URL (default: ".")

Flags:
  --git-key-file=STRING    Path to SSH private key file for Git authentication ($GOMDDOC_SERVER_GIT_SSH_KEY)
```

## Architecture

The MCP server reuses gomddoc's existing internals as a thin adapter:

```
MCP Client (Claude, Cursor, etc.)
    ↕ JSON-RPC (stdio or Streamable HTTP)
internal/mcp/server.go
    ↓ adapts to existing interfaces
Provider → Metadata Index → Search Index → Navigation
```

No new parsing, rendering, or indexing logic is introduced. The MCP package calls the same
`Provider.ReadFile()`, `MetaIndex.AllPages()`, `SearchIndex.Search()`, and
`navigation.Generator.Generate()` methods used by the HTTP server.

## Tips for AI Users

- **Start with the TOC.** Use `get_table_of_contents` to understand the documentation structure
  before diving into specific pages.
- **Use section-level reads.** `read_section` returns just the content under a specific heading,
  saving tokens compared to reading entire pages.
- **Search before reading.** `search_docs` uses TF-IDF ranking with title boosts — it will surface
  the most relevant pages for your query.
- **Discover via tags.** Use `list_pages` with a tag filter to find all pages on a topic, then
  `find_related` to explore connections between documents.
