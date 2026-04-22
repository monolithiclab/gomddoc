package navigation

// FlattenPages returns all leaf (non-directory) page paths from the
// navigation tree in depth-first order. This produces the sequential
// reading order used for next/previous page links.
func FlattenPages(root *NavNode) []string {
	if root == nil {
		return nil
	}
	var pages []string
	flattenDFS(root, &pages)
	return pages
}

func flattenDFS(node *NavNode, pages *[]string) {
	for _, child := range node.Children {
		if child.IsDir {
			flattenDFS(child, pages)
		} else {
			*pages = append(*pages, child.Path)
		}
	}
}

// FindPrevNext locates the current page in the navigation tree and returns
// the paths of the previous and next pages in sequential order.
// Returns empty strings at the boundaries (first page has no prev,
// last page has no next).
func FindPrevNext(root *NavNode, currentPath string) (prev, next string) {
	pages := FlattenPages(root)
	normalized := cleanPath(currentPath)

	for i, p := range pages {
		if cleanPath(p) == normalized {
			if i > 0 {
				prev = pages[i-1]
			}
			if i < len(pages)-1 {
				next = pages[i+1]
			}
			return prev, next
		}
	}

	return "", ""
}
