package provider

import (
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strings"
)

// GenerateMarkdownListing creates a markdown directory listing from fs.DirEntry items.
//
// The listing includes:
//   - Heading with the directory path
//   - Sorted list of files and directories (directories first, then alphabetically)
//   - Hidden files (starting with '.') are automatically excluded
//   - Directory entries shown with trailing '/'
//   - All entries are hyperlinked
//
// The output is valid markdown that can be processed by MarkdownRenderer.
func GenerateMarkdownListing(dirPath string, entries []fs.DirEntry, excludePatterns []string) []byte {
	// Filter out hidden and excluded files
	visible := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		entryPath := path.Join(dirPath, name)
		if IsRestrictedPath(entryPath, excludePatterns) {
			continue
		}
		visible = append(visible, entry)
	}

	// Sort: directories first, then alphabetically within each group
	sort.Slice(visible, func(i, j int) bool {
		iIsDir := visible[i].IsDir()
		jIsDir := visible[j].IsDir()

		// If one is a directory and the other isn't, directory comes first
		if iIsDir != jIsDir {
			return iIsDir
		}

		// Both are same type (both dirs or both files), sort alphabetically
		return visible[i].Name() < visible[j].Name()
	})

	// Build markdown listing
	var builder strings.Builder

	// Add heading (dirPath is internal, but escape for safety)
	if dirPath == "" || dirPath == "." || dirPath == "/" {
		builder.WriteString("# Index\n\n")
	} else {
		builder.WriteString(fmt.Sprintf("# Index of %s\n\n", escapeMarkdown(dirPath)))
	}

	// Add entries
	if len(visible) == 0 {
		builder.WriteString("*This directory is empty.*\n")
	} else {
		for _, entry := range visible {
			name := entry.Name()
			safeName := escapeMarkdown(name)
			safeURL := url.PathEscape(name)
			if entry.IsDir() {
				// Directory: add trailing slash and link to directory path
				builder.WriteString(fmt.Sprintf("- [%s/](%s/)\n", safeName, safeURL))
			} else {
				// File: link to file
				builder.WriteString(fmt.Sprintf("- [%s](%s)\n", safeName, safeURL))
			}
		}
	}

	return []byte(builder.String())
}

// escapeMarkdown escapes markdown special characters in text to prevent injection.
func escapeMarkdown(s string) string {
	r := strings.NewReplacer(
		"[", `\[`,
		"]", `\]`,
		"(", `\(`,
		")", `\)`,
	)
	return r.Replace(s)
}
