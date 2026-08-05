package main

import (
	"maps"
	"strings"
	"testing"
)

// The checks worth pinning are the ones that read bytes off the wire: a wrong
// answer there is a confident [OK] on a broken site, or a [FAIL] that sends
// someone rebuilding an icon that was already correct. The HTTP plumbing and
// checkPNGSize are left alone — the latter is a comparison against what
// png.DecodeConfig returns, with one edge length and so nothing to transpose.

// makeICO builds an ICO directory (little-endian) with one 16-byte entry per
// width. Width is the only field the check reads.
func makeICO(typ byte, widths ...byte) []byte {
	out := []byte{0, 0, typ, 0, byte(len(widths)), 0} // reserved, type, entry count
	for _, w := range widths {
		//     w  h  colors  reserved  planes  bitcount  bytesInRes   offset
		out = append(out, w, w, 0, 0, 1, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	return out
}

// pngChunk builds a length-prefixed PNG chunk with a zeroed CRC — nothing in
// this file verifies CRCs, and a real one would only obscure the fixture.
func pngChunk(typ string, payload ...byte) []byte {
	n := len(payload)
	c := []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	c = append(c, typ...)
	c = append(c, payload...)
	return append(c, 0, 0, 0, 0)
}

// makePNG emits a signature plus a 13-byte IHDR, which puts colorType at overall
// offset 25 — the byte checkPNGNoAlpha indexes directly.
func makePNG(colorType byte, extra ...string) []byte {
	out := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	out = append(out, pngChunk("IHDR", 0, 0, 0, 180, 0, 0, 0, 180, 8, colorType, 0, 0, 0)...)
	for _, typ := range extra {
		out = append(out, pngChunk(typ)...)
	}
	return append(out, pngChunk("IEND")...)
}

func TestCheckICOSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantOK  bool
		wantMsg string
	}{
		{name: "16/32/48 present", data: makeICO(1, 16, 32, 48), wantOK: true, wantMsg: "sizes: [16 32 48]"},
		{name: "single resolution", data: makeICO(1, 32), wantMsg: "missing [16 48] (have [32])"},
		{
			// A width byte of 0 means 256 — the ICO directory has one byte per
			// dimension, so 256 cannot be encoded literally. Reading it as 0
			// reports "have [0 16 32 48]" and sorts the entry first.
			name:    "zero width means 256",
			data:    makeICO(1, 16, 32, 48, 0),
			wantOK:  true,
			wantMsg: "sizes: [16 32 48 256]",
		},
		{name: "cursor, not icon", data: makeICO(2, 16, 32, 48), wantMsg: "not a valid ICO file"},
		{name: "truncated header", data: makeICO(1, 16)[:4], wantMsg: "file too small to be ICO"},
		// The header promises three entries; the second one is cut in half.
		{name: "truncated entry", data: makeICO(1, 16, 32, 48)[:6+16+8], wantMsg: "unexpected EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := checkICOSizes(tt.data)
			if got.ok != tt.wantOK {
				t.Errorf("ok = %v, want %v (msg %q)", got.ok, tt.wantOK, got.msg)
			}
			if got.msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", got.msg, tt.wantMsg)
			}
		})
	}
}

func TestCheckPNGNoAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   []byte
		wantOK bool
	}{
		{name: "truecolor", data: makePNG(2), wantOK: true},
		{name: "palette", data: makePNG(3), wantOK: true},
		{name: "grayscale", data: makePNG(0), wantOK: true},
		{name: "truecolor with alpha", data: makePNG(6)},
		{name: "grayscale with alpha", data: makePNG(4)},
		// tRNS makes a palette or truecolor image transparent without changing
		// the color type, so the color-type test alone passes this one.
		{name: "palette with tRNS", data: makePNG(3, "tRNS")},
		{name: "not a PNG", data: append([]byte("GIF89a__"), makePNG(2)[8:]...)},
		{name: "too short", data: makePNG(2)[:20]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checkPNGNoAlpha("/apple-touch-icon.png", tt.data); got.ok != tt.wantOK {
				t.Errorf("ok = %v, want %v (msg %q)", got.ok, tt.wantOK, got.msg)
			}
		})
	}
}

// TestHasPNGChunk covers the walk itself, including the guard that stops a
// bogus chunk length from indexing past the buffer.
func TestHasPNGChunk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		data  []byte
		chunk string
		want  bool
	}{
		// The positive control; the negative one is TestCheckPNGNoAlpha's
		// "palette" row, which reaches this function through the same path.
		{name: "present", data: makePNG(3, "tRNS"), chunk: "tRNS", want: true},
		// IEND terminates the walk, so a chunk after it is not reachable.
		{name: "after IEND", data: append(makePNG(3), pngChunk("tRNS")...), chunk: "tRNS"},
		{
			name:  "length overflows buffer",
			data:  append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, 0xFF, 0xFF, 0xFF, 0xFF, 'I', 'H', 'D', 'R'),
			chunk: "tRNS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasPNGChunk(tt.data, tt.chunk); got != tt.want {
				t.Errorf("hasPNGChunk(%q) = %v, want %v", tt.chunk, got, tt.want)
			}
		})
	}
}

func TestHasPurpose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		field, want string
		expect      bool
	}{
		// An absent purpose defaults to "any" per the W3C manifest spec, so a
		// manifest that omits it still satisfies the two "any" entries.
		{field: "", want: "any", expect: true},
		{field: "", want: "maskable"},
		{field: "maskable", want: "maskable", expect: true},
		{field: "any maskable", want: "maskable", expect: true},
		{field: "any maskable", want: "any", expect: true},
		{field: "Maskable", want: "maskable", expect: true},
		{field: "monochrome", want: "maskable"},
	}

	for _, tt := range tests {
		t.Run(tt.field+"/"+tt.want, func(t *testing.T) {
			t.Parallel()

			if got := hasPurpose(tt.field, tt.want); got != tt.expect {
				t.Errorf("hasPurpose(%q, %q) = %v, want %v", tt.field, tt.want, got, tt.expect)
			}
		})
	}
}

func TestAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, tag string
		want      map[string]string
	}{
		{name: "double quoted", tag: `<link rel="icon" sizes="48x48">`, want: map[string]string{"rel": "icon", "sizes": "48x48"}},
		{name: "single quoted", tag: `<link rel='icon' sizes='any'>`, want: map[string]string{"rel": "icon", "sizes": "any"}},
		{name: "unquoted", tag: `<link rel=icon sizes=48x48>`, want: map[string]string{"rel": "icon", "sizes": "48x48"}},
		{name: "spaces around equals", tag: `<link rel = "icon">`, want: map[string]string{"rel": "icon"}},
		{name: "mixed case name", tag: `<link REL="icon">`, want: map[string]string{"rel": "icon"}},
		{name: "no attributes", tag: `<link>`, want: map[string]string{}},
		// A prefixed attribute must land under its own key, so that a lookup of
		// "sizes" misses: a word boundary sits between the hyphen and the "s"
		// as readily as it sits before a standalone attribute.
		{name: "prefixed name keeps its prefix", tag: `<link data-sizes="16x16">`, want: map[string]string{"data-sizes": "16x16"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := attrs(tt.tag); !maps.Equal(got, tt.want) {
				t.Errorf("attrs(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

// TestExtractHead pins that body content is excluded, which is what keeps a
// <link> in the body from satisfying a head-tag check.
func TestExtractHead(t *testing.T) {
	t.Parallel()

	// Every case puts the same link in the head, so only what must be *absent*
	// varies per row.
	const wantHas = `rel="manifest"`

	tests := []struct {
		name, html  string
		wantMissing string
	}{
		{
			name:        "head delimited",
			html:        `<html><head><link rel="manifest"></head><body><link rel="icon"></body></html>`,
			wantMissing: `rel="icon"`,
		},
		{
			name: "head with attributes",
			html: `<html><HEAD lang="en"><link rel="manifest"></HEAD><body></body></html>`,
		},
		{
			// No head element: fall back to the whole document rather than
			// reporting every tag as missing.
			name: "no head element",
			html: `<link rel="manifest">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractHead(tt.html)
			if !strings.Contains(got, wantHas) {
				t.Errorf("extractHead(%q) = %q, want it to contain %q", tt.html, got, wantHas)
			}
			if tt.wantMissing != "" && strings.Contains(got, tt.wantMissing) {
				t.Errorf("extractHead(%q) = %q, want it to exclude %q", tt.html, got, tt.wantMissing)
			}
		})
	}
}

func TestCheckLinkICO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, head string
		wantOK     bool
	}{
		{name: "correct", head: `<link rel="icon" href="/favicon.ico" sizes="48x48">`, wantOK: true},
		// Without sizes="48x48" Chromium prefers the ICO over the SVG, which is
		// the whole reason the attribute is mandatory.
		{name: "missing sizes", head: `<link rel="icon" href="/favicon.ico">`},
		{name: "wrong sizes", head: `<link rel="icon" href="/favicon.ico" sizes="any">`},
		{name: "absent", head: `<link rel="icon" href="/favicon.svg" sizes="any">`},
		// The SVG link must not be mistaken for the ICO link, and vice versa,
		// when both carry rel="icon".
		{
			name:   "both icons declared",
			head:   `<link rel="icon" href="/favicon.svg" sizes="any"><link rel="icon" href="/favicon.ico" sizes="48x48">`,
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checkLinkICO(tt.head); got.ok != tt.wantOK {
				t.Errorf("ok = %v, want %v (msg %q)", got.ok, tt.wantOK, got.msg)
			}
		})
	}
}

// TestCheckThemeColor pins that the two theme-color tags are matched by their
// media query — a site with one unconditional tag must fail both.
func TestCheckThemeColor(t *testing.T) {
	t.Parallel()

	both := `<meta name="theme-color" content="#FFF" media="(prefers-color-scheme: light)">` +
		`<meta name="theme-color" content="#000" media="(prefers-color-scheme: dark)">`

	tests := []struct {
		name, head, scheme string
		wantOK             bool
	}{
		{name: "light present", head: both, scheme: "light", wantOK: true},
		{name: "dark present", head: both, scheme: "dark", wantOK: true},
		{name: "unconditional tag", head: `<meta name="theme-color" content="#FFF">`, scheme: "light"},
		{
			name:   "only light declared",
			head:   `<meta name="theme-color" content="#FFF" media="(prefers-color-scheme: light)">`,
			scheme: "dark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checkThemeColor(tt.head, tt.scheme); got.ok != tt.wantOK {
				t.Errorf("ok = %v, want %v (msg %q)", got.ok, tt.wantOK, got.msg)
			}
		})
	}
}

func TestCheckManifest(t *testing.T) {
	t.Parallel()

	const complete = `{
		"name": "gomddoc", "short_name": "gomddoc", "theme_color": "#000",
		"background_color": "#fff", "display": "standalone", "start_url": "/",
		"icons": [
			{"src": "/android-chrome-192x192.png", "sizes": "192x192", "purpose": "any"},
			{"src": "/android-chrome-512x512.png", "sizes": "512x512", "purpose": "any"},
			{"src": "/maskable-icon-192x192.png", "sizes": "192x192", "purpose": "maskable"},
			{"src": "/maskable-icon-512x512.png", "sizes": "512x512", "purpose": "maskable"}
		]
	}`

	t.Run("complete manifest passes every check", func(t *testing.T) {
		t.Parallel()

		got := checkManifest([]byte(complete))
		if len(got) != 11 {
			t.Fatalf("got %d results, want 11 (valid JSON + 6 fields + 4 icons)", len(got))
		}
		for _, r := range got {
			if !r.ok {
				t.Errorf("%s failed: %s", r.name, r.msg)
			}
		}
	})

	t.Run("invalid JSON short-circuits", func(t *testing.T) {
		t.Parallel()

		got := checkManifest([]byte("{"))
		if len(got) != 1 || got[0].ok {
			t.Fatalf("got %d results (want 1 failure): %+v", len(got), got)
		}
	})

	t.Run("maskable icons missing", func(t *testing.T) {
		t.Parallel()

		// The same two files listed without purpose default to "any", so the
		// four "any" and "maskable" slots cannot all be satisfied.
		const anyOnly = `{
			"name": "n", "short_name": "n", "theme_color": "#000",
			"background_color": "#fff", "display": "standalone", "start_url": "/",
			"icons": [
				{"src": "/a-192.png", "sizes": "192x192"},
				{"src": "/a-512.png", "sizes": "512x512"}
			]
		}`

		var failed []string
		for _, r := range checkManifest([]byte(anyOnly)) {
			if !r.ok {
				failed = append(failed, r.name)
			}
		}
		want := []string{
			`manifest.icons has 192x192 purpose="maskable"`,
			`manifest.icons has 512x512 purpose="maskable"`,
		}
		if len(failed) != len(want) {
			t.Fatalf("failed = %q, want %q", failed, want)
		}
		for i := range want {
			if failed[i] != want[i] {
				t.Errorf("failed[%d] = %q, want %q", i, failed[i], want[i])
			}
		}
	})
}

func TestCheckSVGViewBox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   string
		wantOK bool
	}{
		{name: "declared", data: `<svg viewBox="0 0 32 32"></svg>`, wantOK: true},
		{name: "lowercase spelling", data: `<svg viewbox="0 0 32 32"></svg>`, wantOK: true},
		{name: "absent", data: `<svg width="32" height="32"></svg>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checkSVGViewBox([]byte(tt.data)); got.ok != tt.wantOK {
				t.Errorf("ok = %v, want %v (msg %q)", got.ok, tt.wantOK, got.msg)
			}
		})
	}
}
