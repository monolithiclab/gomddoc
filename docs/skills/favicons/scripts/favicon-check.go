// favicon-check.go assesses whether a website implements the favicon
// requirements documented in docs/skills/favicons/SKILL.md.
//
// Usage, from the skill directory:
//
//	go run scripts/favicon-check.go <url>
//
// Each requirement prints one line of the form:
//
//	[OK]   <check name> [— note]
//	[FAIL] <check name> — <reason>
//
// Exits non-zero when any check fails.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

const userAgent = "favicon-check/1.0"

var client = &http.Client{Timeout: 15 * time.Second}

type result struct {
	name string
	ok   bool
	msg  string
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/favicon-check.go <url>")
		os.Exit(2)
	}

	siteURL := os.Args[1]
	if !strings.HasPrefix(siteURL, "http://") && !strings.HasPrefix(siteURL, "https://") {
		siteURL = "https://" + siteURL
	}
	parsed, err := url.Parse(siteURL)
	if err != nil || parsed.Host == "" {
		fmt.Fprintf(os.Stderr, "invalid URL: %v\n", err)
		os.Exit(2)
	}
	root := &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}

	html, err := fetchString(siteURL)
	if err != nil {
		fmt.Printf("[FAIL] fetch HTML — %v\n", err)
		os.Exit(1)
	}
	head := extractHead(html)

	var results []result
	add := func(r result) { results = append(results, r) }

	// --- HTML head tags ---
	add(checkLinkICO(head))
	add(checkLinkSVG(head))
	add(checkLinkPresent(head, "apple-touch-icon"))
	add(checkLinkPresent(head, "manifest"))
	add(checkThemeColor(head, "light"))
	add(checkThemeColor(head, "dark"))

	// --- File availability (must be at site root) ---
	files := []string{
		"/favicon.ico",
		"/favicon.svg",
		"/apple-touch-icon.png",
		"/android-chrome-192x192.png",
		"/android-chrome-512x512.png",
		"/maskable-icon-192x192.png",
		"/maskable-icon-512x512.png",
		"/site.webmanifest",
	}
	bodies := map[string][]byte{}
	for _, f := range files {
		u := root.ResolveReference(&url.URL{Path: f}).String()
		body, status, err := fetchBytes(u)
		ok := err == nil && status == 200
		r := result{name: fmt.Sprintf("%s served (HTTP 200)", f), ok: ok}
		if !ok {
			r.msg = fmt.Sprintf("status=%d err=%v", status, err)
		}
		add(r)
		if ok {
			bodies[f] = body
		}
	}

	// --- ICO multi-resolution check ---
	if data, ok := bodies["/favicon.ico"]; ok {
		add(checkICOSizes(data))
	}

	// --- SVG viewBox check ---
	if data, ok := bodies["/favicon.svg"]; ok {
		add(checkSVGViewBox(data))
	}

	// --- PNG dimension checks (every icon is square, so one edge length) ---
	for _, c := range []struct {
		path string
		edge int
	}{
		{"/apple-touch-icon.png", 180},
		{"/android-chrome-192x192.png", 192},
		{"/android-chrome-512x512.png", 512},
		{"/maskable-icon-192x192.png", 192},
		{"/maskable-icon-512x512.png", 512},
	} {
		if data, ok := bodies[c.path]; ok {
			add(checkPNGSize(c.path, data, c.edge))
		}
	}

	// --- Opacity check (iOS only; Android composites the icon itself) ---
	if data, ok := bodies["/apple-touch-icon.png"]; ok {
		add(checkPNGNoAlpha("/apple-touch-icon.png", data))
	}

	// --- Manifest checks ---
	if data, ok := bodies["/site.webmanifest"]; ok {
		results = append(results, checkManifest(data)...)
	}

	failures := 0
	for _, r := range results {
		status := "OK  "
		if !r.ok {
			status = "FAIL"
			failures++
		}
		if r.msg != "" {
			fmt.Printf("[%s] %s — %s\n", status, r.name, r.msg)
		} else {
			fmt.Printf("[%s] %s\n", status, r.name)
		}
	}

	fmt.Printf("\n%d checks, %d failures\n", len(results), failures)
	if failures > 0 {
		os.Exit(1)
	}
}

// --- HTTP helpers ---

func fetchString(u string) (string, error) {
	body, status, err := fetchBytes(u)
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "", fmt.Errorf("status %d", status)
	}
	return string(body), nil
}

func fetchBytes(u string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil) // #nosec G704
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req) // #nosec G704
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	return body, resp.StatusCode, err
}

// --- HTML parsing helpers (regex-based; sufficient for <link>/<meta>) ---

var (
	linkTagRE = regexp.MustCompile(`(?is)<link\b[^>]*?/?>`)
	metaTagRE = regexp.MustCompile(`(?is)<meta\b[^>]*?/?>`)
	attrRE    = regexp.MustCompile(`(?i)(?:^|[\s'"])([\w:.-]+)\s*=\s*("[^"]*"|'[^']*'|[^\s>]*)`)
)

func extractHead(html string) string {
	low := strings.ToLower(html)
	// Offsets found in the folded copy only index the original while the two
	// are the same length, and simple case mapping does not guarantee that —
	// U+0130 folds from two bytes to one. Searching the whole document is a
	// harmless fallback; slicing it at a shifted offset is not.
	if len(low) != len(html) {
		return html
	}
	start := strings.Index(low, "<head")
	end := strings.Index(low, "</head>")
	if start == -1 || end == -1 || end < start {
		return html
	}
	return html[start:end]
}

// attrs parses a tag's attributes into a map keyed by lowercased name. Reading
// whole names, rather than searching a tag for one name at a time, is what
// keeps data-sizes out of sizes: the value lands under its own key, so a lookup
// of "sizes" misses by construction. Anchoring a per-name pattern cannot manage
// that on its own — \b sits between the hyphen and the "s" as readily as it
// sits before a standalone attribute.
func attrs(tag string) map[string]string {
	out := make(map[string]string)
	for _, m := range attrRE.FindAllStringSubmatch(tag, -1) {
		v := m[2]
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
			v = v[1 : len(v)-1]
		}
		out[strings.ToLower(m[1])] = v
	}
	return out
}

// tagsByAttr returns the parsed attributes of each tag whose attrName equals
// attrValue, case-insensitively.
func tagsByAttr(html string, re *regexp.Regexp, attrName, attrValue string) []map[string]string {
	var out []map[string]string
	for _, t := range re.FindAllString(html, -1) {
		if a := attrs(t); strings.EqualFold(a[attrName], attrValue) {
			out = append(out, a)
		}
	}
	return out
}

// --- Head tag checks ---

// Both icon links carry rel="icon", so each check skips the other's tag by
// href rather than assuming there is only one.
func checkLinkICO(head string) result {
	name := `<link rel="icon" href="/favicon.ico" sizes="48x48">`
	for _, a := range tagsByAttr(head, linkTagRE, "rel", "icon") {
		if !strings.HasSuffix(strings.ToLower(a["href"]), "favicon.ico") {
			continue
		}
		if a["sizes"] != "48x48" {
			return result{name: name, ok: false, msg: `missing sizes="48x48" (Chromium will pick the ICO over the SVG)`}
		}
		return result{name: name, ok: true}
	}
	return result{name: name, ok: false, msg: "tag not found"}
}

func checkLinkSVG(head string) result {
	name := `<link rel="icon" href="/favicon.svg" sizes="any" type="image/svg+xml">`
	for _, a := range tagsByAttr(head, linkTagRE, "rel", "icon") {
		if !strings.HasSuffix(strings.ToLower(a["href"]), ".svg") {
			continue
		}
		if a["sizes"] != "any" {
			return result{name: name, ok: false, msg: `missing sizes="any"`}
		}
		if t := strings.ToLower(a["type"]); t != "" && t != "image/svg+xml" {
			return result{name: name, ok: false, msg: `type should be image/svg+xml`}
		}
		return result{name: name, ok: true}
	}
	return result{name: name, ok: false, msg: "tag not found"}
}

// checkLinkPresent covers the links whose mere presence is the requirement —
// the browser derives everything else from the referenced file.
func checkLinkPresent(head, rel string) result {
	name := fmt.Sprintf(`<link rel="%s">`, rel)
	if len(tagsByAttr(head, linkTagRE, "rel", rel)) > 0 {
		return result{name: name, ok: true}
	}
	return result{name: name, ok: false, msg: "tag not found"}
}

func checkThemeColor(head, scheme string) result {
	name := fmt.Sprintf(`<meta name="theme-color" media="(prefers-color-scheme: %s)">`, scheme)
	for _, a := range tagsByAttr(head, metaTagRE, "name", "theme-color") {
		if strings.Contains(strings.ToLower(a["media"]), scheme) {
			return result{name: name, ok: true}
		}
	}
	return result{name: name, ok: false, msg: "no theme-color meta with this color scheme"}
}

// --- ICO check (multi-resolution) ---

func checkICOSizes(data []byte) result {
	name := "/favicon.ico is multi-resolution (16, 32, 48)"
	if len(data) < 6 {
		return result{name: name, ok: false, msg: "file too small to be ICO"}
	}
	r := bytes.NewReader(data)
	var hdr struct {
		Reserved uint16
		Type     uint16
		Count    uint16
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return result{name: name, ok: false, msg: err.Error()}
	}
	if hdr.Reserved != 0 || hdr.Type != 1 {
		return result{name: name, ok: false, msg: "not a valid ICO file"}
	}
	sizes := map[int]struct{}{}
	for range int(hdr.Count) {
		var e struct {
			Width, Height      uint8
			Colors, Reserved   uint8
			Planes, BitCount   uint16
			BytesInRes, Offset uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			return result{name: name, ok: false, msg: err.Error()}
		}
		// The directory has one byte per dimension, so 256 is encoded as 0.
		w := int(e.Width)
		if w == 0 {
			w = 256
		}
		sizes[w] = struct{}{}
	}
	missing := []int{}
	for _, want := range []int{16, 32, 48} {
		if _, ok := sizes[want]; !ok {
			missing = append(missing, want)
		}
	}
	got := slices.Sorted(maps.Keys(sizes))
	if len(missing) > 0 {
		return result{name: name, ok: false, msg: fmt.Sprintf("missing %v (have %v)", missing, got)}
	}
	return result{name: name, ok: true, msg: fmt.Sprintf("sizes: %v", got)}
}

// --- PNG checks ---

// Every favicon is square, so one edge length covers both dimensions — which
// also means there is no width/height pair to transpose.
func checkPNGSize(path string, data []byte, edge int) result {
	name := fmt.Sprintf("%s is %dx%d", path, edge, edge)
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return result{name: name, ok: false, msg: err.Error()}
	}
	if cfg.Width != edge || cfg.Height != edge {
		return result{name: name, ok: false, msg: fmt.Sprintf("got %dx%d", cfg.Width, cfg.Height)}
	}
	return result{name: name, ok: true}
}

// PNG color types (IHDR byte 25): 0 grayscale, 2 RGB, 3 palette, 4 grayscale+alpha, 6 RGB+alpha.
// A tRNS chunk also adds transparency to non-alpha types.
func checkPNGNoAlpha(path string, data []byte) result {
	name := fmt.Sprintf("%s has no alpha channel (iOS expects opaque)", path)
	if len(data) < 26 {
		return result{name: name, ok: false, msg: "PNG too short"}
	}
	pngSig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.Equal(data[:8], pngSig) {
		return result{name: name, ok: false, msg: "not a PNG"}
	}
	colorType := data[25]
	if colorType == 4 || colorType == 6 {
		return result{name: name, ok: false, msg: fmt.Sprintf("PNG color type %d includes alpha", colorType)}
	}
	if hasPNGChunk(data, "tRNS") {
		return result{name: name, ok: false, msg: "tRNS chunk introduces transparency"}
	}
	return result{name: name, ok: true}
}

func hasPNGChunk(data []byte, chunk string) bool {
	i := 8
	for i+8 <= len(data) {
		length := binary.BigEndian.Uint32(data[i : i+4])
		typ := string(data[i+4 : i+8])
		if typ == chunk {
			return true
		}
		if typ == "IEND" {
			return false
		}
		next := i + 8 + int(length) + 4
		if next <= i || next > len(data) {
			return false
		}
		i = next
	}
	return false
}

// --- SVG check ---

func checkSVGViewBox(data []byte) result {
	name := "/favicon.svg declares a viewBox"
	if !bytes.Contains(bytes.ToLower(data), []byte("viewbox")) {
		return result{name: name, ok: false, msg: "missing viewBox attribute"}
	}
	return result{name: name, ok: true}
}

// --- Manifest check ---

// These two carry only the fields checkManifest reads, so the structs double as
// the list of what is validated. The icons' src is not among them: the eight
// availability probes above already fetch every icon by its canonical URL.
type manifestIcon struct {
	Sizes   string `json:"sizes"`
	Purpose string `json:"purpose"`
}

type manifestDoc struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Icons           []manifestIcon `json:"icons"`
	ThemeColor      string         `json:"theme_color"`
	BackgroundColor string         `json:"background_color"`
	Display         string         `json:"display"`
	StartURL        string         `json:"start_url"`
}

func checkManifest(data []byte) []result {
	var m manifestDoc
	if err := json.Unmarshal(data, &m); err != nil {
		return []result{{name: "site.webmanifest is valid JSON", ok: false, msg: err.Error()}}
	}
	out := []result{
		{name: "site.webmanifest is valid JSON", ok: true},
		field("manifest.name", m.Name),
		field("manifest.short_name", m.ShortName),
		field("manifest.theme_color", m.ThemeColor),
		field("manifest.background_color", m.BackgroundColor),
		field("manifest.display", m.Display),
		field("manifest.start_url", m.StartURL),
	}
	required := []struct{ size, purpose string }{
		{"192x192", "any"},
		{"512x512", "any"},
		{"192x192", "maskable"},
		{"512x512", "maskable"},
	}
	for _, req := range required {
		found := false
		for _, ic := range m.Icons {
			if ic.Sizes == req.size && hasPurpose(ic.Purpose, req.purpose) {
				found = true
				break
			}
		}
		r := result{name: fmt.Sprintf(`manifest.icons has %s purpose="%s"`, req.size, req.purpose), ok: found}
		if !found {
			r.msg = "no matching entry"
		}
		out = append(out, r)
	}
	return out
}

func hasPurpose(field, want string) bool {
	if field == "" && want == "any" {
		return true // default purpose per W3C spec
	}
	for p := range strings.FieldsSeq(field) {
		if strings.EqualFold(p, want) {
			return true
		}
	}
	return false
}

func field(name, value string) result {
	r := result{name: name, ok: value != ""}
	if !r.ok {
		r.msg = "missing or empty"
	}
	return r
}
