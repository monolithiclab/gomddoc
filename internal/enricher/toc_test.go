package enricher

import (
	"testing"

	"go.abhg.dev/goldmark/toc"
)

func TestConvertTOC(t *testing.T) {
	tests := []struct {
		name     string
		items    toc.Items
		wantNil  bool
		validate func(t *testing.T, root *TOCNode)
	}{
		{
			name:    "nil items",
			items:   nil,
			wantNil: true,
		},
		{
			name:    "empty items",
			items:   toc.Items{},
			wantNil: true,
		},
		{
			name: "single heading",
			items: toc.Items{
				{Title: []byte("Introduction"), ID: []byte("introduction")},
			},
			validate: func(t *testing.T, root *TOCNode) {
				if root.Level != 0 {
					t.Errorf("root.Level = %d, want 0", root.Level)
				}
				if len(root.Children) != 1 {
					t.Fatalf("root has %d children, want 1", len(root.Children))
				}
				child := root.Children[0]
				if child.Text != "Introduction" {
					t.Errorf("child.Text = %q, want 'Introduction'", child.Text)
				}
				if child.ID != "introduction" {
					t.Errorf("child.ID = %q, want 'introduction'", child.ID)
				}
				if child.Level != 1 {
					t.Errorf("child.Level = %d, want 1", child.Level)
				}
			},
		},
		{
			name: "deeply nested headings",
			items: toc.Items{
				{
					Title: []byte("L1"), ID: []byte("l1"),
					Items: toc.Items{
						{
							Title: []byte("L2"), ID: []byte("l2"),
							Items: toc.Items{
								{
									Title: []byte("L3"), ID: []byte("l3"),
									Items: toc.Items{
										{Title: []byte("L4"), ID: []byte("l4")},
									},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, root *TOCNode) {
				if len(root.Children) != 1 {
					t.Fatalf("root has %d children, want 1", len(root.Children))
				}

				l1 := root.Children[0]
				if l1.Level != 1 || l1.Text != "L1" {
					t.Errorf("L1: Level=%d Text=%q", l1.Level, l1.Text)
				}

				if len(l1.Children) != 1 {
					t.Fatalf("L1 has %d children, want 1", len(l1.Children))
				}
				l2 := l1.Children[0]
				if l2.Level != 2 || l2.Text != "L2" {
					t.Errorf("L2: Level=%d Text=%q", l2.Level, l2.Text)
				}

				if len(l2.Children) != 1 {
					t.Fatalf("L2 has %d children, want 1", len(l2.Children))
				}
				l3 := l2.Children[0]
				if l3.Level != 3 || l3.Text != "L3" {
					t.Errorf("L3: Level=%d Text=%q", l3.Level, l3.Text)
				}

				if len(l3.Children) != 1 {
					t.Fatalf("L3 has %d children, want 1", len(l3.Children))
				}
				l4 := l3.Children[0]
				if l4.Level != 4 || l4.Text != "L4" {
					t.Errorf("L4: Level=%d Text=%q", l4.Level, l4.Text)
				}
			},
		},
		{
			name: "multiple siblings",
			items: toc.Items{
				{Title: []byte("First"), ID: []byte("first")},
				{Title: []byte("Second"), ID: []byte("second")},
				{Title: []byte("Third"), ID: []byte("third")},
			},
			validate: func(t *testing.T, root *TOCNode) {
				if len(root.Children) != 3 {
					t.Fatalf("root has %d children, want 3", len(root.Children))
				}
				for i, want := range []string{"First", "Second", "Third"} {
					if root.Children[i].Text != want {
						t.Errorf("child[%d].Text = %q, want %q", i, root.Children[i].Text, want)
					}
				}
			},
		},
		{
			name: "item with no children has nil Children slice",
			items: toc.Items{
				{Title: []byte("Leaf"), ID: []byte("leaf")},
			},
			validate: func(t *testing.T, root *TOCNode) {
				leaf := root.Children[0]
				if leaf.Children != nil {
					t.Errorf("leaf.Children = %v, want nil", leaf.Children)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertTOC(tt.items)
			if tt.wantNil {
				if result != nil {
					t.Errorf("ConvertTOC() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("ConvertTOC() returned nil, want non-nil")
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
