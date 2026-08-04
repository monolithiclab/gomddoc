package enricher

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

// buildSmallDoc returns a minimal markdown document (~50 bytes) with frontmatter and a heading.
func buildSmallDoc() []byte {
	return []byte(`---
title: Hello
---
# Hello World
`)
}

// buildMediumDoc returns a markdown document (~2KB) with frontmatter, multiple headings, and paragraphs.
func buildMediumDoc() []byte {
	return []byte(`---
title: Getting Started
description: A guide to getting started with the project
tags:
  - guide
  - tutorial
  - quickstart
author: Jane Doe
date: 2026-01-15
---

# Getting Started

Welcome to the project. This guide walks you through the initial setup and configuration.

## Prerequisites

Before you begin, make sure you have the following tools installed on your system.
You will need Go 1.25 or later, a modern terminal, and access to a Git repository.

## Installation

Install the binary using the standard Go toolchain. Run the following command
in your terminal to download and build the latest version.

### From Source

Clone the repository and build from source. This gives you access to the latest
development features and allows you to contribute back to the project.

### Using Go Install

The simplest method is to use go install, which downloads, builds, and places
the binary in your GOPATH/bin directory automatically.

## Configuration

Configuration follows a layered precedence model. CLI flags take the highest
priority, followed by environment variables, then the config file, and finally
built-in defaults.

### Config File

Create a config file in the .gomddoc directory at the root of your project.
The file uses YAML format and supports all configuration options.

### Environment Variables

All configuration options can be set via environment variables. Use the prefix
GOMDDOC_ followed by the uppercase option name with underscores.

## First Steps

Once installed and configured, you can start serving your documentation locally
by running the serve command. This starts an HTTP server on the default port.
`)
}

// buildLargeDoc returns a rich markdown document (~20KB) with deep TOC, GFM features, and varied content.
func buildLargeDoc() []byte {
	var sb strings.Builder
	sb.WriteString(`---
title: Complete API Reference
description: Full reference documentation for the public API surface
tags:
  - api
  - reference
  - advanced
  - v2
author: Documentation Team
date: 2026-03-01
version: "2.0"
status: stable
---

# Complete API Reference

This document provides a comprehensive reference for the entire public API surface.
All endpoints, types, and behaviors are documented here.

`)

	// Generate sections with subsections to create a deep TOC and reach ~20KB.
	sections := []struct {
		title       string
		subsections []string
	}{
		{"Authentication", []string{"OAuth2 Flow", "API Keys", "JWT Tokens", "Session Management"}},
		{"Resources", []string{"Users", "Projects", "Documents", "Tags", "Comments"}},
		{"Query Parameters", []string{"Filtering", "Sorting", "Pagination", "Field Selection"}},
		{"Error Handling", []string{"Error Codes", "Retry Logic", "Rate Limiting", "Validation Errors"}},
		{"Webhooks", []string{"Event Types", "Payload Format", "Delivery Guarantees", "Security"}},
		{"Data Types", []string{"Timestamps", "Identifiers", "Enumerations", "Nested Objects"}},
		{"Performance", []string{"Caching", "Compression", "Batch Operations", "Connection Pooling"}},
	}

	for i, sec := range sections {
		fmt.Fprintf(&sb, "## %d. %s\n\n", i+1, sec.title)
		sb.WriteString("This section covers all aspects of " + strings.ToLower(sec.title) +
			" in the API. Understanding these concepts is essential for building reliable integrations.\n\n")

		// Add a GFM table.
		sb.WriteString("| Parameter | Type | Required | Description |\n")
		sb.WriteString("|-----------|------|----------|-------------|\n")
		for j, sub := range sec.subsections {
			fmt.Fprintf(&sb, "| %s | string | %v | Configuration for %s |\n",
				strings.ToLower(strings.ReplaceAll(sub, " ", "_")),
				j == 0,
				strings.ToLower(sub))
		}
		sb.WriteString("\n")

		for j, sub := range sec.subsections {
			fmt.Fprintf(&sb, "### %d.%d %s\n\n", i+1, j+1, sub)

			// Paragraph content.
			sb.WriteString("The " + strings.ToLower(sub) + " feature provides a robust mechanism " +
				"for handling complex scenarios in production environments. It supports both " +
				"synchronous and asynchronous operation modes.\n\n")

			// Code block (GFM fenced).
			sb.WriteString("```go\n")
			fmt.Fprintf(&sb, "// Example: configure %s\n", strings.ToLower(sub))
			fmt.Fprintf(&sb, "client.Set%s(ctx, %sOptions{\n", strings.ReplaceAll(sub, " ", ""), strings.ReplaceAll(sub, " ", ""))
			sb.WriteString("    Enabled:  true,\n")
			sb.WriteString("    Timeout:  30 * time.Second,\n")
			sb.WriteString("    Retries:  3,\n")
			sb.WriteString("})\n")
			sb.WriteString("```\n\n")

			// GFM task list.
			sb.WriteString("Implementation checklist:\n\n")
			sb.WriteString("- [x] Basic functionality\n")
			sb.WriteString("- [x] Error handling\n")
			sb.WriteString("- [ ] Performance optimization\n")
			sb.WriteString("- [ ] Documentation\n\n")

			// Blockquote.
			fmt.Fprintf(&sb, "> **Note**: The %s API is available starting from version 2.0. "+
				"Ensure your client library is up to date before using these endpoints.\n\n", strings.ToLower(sub))
		}
	}

	return []byte(sb.String())
}

func BenchmarkMarkdownEnricher_Enrich(b *testing.B) {
	smallDoc := buildSmallDoc()
	mediumDoc := buildMediumDoc()
	largeDoc := buildLargeDoc()

	tests := []struct {
		name    string
		content []byte
	}{
		{"small", smallDoc},
		{"medium", mediumDoc},
		{"large", largeDoc},
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = e.Enrich(context.Background(), tt.content, "test.md")
			}
		})
	}
}

// buildTaggedIndex returns a metadata index of n pages that all carry the
// "common" tag, plus one page-local tag each. It models the corpus shape that
// makes related-docs expensive: a tag shared by the whole site.
func buildTaggedIndex(tb testing.TB, n int) *metadata.Index {
	tb.Helper()
	fsys := fstest.MapFS{}
	for i := range n {
		fsys[fmt.Sprintf("page%04d.md", i)] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "---\ntitle: Page %04d\ntags: [common, t%04d]\n---\n# Page\n", i, i),
		}
	}
	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		tb.Fatalf("BuildIndex() error = %v", err)
	}
	return idx
}

// BenchmarkMarkdownEnricher_RelatedDocs measures enrichment against corpora
// where every page shares one tag. Before REVIEW §10.4, cost grew linearly with
// the corpus: ByTag materialized a full []PageInfo per tag and the candidate
// list was sorted whole, then truncated to ten. The scaling here is the point —
// the 2000-page run should not be far off the 100-page one.
func BenchmarkMarkdownEnricher_RelatedDocs(b *testing.B) {
	doc := []byte("---\ntitle: Current\ntags: [common]\n---\n# Current\n\nBody text.\n")

	sizes := []int{0, 100, 500, 2000}
	for _, n := range sizes {
		name := fmt.Sprintf("%d-pages", n)
		var idx *metadata.Index
		if n > 0 {
			idx = buildTaggedIndex(b, n)
		} else {
			name = "baseline-no-index"
		}

		b.Run(name, func(b *testing.B) {
			e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})
			b.ReportAllocs()
			for b.Loop() {
				_, _ = e.Enrich(context.Background(), doc, "/current.md")
			}
		})
	}
}
