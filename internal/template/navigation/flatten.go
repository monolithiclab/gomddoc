package navigation

// FlattenPages returns all leaf (non-directory) page paths from the
// navigation tree in depth-first order.
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
