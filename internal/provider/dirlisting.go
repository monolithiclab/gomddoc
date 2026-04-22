package provider

import (
	"fmt"
	"io/fs"
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
func GenerateMarkdownListing(path string, entries []fs.DirEntry) []byte {
	// Filter out hidden files and prepare sorted entries
	visible := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			visible = append(visible, entry)
		}
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

	// Add heading
	if path == "" || path == "." || path == "/" {
		builder.WriteString("# Index\n\n")
	} else {
		builder.WriteString(fmt.Sprintf("# Index of %s\n\n", path))
	}

	// Add entries
	if len(visible) == 0 {
		builder.WriteString("*This directory is empty.*\n")
	} else {
		for _, entry := range visible {
			name := entry.Name()
			if entry.IsDir() {
				// Directory: add trailing slash and link to directory path
				builder.WriteString(fmt.Sprintf("- [%s/](%s/)\n", name, name))
			} else {
				// File: link to file
				builder.WriteString(fmt.Sprintf("- [%s](%s)\n", name, name))
			}
		}
	}

	return []byte(builder.String())
}
