package provider

import (
	"io/fs"
	"net/url"
	"path"
	"slices"
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
	slices.SortFunc(visible, func(a, b fs.DirEntry) int {
		aIsDir := a.IsDir()
		bIsDir := b.IsDir()
		if aIsDir != bIsDir {
			if aIsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name(), b.Name())
	})

	// Build markdown listing
	var builder strings.Builder

	// Add heading (dirPath is internal, but escape for safety)
	if dirPath == "" || dirPath == "." || dirPath == "/" {
		builder.WriteString("# Index\n\n")
	} else {
		builder.WriteString("# Index of ")
		builder.WriteString(escapeMarkdown(dirPath))
		builder.WriteString("\n\n")
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
				safeName += "/"
				safeURL += "/"
			}
			builder.WriteString("- [")
			builder.WriteString(safeName)
			builder.WriteString("](")
			builder.WriteString(safeURL)
			builder.WriteString(")\n")
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
