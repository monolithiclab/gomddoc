package seo

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestLastModified(t *testing.T) {
	t.Parallel()

	paris := time.FixedZone("CET", 1*60*60)

	tests := []struct {
		name    string
		modTime time.Time
		date    time.Time
		want    time.Time
	}{
		{
			name:    "mtime wins over the frontmatter date",
			modTime: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
			date:    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			want:    time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		},
		{
			name: "falls back to the frontmatter date",
			date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			want: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "neither is zero, and stays zero",
		},
		{
			// The whole point of folding .UTC() in here: a stat on a machine in
			// any other zone must not make sitemap.xml and JSON-LD look like
			// they are claiming different instants.
			name:    "normalizes to UTC",
			modTime: time.Date(2026, 3, 4, 6, 6, 7, 0, paris),
			want:    time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := LastModified(tt.modTime, tt.date)
			if !got.Equal(tt.want) {
				t.Errorf("LastModified() = %v, want %v", got, tt.want)
			}
			// Equal() compares instants and is blind to the zone, which is the
			// one thing the UTC row exists to check.
			if got.Location() != time.UTC {
				t.Errorf("LastModified() location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestStatModTime(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	files := fstest.MapFS{
		"guide.md": &fstest.MapFile{Data: []byte("# Guide"), ModTime: modTime},
	}

	// The leading slash is what every caller has in hand: page paths are site
	// paths, fs.FS paths are not.
	if got := StatModTime(files, "/guide.md"); !got.Equal(modTime) {
		t.Errorf("StatModTime(/guide.md) = %v, want %v", got, modTime)
	}
	if got := StatModTime(files, "/missing.md"); !got.IsZero() {
		t.Errorf("a missing file should yield the zero time, got %v", got)
	}
	if got := StatModTime(nil, "/guide.md"); !got.IsZero() {
		t.Errorf("a nil filesystem should yield the zero time, got %v", got)
	}
}
