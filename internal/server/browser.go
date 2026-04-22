package server

import (
	"os"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the given URL in the default browser.
// It respects the $BROWSER environment variable if set.
func OpenBrowser(url string) error {
	if browser := os.Getenv("BROWSER"); browser != "" {
		return exec.Command(browser, url).Start() // #nosec G204 G702 -- URL is internally generated from server address
	}

	return openBrowserPlatform(url)
}

// openBrowserPlatform opens the URL using the platform-specific command.
func openBrowserPlatform(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start() // #nosec G204 -- URL is internally generated
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start() // #nosec G204 -- URL is internally generated
	default: // linux, freebsd, etc.
		return exec.Command("xdg-open", url).Start() // #nosec G204 -- URL is internally generated
	}
}
