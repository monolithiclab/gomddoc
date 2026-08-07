package provider

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

// ParsedGitURL holds the parsed components of a git:// URL.
type ParsedGitURL struct {
	// Endpoint is the go-git transport endpoint for cloning.
	Endpoint *transport.Endpoint

	// Ref is the branch, tag, or commit hash to checkout.
	// Defaults to "HEAD" if not specified.
	Ref string

	// Subdir is the subdirectory within the repository to serve.
	// Empty string means serve from repository root.
	Subdir string
}

// parseGitURL parses a git URL into its components.
//
// Supported formats:
//   - git://host/path[.git][#ref[:subdir]]
//   - git+ssh://[user@]host[:port]/path[.git][#ref[:subdir]]
//   - git+https://host[:port]/path[.git][#ref[:subdir]]
//
// The fragment (after #) contains: ref[:subdir]
//   - ref: Branch name, tag, or commit hash. Defaults to "HEAD" if omitted.
//   - subdir: Subdirectory to serve. Defaults to repository root if omitted.
//
// Examples:
//   - git+https://github.com/user/docs.git
//   - git+ssh://git@github.com/org/docs#main
//   - git+https://github.com/user/monorepo#main:docs/api
func parseGitURL(rawURL string) (*ParsedGitURL, error) {
	if rawURL == "" {
		return nil, errors.New("empty URL")
	}

	// Normalize scheme for net/url parsing
	// go-git understands standard schemes, so we convert:
	//   git+ssh:// -> ssh://
	//   git+https:// -> https://
	//   git:// -> git://
	var normalized string
	switch {
	case strings.HasPrefix(rawURL, "git+ssh://"):
		normalized = "ssh://" + strings.TrimPrefix(rawURL, "git+ssh://")
	case strings.HasPrefix(rawURL, "git+https://"):
		normalized = "https://" + strings.TrimPrefix(rawURL, "git+https://")
	case strings.HasPrefix(rawURL, "git://"):
		// Keep as-is, go-git handles git:// protocol
		normalized = rawURL
	default:
		return nil, errors.New("unsupported scheme: must be git://, git+ssh://, or git+https://")
	}

	// Parse URL to extract fragment
	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("malformed URL: %w", err)
	}

	if u.Host == "" {
		return nil, errors.New("missing host")
	}

	// Extract ref and subdir from fragment: "main:docs/api"
	ref := "HEAD"
	subdir := ""
	if u.Fragment != "" {
		parts := strings.SplitN(u.Fragment, ":", 2)
		if parts[0] != "" {
			ref = parts[0]
		}
		if len(parts) == 2 {
			// Clean the subdir path: remove leading/trailing slashes
			subdir = strings.Trim(parts[1], "/")
		}
	}

	// Clear fragment before creating endpoint
	// (go-git doesn't understand fragments)
	u.Fragment = ""
	u.RawFragment = ""

	// Use go-git's endpoint parsing for protocol-specific handling
	endpoint, err := transport.NewEndpoint(u.String())
	if err != nil {
		return nil, fmt.Errorf("invalid git endpoint: %w", err)
	}

	return &ParsedGitURL{
		Endpoint: endpoint,
		Ref:      ref,
		Subdir:   subdir,
	}, nil
}
