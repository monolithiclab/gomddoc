package enricher

import "go.abhg.dev/goldmark/toc"

// ConvertTOC converts goldmark-toc Items to the internal TOCNode structure.
func ConvertTOC(items toc.Items) *TOCNode {
	if len(items) == 0 {
		return nil
	}

	root := &TOCNode{
		Level:    0,
		Children: make([]*TOCNode, len(items)),
	}

	for i, item := range items {
		root.Children[i] = convertItem(item, 1)
	}

	return root
}

func convertItem(item *toc.Item, level int) *TOCNode {
	node := &TOCNode{
		Level: level,
		Text:  string(item.Title),
		ID:    string(item.ID),
	}

	if len(item.Items) > 0 {
		node.Children = make([]*TOCNode, 0, len(item.Items))
		for _, child := range item.Items {
			node.Children = append(node.Children, convertItem(child, level+1))
		}
	}

	return node
}
