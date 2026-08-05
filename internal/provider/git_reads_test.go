package provider

import (
	"io/fs"
	"strings"
	"testing"
)

// TestGitProvider_Stat_ReadsNoObjects pins the shape of Stat's object access:
// it answers from the tree entry plus the object header, never by decoding.
//
// Two regressions turn it red. Restoring the tree.File / tree.Tree probe costs
// a decode per case — File runs GetBlob before the type check can reject a
// subtree, for a Size the header already carries. Going back to
// tree.Size(cleanPath) costs a second decode of the parent tree on "nested
// file": FindEntry only consults its subtree cache from three segments up.
//
// Stat is on the breadcrumb path (once per page request) and runs under the
// exclusive tree lock, so those decodes serialise against every other reader.
func TestGitProvider_Stat_ReadsNoObjects(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"README.md":         "# Test",
		"docs/guide.md":     strings.Repeat("body ", 500),
		"docs/api/types.md": "# Types",
	}

	tests := []struct {
		name        string
		path        string
		wantObjects int64 // subtree decodes needed to walk to the entry
		wantSizes   int64 // header reads (1 for a file, 0 for a directory)
		wantIsDir   bool
		wantSize    int64
	}{
		{
			name:      "root-level file",
			path:      "/README.md",
			wantSizes: 1,
			wantSize:  int64(len("# Test")),
		},
		{
			name:        "nested file",
			path:        "/docs/guide.md",
			wantObjects: 1, // the docs tree, to reach the entry at all
			wantSizes:   1,
			wantSize:    int64(len(strings.Repeat("body ", 500))),
		},
		{
			name:      "root-level directory",
			path:      "/docs",
			wantIsDir: true,
		},
		{
			name:        "nested directory",
			path:        "/docs/api",
			wantObjects: 1, // the docs tree
			wantIsDir:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// One provider per case, deliberately: object.Tree memoises the
			// subtrees it walks, so the counts above are the cold-tree counts a
			// shared provider could not reproduce.
			p, counting := newCountingProvider(t, files)

			info, err := p.Stat(t.Context(), tt.path)
			if err != nil {
				t.Fatalf("Stat(%q) error = %v", tt.path, err)
			}

			if got := counting.objects.Load(); got != tt.wantObjects {
				t.Errorf("Stat(%q) materialised %d objects, want %d — it should read entries and headers only",
					tt.path, got, tt.wantObjects)
			}
			if got := counting.sizes.Load(); got != tt.wantSizes {
				t.Errorf("Stat(%q) did %d header reads, want %d", tt.path, got, tt.wantSizes)
			}
			if info.IsDir() != tt.wantIsDir {
				t.Errorf("Stat(%q).IsDir() = %v, want %v", tt.path, info.IsDir(), tt.wantIsDir)
			}
			if info.Size() != tt.wantSize {
				t.Errorf("Stat(%q).Size() = %d, want %d", tt.path, info.Size(), tt.wantSize)
			}
		})
	}
}

// TestGitProvider_ReadFile_NoWastedDecodeOnDirectory pins that a directory read
// does not first decode the subtree as a blob. The old probe called
// tree.File(path) — GetBlob against the subtree's own hash — and only then fell
// back to tree.Tree, so every directory hit paid for one decode it discarded.
func TestGitProvider_ReadFile_NoWastedDecodeOnDirectory(t *testing.T) {
	t.Parallel()

	p, counting := newCountingProvider(t, map[string]string{
		"README.md":     "# Test",
		"docs/guide.md": "# Guide",
	})

	body, _, err := p.ReadFile(t.Context(), "/docs")
	if err != nil {
		t.Fatalf("ReadFile(/docs) error = %v", err)
	}
	if !strings.Contains(string(body), "guide.md") {
		t.Fatalf("ReadFile(/docs) did not list the directory: %s", body)
	}

	// Exactly one: the docs tree itself. Two means the blob probe is back.
	if got := counting.objects.Load(); got != 1 {
		t.Errorf("ReadFile(/docs) materialised %d objects, want 1 (the subtree)", got)
	}
}

// TestGitProvider_ReadFile_ExactSizeBuffer pins blobBytes: the returned slice is
// allocated at the blob's size, not grown by a bytes.Buffer. file.Contents()
// leaves a buffer whose capacity is rounded up past len, and the []byte(string)
// conversion behind it copied the payload a second time.
func TestGitProvider_ReadFile_ExactSizeBuffer(t *testing.T) {
	t.Parallel()

	p := benchProvider(t)

	body, _, err := p.ReadFile(t.Context(), "/docs/guide.md")
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(body) != benchPage {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(body), len(benchPage))
	}
	if cap(body) != len(body) {
		t.Errorf("cap = %d, len = %d — the blob was grown into a buffer, not read into an exact-size slice",
			cap(body), len(body))
	}
}

// TestGitTreeFS_Stat_ReadsNoBlob pins gitTreeFS's fs.StatFS implementation.
// Without it fs.Stat falls back to Open, which decodes the blob to hand back a
// handle nobody reads — sitemap and feed generation stat every page in the site
// for a ModTime that is the same commit timestamp throughout.
func TestGitTreeFS_Stat_ReadsNoBlob(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("body ", 500)
	p, counting := newCountingProvider(t, map[string]string{"docs/guide.md": content})
	fsys, err := p.RootFS(t.Context())
	if err != nil {
		t.Fatalf("RootFS error = %v", err)
	}

	info, err := fs.Stat(fsys, "docs/guide.md")
	if err != nil {
		t.Fatalf("fs.Stat error = %v", err)
	}

	// One object: the docs tree. Two means the blob was decoded, which is what
	// the io/fs fallback to Open does.
	if got := counting.objects.Load(); got != 1 {
		t.Errorf("fs.Stat materialised %d objects, want 1 (the parent tree)", got)
	}
	if got := counting.sizes.Load(); got != 1 {
		t.Errorf("fs.Stat did %d header reads, want 1", got)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", info.Size(), len(content))
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}
}

// TestGitTreeFS_ReadFile_ExactSizeBuffer pins gitTreeFS's fs.ReadFileFS
// implementation. The io/fs fallback opens the file and copies it into a
// buffer it sizes at size+1, so cap != len — the second copy blobBytes exists
// to avoid, on the path the metadata and search index builds take over every
// file in the repository.
func TestGitTreeFS_ReadFile_ExactSizeBuffer(t *testing.T) {
	t.Parallel()

	p := benchProvider(t)
	fsys, err := p.RootFS(t.Context())
	if err != nil {
		t.Fatalf("RootFS error = %v", err)
	}

	body, err := fs.ReadFile(fsys, "docs/guide.md")
	if err != nil {
		t.Fatalf("fs.ReadFile error = %v", err)
	}
	if string(body) != benchPage {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(body), len(benchPage))
	}
	if cap(body) != len(body) {
		t.Errorf("cap = %d, len = %d — fs.ReadFile fell back to Open and copied the blob again",
			cap(body), len(body))
	}
}

// TestGitTreeFS_ReadFile_Directory keeps ReadFile's error path in line with the
// io/fs fallback it replaces: reading a directory fails rather than returning
// its listing (which is what GitProvider.ReadFile does, one layer up).
func TestGitTreeFS_ReadFile_Directory(t *testing.T) {
	t.Parallel()

	p := benchProvider(t)
	fsys, err := p.RootFS(t.Context())
	if err != nil {
		t.Fatalf("RootFS error = %v", err)
	}

	if _, err := fs.ReadFile(fsys, "docs"); err == nil {
		t.Error("fs.ReadFile(docs) error = nil, want an error")
	}
}

// The benchmarks run against memory storage, which understates the win: the
// deployed storer for --git-storage-dir is filesystem.ObjectStorage, where
// materialising an object is a packfile seek plus a delta and zlib decode
// rather than a map lookup.
func BenchmarkGitProvider_Stat(b *testing.B) {
	p := benchProvider(b)
	ctx := b.Context()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.Stat(ctx, "/docs/guide.md"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitProvider_ReadFile(b *testing.B) {
	p := benchProvider(b)
	ctx := b.Context()

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := p.ReadFile(ctx, "/docs/guide.md"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitProvider_ReadDir(b *testing.B) {
	p := benchProvider(b)
	ctx := b.Context()

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := p.ReadFile(ctx, "/docs"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGitTreeFS_Stat and BenchmarkGitTreeFS_ReadFile cover the RootFS
// surface the index builds and the sitemap use, which reaches the same objects
// through io/fs rather than through the provider methods.
func BenchmarkGitTreeFS_Stat(b *testing.B) {
	fsys := benchRootFS(b)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := fs.Stat(fsys, "docs/guide.md"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitTreeFS_ReadFile(b *testing.B) {
	fsys := benchRootFS(b)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := fs.ReadFile(fsys, "docs/guide.md"); err != nil {
			b.Fatal(err)
		}
	}
}
