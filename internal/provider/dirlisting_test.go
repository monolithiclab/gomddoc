package provider

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestGenerateMarkdownListing(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		entries        []fs.DirEntry
		wantContains   []string
		wantNotContain []string
		wantOrder      []string // Check these appear in this order
	}{
		{
			name: "mixed files and directories",
			path: "docs",
			entries: []fs.DirEntry{
				mockDirEntry{name: "file1.md", isDir: false},
				mockDirEntry{name: "subdir", isDir: true},
				mockDirEntry{name: "file2.txt", isDir: false},
				mockDirEntry{name: "another-dir", isDir: true},
			},
			wantContains: []string{
				"# Index of docs",
				"[another-dir/](another-dir/)",
				"[subdir/](subdir/)",
				"[file1.md](file1.md)",
				"[file2.txt](file2.txt)",
			},
			wantOrder: []string{
				"another-dir/", // Directories first, alphabetically
				"subdir/",
				"file1.md", // Then files, alphabetically
				"file2.txt",
			},
		},
		{
			name: "hidden files excluded",
			path: "root",
			entries: []fs.DirEntry{
				mockDirEntry{name: ".hidden", isDir: false},
				mockDirEntry{name: ".git", isDir: true},
				mockDirEntry{name: "visible.md", isDir: false},
				mockDirEntry{name: "public-dir", isDir: true},
			},
			wantContains: []string{
				"[public-dir/](public-dir/)",
				"[visible.md](visible.md)",
			},
			wantNotContain: []string{
				".hidden",
				".git",
			},
		},
		{
			name:    "empty directory",
			path:    "empty",
			entries: []fs.DirEntry{},
			wantContains: []string{
				"# Index of empty",
				"*This directory is empty.*",
			},
		},
		{
			name: "only hidden files",
			path: "hidden-only",
			entries: []fs.DirEntry{
				mockDirEntry{name: ".dotfile", isDir: false},
				mockDirEntry{name: ".another", isDir: false},
			},
			wantContains: []string{
				"*This directory is empty.*",
			},
			wantNotContain: []string{
				".dotfile",
				".another",
			},
		},
		{
			name: "root path",
			path: "/",
			entries: []fs.DirEntry{
				mockDirEntry{name: "readme.md", isDir: false},
			},
			wantContains: []string{
				"# Index",
				"[readme.md](readme.md)",
			},
		},
		{
			name: "current directory",
			path: ".",
			entries: []fs.DirEntry{
				mockDirEntry{name: "file.txt", isDir: false},
			},
			wantContains: []string{
				"# Index",
				"[file.txt](file.txt)",
			},
		},
		{
			name: "empty path",
			path: "",
			entries: []fs.DirEntry{
				mockDirEntry{name: "doc.md", isDir: false},
			},
			wantContains: []string{
				"# Index",
				"[doc.md](doc.md)",
			},
		},
		{
			name: "only directories",
			path: "dirs-only",
			entries: []fs.DirEntry{
				mockDirEntry{name: "zulu", isDir: true},
				mockDirEntry{name: "alpha", isDir: true},
				mockDirEntry{name: "beta", isDir: true},
			},
			wantOrder: []string{
				"alpha/",
				"beta/",
				"zulu/",
			},
		},
		{
			name: "only files",
			path: "files-only",
			entries: []fs.DirEntry{
				mockDirEntry{name: "zebra.txt", isDir: false},
				mockDirEntry{name: "apple.md", isDir: false},
				mockDirEntry{name: "banana.json", isDir: false},
			},
			wantOrder: []string{
				"apple.md",
				"banana.json",
				"zebra.txt",
			},
		},
		{
			name: "special characters in names",
			path: "special",
			entries: []fs.DirEntry{
				mockDirEntry{name: "file with spaces.txt", isDir: false},
				mockDirEntry{name: "dir-with-dashes", isDir: true},
			},
			wantContains: []string{
				"[dir-with-dashes/](dir-with-dashes/)",
				"[file with spaces.txt](file with spaces.txt)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMarkdownListing(tt.path, tt.entries)
			resultStr := string(result)

			// Check contains
			for _, want := range tt.wantContains {
				if !strings.Contains(resultStr, want) {
					t.Errorf("GenerateMarkdownListing() missing %q\nGot:\n%s", want, resultStr)
				}
			}

			// Check not contains
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(resultStr, notWant) {
					t.Errorf("GenerateMarkdownListing() should not contain %q\nGot:\n%s", notWant, resultStr)
				}
			}

			// Check order
			if len(tt.wantOrder) > 0 {
				lastIndex := -1
				for i, item := range tt.wantOrder {
					index := strings.Index(resultStr, item)
					if index == -1 {
						t.Errorf("GenerateMarkdownListing() missing ordered item %q", item)
						continue
					}
					if index <= lastIndex {
						t.Errorf("GenerateMarkdownListing() item %d (%q) at index %d should appear after item %d (index %d)",
							i, item, index, i-1, lastIndex)
					}
					lastIndex = index
				}
			}
		})
	}
}

func TestGenerateMarkdownListing_ValidMarkdown(t *testing.T) {
	entries := []fs.DirEntry{
		mockDirEntry{name: "docs", isDir: true},
		mockDirEntry{name: "readme.md", isDir: false},
	}

	result := GenerateMarkdownListing("test", entries)
	resultStr := string(result)

	// Should start with heading
	if !strings.HasPrefix(resultStr, "# Index of test\n\n") {
		t.Errorf("GenerateMarkdownListing() should start with heading, got:\n%s", resultStr)
	}

	// Should contain markdown list syntax
	if !strings.Contains(resultStr, "- [") {
		t.Error("GenerateMarkdownListing() should contain markdown list items")
	}

	// Each entry should be on its own line with markdown link syntax
	lines := strings.Split(strings.TrimSpace(resultStr), "\n")
	foundListItems := false
	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			foundListItems = true
			// Check link format: - [text](url)
			if !strings.Contains(line, "](") || !strings.HasSuffix(line, ")") {
				t.Errorf("GenerateMarkdownListing() invalid link format: %s", line)
			}
		}
	}

	if !foundListItems {
		t.Error("GenerateMarkdownListing() should contain list items")
	}
}

func TestGenerateMarkdownListing_SortStability(t *testing.T) {
	// Test that sorting is stable and consistent
	entries := []fs.DirEntry{
		mockDirEntry{name: "file1.txt", isDir: false},
		mockDirEntry{name: "dir1", isDir: true},
		mockDirEntry{name: "file2.txt", isDir: false},
		mockDirEntry{name: "dir2", isDir: true},
		mockDirEntry{name: "file0.txt", isDir: false},
		mockDirEntry{name: "dir0", isDir: true},
	}

	// Run multiple times to ensure stability
	var firstResult string
	for i := 0; i < 5; i++ {
		result := string(GenerateMarkdownListing("test", entries))
		if i == 0 {
			firstResult = result
		} else if result != firstResult {
			t.Error("GenerateMarkdownListing() produces inconsistent results across runs")
		}
	}

	// Verify correct order
	result := string(GenerateMarkdownListing("test", entries))
	expectedOrder := []string{"dir0/", "dir1/", "dir2/", "file0.txt", "file1.txt", "file2.txt"}

	lastIndex := -1
	for _, item := range expectedOrder {
		index := strings.Index(result, item)
		if index == -1 {
			t.Errorf("GenerateMarkdownListing() missing item %q", item)
		} else if index <= lastIndex {
			t.Errorf("GenerateMarkdownListing() incorrect order for %q", item)
		}
		lastIndex = index
	}
}

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string {
	return m.name
}

func (m mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m mockDirEntry) Type() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}

func (m mockDirEntry) Info() (fs.FileInfo, error) {
	return mockFileInfo{
		name:  m.name,
		isDir: m.isDir,
		mode:  m.Type(),
	}, nil
}

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
	mode  fs.FileMode
}

func (m mockFileInfo) Name() string {
	return m.name
}

func (m mockFileInfo) Size() int64 {
	return 0
}

func (m mockFileInfo) Mode() fs.FileMode {
	return m.mode
}

func (m mockFileInfo) ModTime() time.Time {
	return time.Now()
}

func (m mockFileInfo) IsDir() bool {
	return m.isDir
}

func (m mockFileInfo) Sys() any {
	return nil
}
