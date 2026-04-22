package provider

import (
	"log/slog"
	"net"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// resolveAuth determines the authentication method for the endpoint.
// Returns nil for anonymous access (git://, https://).
// Returns SSH agent auth for ssh:// endpoints.
func resolveAuth(endpoint *transport.Endpoint) (transport.AuthMethod, error) {
	switch endpoint.Protocol {
	case "ssh":
		return setupSSHAuth(endpoint.User)
	case "git":
		return nil, nil // Git protocol is anonymous
	case "https", "http":
		return nil, nil // Anonymous HTTPS (token auth deferred)
	default:
		return nil, nil
	}
}

// setupSSHAuth creates an SSH agent-based authentication method.
// Uses the running SSH agent (via SSH_AUTH_SOCK) for key management.
func setupSSHAuth(user string) (transport.AuthMethod, error) {
	if user == "" {
		user = "git"
	}

	auth, err := ssh.NewSSHAgentAuth(user)
	if err != nil {
		return nil, &PathError{Op: "ssh-agent", Path: user, Err: ErrGitAuthFailed}
	}

	// Configure host key verification
	auth.HostKeyCallback = createHostKeyCallback()

	return auth, nil
}

// createHostKeyCallback creates a host key verification callback.
// Tries to use known_hosts file, falls back to TOFU (trust on first use).
func createHostKeyCallback() gossh.HostKeyCallback {
	// Try to use known_hosts file
	knownHostsPath := os.ExpandEnv("$HOME/.ssh/known_hosts")
	callback, err := ssh.NewKnownHostsCallback(knownHostsPath)
	if err == nil {
		return callback
	}

	// Fallback: log warning and accept (TOFU)
	slog.Warn("known_hosts not available, accepting all host keys (TOFU)",
		slog.String("path", knownHostsPath),
		slog.Any("error", err),
	)

	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		slog.Debug("Accepting host key",
			slog.String("host", hostname),
			slog.String("key_type", key.Type()),
		)
		return nil
	}
}
