package navigation

import (
	"slices"
	"testing"
)

func TestFlattenPages(t *testing.T) {
	t.Parallel()

	root := &NavNode{
		Label: "Root",
		Path:  "/",
		IsDir: true,
		Children: []*NavNode{
			{Label: "Intro", Path: "/intro.md"},
			{Label: "Guide", Path: "/guide/", IsDir: true, Children: []*NavNode{
				{Label: "Setup", Path: "/guide/setup.md"},
				{Label: "Usage", Path: "/guide/usage.md"},
			}},
			{Label: "FAQ", Path: "/faq.md"},
		},
	}

	got := FlattenPages(root)
	want := []string{"/intro.md", "/guide/setup.md", "/guide/usage.md", "/faq.md"}

	if !slices.Equal(got, want) {
		t.Errorf("FlattenPages = %v, want %v", got, want)
	}
}

func TestFlattenPages_Nil(t *testing.T) {
	t.Parallel()

	got := FlattenPages(nil)
	if got != nil {
		t.Errorf("FlattenPages(nil) = %v, want nil", got)
	}
}
