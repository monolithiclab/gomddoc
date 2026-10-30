package server

import (
	"encoding/json"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
)

// WebManifestPath is where serve answers the web app manifest and where build
// writes it; head-icons links it.
const WebManifestPath = "/site.webmanifest"

const mimeManifest = "application/manifest+json"

// manifestIcon is one entry of a web app manifest's icons list.
type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

// webManifest is the subset of the W3C manifest gomddoc publishes.
type webManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description,omitempty"`
	Icons           []manifestIcon `json:"icons"`
	ThemeColor      string         `json:"theme_color"`
	BackgroundColor string         `json:"background_color"`
	Display         string         `json:"display"`
	StartURL        string         `json:"start_url"`
}

// GenerateWebManifest is the site's web app manifest. It is generated rather
// than shipped as a static file because the name is the site's: a static one
// would install every gomddoc site as "gomddoc". The icons are the shared set
// in /_assets/ (a site replaces them in .gomddoc/static/); theme_color is the
// icon's blue, and background_color the page's, theme.vars bg first like the
// themes' theme-color meta.
func GenerateWebManifest(site config.SiteConfig) []byte {
	icons := make([]manifestIcon, len(assets.ManifestIcons))
	for i, icon := range assets.ManifestIcons {
		icons[i] = manifestIcon{Src: "/_assets/" + icon.Name, Sizes: icon.Sizes, Type: "image/png", Purpose: icon.Purpose}
	}
	background := site.Theme.Vars["bg"]
	if background == "" {
		background = "#ffffff"
	}
	data, _ := json.MarshalIndent(webManifest{ // a struct of strings always marshals
		Name:            site.Meta.Title,
		ShortName:       site.Meta.Title,
		Description:     site.Meta.Description,
		Icons:           icons,
		ThemeColor:      "#2563eb",
		BackgroundColor: background,
		Display:         "standalone",
		StartURL:        "/",
	}, "", "  ")
	return append(data, '\n')
}

// WebManifestHandler serves the generated web app manifest.
type WebManifestHandler struct {
	cached []byte
}

// NewWebManifestHandler creates a WebManifestHandler for the site.
func NewWebManifestHandler(site config.SiteConfig) *WebManifestHandler {
	return &WebManifestHandler{cached: GenerateWebManifest(site)}
}

// ServeHTTP writes the manifest.
func (h *WebManifestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveWithETag(w, r, h.cached, mimeManifest, cacheDynamic)
}
