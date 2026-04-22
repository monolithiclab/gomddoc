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
`gomddoc mcp`, it builds the metadata index and search index from your markdown files, then
exposes them via the MCP protocol over standard input/output. There is no new parsing or indexing
logic — the MCP server calls the same `Provider`, `MetaIndex`, `SearchIndex`, and `Navigation`
components that power `gomddoc serve`.

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
gomddoc mcp https://github.com/org/docs.git
```

For private repositories, provide an SSH key:

```bash
gomddoc mcp git@github.com:org/docs.git --git-key-file ~/.ssh/id_ed25519
```

## Streamable HTTP Transport

In addition to the stdio transport used by `gomddoc mcp`, the MCP server is also available over
HTTP when running `gomddoc serve` or `gomddoc preview`. The endpoint is mounted at `/_mcp/` and
uses the MCP Streamable HTTP transport — the same protocol, but over HTTP instead of stdin/stdout.

This enables remote MCP clients to connect to a running gomddoc instance without launching a
subprocess. The endpoint is protected by the same authentication middleware as other endpoints
(when `--basic-auth-file` is configured).

```
POST /_mcp/   → MCP JSON-RPC over Streamable HTTP
```

No additional configuration is needed — the `/_mcp/` endpoint is always available when the HTTP
server is running.

## Available Tools

gomddoc exposes six tools through the MCP protocol. All tools are annotated as read-only and
idempotent, which signals to MCP clients that they are safe for automatic approval — the AI model
does not need to ask permission before using them.

### search_docs

Full-text search across all documentation pages, using the same TF-IDF ranking engine as the
web search UI. Returns ranked results with contextual snippets, titles, and relevance scores.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query text |
| `limit` | int | No | Maximum results to return (default: 20, max: 100) |

Queries use AND semantics — all terms must appear in a document for it to match. Title matches
are boosted 3x and description matches 1.5x over body content.

### read_page

Reads a documentation page and returns its content as clean markdown with YAML frontmatter stripped.
When metadata is available from the index, a structured header block (title, description, tags) is
prepended so the model has full context about the page.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path (e.g., `guide/configuration.md`) |

### read_section

Reads a specific section of a page by heading anchor ID. This is the key differentiator — instead of
loading an entire document, the model can request just the content under a particular heading. The
tool returns everything from the matched heading to the next heading at the same or higher level
(or the end of the file).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page file path |
| `heading_id` | string | Yes | Heading anchor ID (e.g., `installation`) |

Heading IDs follow the same slugification as HTML anchors: text is lowercased, non-alphanumeric
characters are replaced with hyphens, consecutive hyphens are collapsed, and leading/trailing
hyphens are trimmed. For example, "Getting Started" becomes `getting-started`.

### list_pages

Lists all documentation pages with their metadata, or filters by tag. This gives the model an
overview of what documentation is available before it decides which pages to read.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tag` | string | No | Filter pages by tag |
| `limit` | int | No | Maximum pages to return (default: 50, max: 200) |

### get_table_of_contents

Returns the site-wide navigation tree showing all pages organized by directory. This is the same
tree that powers the navigation sidebar in the web UI. Optionally, pass a path to get a subtree
rooted at a specific directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | No | Subtree root path (default: entire site) |

### find_related

Finds documentation pages related to a given page based on shared frontmatter tags. This helps
the model discover connections between topics and suggest additional reading to users.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Page path to find related documents for |

## Available Resources

In addition to tools, the MCP server exposes four resources using the `docs://` URI scheme.
Resources are data sources that MCP clients can read directly, without invoking a tool.

### Static Resources

| URI | Description |
|-----|-------------|
| `docs://site/index` | JSON array of all pages with path, title, description, and tags |
| `docs://site/tags` | JSON array of all unique tags used across the documentation |

### Resource Templates

| URI Template | Description |
|-------------|-------------|
| `docs://site/page/{+path}` | Read a page by path — returns clean markdown with frontmatter stripped |
| `docs://site/tag/{tag}` | JSON array of pages tagged with a specific tag |

The `{+path}` syntax uses RFC 6570 reserved expansion, which means paths with forward slashes
(like `guide/configuration.md`) are preserved as-is in the URI.

## Available Prompts

Prompt templates provide pre-built interaction patterns that MCP clients can offer to users. Each
prompt searches the documentation for relevant content and constructs a message that guides the
model to use your docs as source material.

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

## CLI Reference

```
Usage: gomddoc mcp [<dir>] [flags]

Start MCP server for AI model integration (stdio transport).

Arguments:
  [<dir>]    Markdown directory or Git URL (default: ".")

Flags:
  --git-key-file=STRING    Path to SSH private key file for Git authentication
```

The `GOMDDOC_SERVER_DIR` and `GOMDDOC_SERVER_GIT_SSH_KEY` environment variables also apply to the
MCP subcommand. Run `gomddoc info` for the full list of environment variables.

## Tips for Effective Use

When using gomddoc's MCP server with an AI model, a few strategies help get the most out of the
integration:

**Start with the table of contents.** Before asking about specific topics, use `get_table_of_contents`
to understand how the documentation is organized. This gives the model a map of available content
and helps it make better decisions about which pages to read.

**Use section-level reads for large pages.** The `read_section` tool returns just the content under a
specific heading, which saves context window space compared to reading entire pages. This is
especially valuable for long reference documents where only one section is relevant.

**Search before reading.** The `search_docs` tool uses TF-IDF ranking with title boosts, so it
surfaces the most relevant pages for a query. Searching first and then reading the top results
is more efficient than guessing which page to read.

**Explore connections via tags.** Use `list_pages` with a tag filter to find all pages on a topic,
then `find_related` to discover pages that share tags with a document you have already read. This
helps the model build a broader understanding of a subject across multiple pages.
