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

func TestFindPrevNext_Middle(t *testing.T) {
	t.Parallel()

	root := buildTestTree()
	prev, next := FindPrevNext(root, "/guide/setup.md")

	if prev != "/intro.md" {
		t.Errorf("prev = %q, want %q", prev, "/intro.md")
	}
	if next != "/guide/usage.md" {
		t.Errorf("next = %q, want %q", next, "/guide/usage.md")
	}
}

func TestFindPrevNext_First(t *testing.T) {
	t.Parallel()

	root := buildTestTree()
	prev, next := FindPrevNext(root, "/intro.md")

	if prev != "" {
		t.Errorf("prev = %q, want empty", prev)
	}
	if next != "/guide/setup.md" {
		t.Errorf("next = %q, want %q", next, "/guide/setup.md")
	}
}

func TestFindPrevNext_Last(t *testing.T) {
	t.Parallel()

	root := buildTestTree()
	prev, next := FindPrevNext(root, "/faq.md")

	if prev != "/guide/usage.md" {
		t.Errorf("prev = %q, want %q", prev, "/guide/usage.md")
	}
	if next != "" {
		t.Errorf("next = %q, want empty", next)
	}
}

func TestFindPrevNext_NotFound(t *testing.T) {
	t.Parallel()

	root := buildTestTree()
	prev, next := FindPrevNext(root, "/nonexistent.md")

	if prev != "" || next != "" {
		t.Errorf("prev = %q, next = %q, want both empty", prev, next)
	}
}

func TestFindPrevNext_SinglePage(t *testing.T) {
	t.Parallel()

	root := &NavNode{
		Label: "Root",
		Path:  "/",
		IsDir: true,
		Children: []*NavNode{
			{Label: "Only", Path: "/only.md"},
		},
	}

	prev, next := FindPrevNext(root, "/only.md")

	if prev != "" || next != "" {
		t.Errorf("prev = %q, next = %q, want both empty", prev, next)
	}
}

func buildTestTree() *NavNode {
	return &NavNode{
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
}
