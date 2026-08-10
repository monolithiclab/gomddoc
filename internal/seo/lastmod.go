package seo

import (
	"io/fs"
	"strings"
	"time"
)

// LastModified folds a page's two freshness signals into the one answer every
// generated document has to give: the source file's modification time, or the
// frontmatter `date` when the file's own time is unavailable.
//
// sitemap.xml's <lastmod>, feed.xml's <updated> and JSON-LD's dateModified all
// describe the same page to the same crawler, and three hand-rolled folds is how
// they came to disagree — sitemap had no fallback at all, so a page whose stat
// failed appeared undated there and dated in the other two.
//
// The result is UTC. RFC 3339 permits any offset, but emitting the serving
// machine's local zone in one document and Z in another makes one instant look
// like two claims.
func LastModified(modTime, date time.Time) time.Time {
	if modTime.IsZero() {
		modTime = date
	}
	return modTime.UTC()
}

// StatModTime returns the modification time of the content file at pagePath
// (a site path, "/guide.md") within fsys, rooted at the content root. A nil fsys
// or a failed stat yields the zero time rather than an error: an unavailable
// timestamp degrades to the frontmatter date through LastModified, it does not
// fail the caller.
func StatModTime(fsys fs.FS, pagePath string) time.Time {
	if fsys == nil {
		return time.Time{}
	}
	info, err := fs.Stat(fsys, strings.TrimPrefix(pagePath, "/"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
