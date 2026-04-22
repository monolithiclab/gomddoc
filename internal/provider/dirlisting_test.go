package provider

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestGenerateMarkdownListing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		entries        []fs.DirEntry
		wantContains   []string
		wantNotContain []string
		wantOrder      []string // Verify these items appear in this order
	}{
		{
			name: "mixed files and directories - sorted",
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
			wantNotContain: []string{".hidden", ".git"},
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
			name: "only hidden files shows empty",
			path: "hidden-only",
			entries: []fs.DirEntry{
				mockDirEntry{name: ".dotfile", isDir: false},
				mockDirEntry{name: ".another", isDir: false},
			},
			wantContains:   []string{"*This directory is empty.*"},
			wantNotContain: []string{".dotfile", ".another"},
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
		{
			name: "sorting stability - consistent across runs",
			path: "test",
			entries: []fs.DirEntry{
				mockDirEntry{name: "file1.txt", isDir: false},
				mockDirEntry{name: "dir1", isDir: true},
				mockDirEntry{name: "file2.txt", isDir: false},
				mockDirEntry{name: "dir2", isDir: true},
				mockDirEntry{name: "file0.txt", isDir: false},
				mockDirEntry{name: "dir0", isDir: true},
			},
			wantOrder: []string{"dir0/", "dir1/", "dir2/", "file0.txt", "file1.txt", "file2.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GenerateMarkdownListing(tt.path, tt.entries)
			resultStr := string(result)

			// Verify markdown structure
			if !strings.HasPrefix(resultStr, "#") {
				t.Error("Result should start with heading")
			}

			// Check contains
			for _, want := range tt.wantContains {
				if !strings.Contains(resultStr, want) {
					t.Errorf("missing %q\nGot:\n%s", want, resultStr)
				}
			}

			// Check not contains
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(resultStr, notWant) {
					t.Errorf("should not contain %q\nGot:\n%s", notWant, resultStr)
				}
			}

			// Check order
			if len(tt.wantOrder) > 0 {
				lastIndex := -1
				for i, item := range tt.wantOrder {
					index := strings.Index(resultStr, item)
					if index == -1 {
						t.Errorf("missing ordered item %q", item)
						continue
					}
					if index <= lastIndex {
						t.Errorf("item %d (%q) at index %d should appear after item %d (index %d)",
							i, item, index, i-1, lastIndex)
					}
					lastIndex = index
				}
			}

			// Verify markdown list format if not empty
			if len(tt.entries) > 0 && len(tt.wantNotContain) == 0 && !strings.Contains(resultStr, "empty") {
				if !strings.Contains(resultStr, "- [") {
					t.Error("Non-empty listing should contain markdown list items")
				}
			}
		})
	}
}

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() fs.FileMode          { return m.modeType() }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return mockFileInfo(m), nil }
func (m mockDirEntry) modeType() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo mockDirEntry

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return mockDirEntry(m).modeType() }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }
