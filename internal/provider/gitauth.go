package provider

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// resolveAuth determines the authentication method for the endpoint.
// Returns nil for anonymous access (git://, https://).
// Returns SSH auth for ssh:// endpoints if a key file is provided.
func resolveAuth(endpoint *transport.Endpoint, sshKeyFile string) (transport.AuthMethod, error) {
	switch endpoint.Protocol {
	case "ssh":
		return setupSSHAuth(endpoint.User, sshKeyFile)
	case "git":
		return nil, nil // Git protocol is anonymous
	case "https", "http":
		return nil, nil // Anonymous HTTPS (token auth deferred)
	default:
		return nil, nil
	}
}

// setupSSHAuth creates an SSH authentication method using the specified key file.
// If no key file is provided, it returns an error as we don't support implicit auth.
func setupSSHAuth(user string, sshKeyFile string) (transport.AuthMethod, error) {
	if user == "" {
		user = "git"
	}

	if sshKeyFile == "" {
		return nil, fmt.Errorf("ssh authentication requires a key file (use --git-key-file)")
	}

	return &ssh.PublicKeysCallback{
		User: user,
		Callback: func() ([]gossh.Signer, error) {
			// #nosec G304 -- Path is provided by admin configuration
			content, err := os.ReadFile(sshKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read key file %s: %w", sshKeyFile, err)
			}

			signer, err := gossh.ParsePrivateKey(content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse key file %s: %w", sshKeyFile, err)
			}

			slog.Debug("Loaded configured SSH key", slog.String("path", sshKeyFile))
			return []gossh.Signer{signer}, nil
		},
		HostKeyCallbackHelper: ssh.HostKeyCallbackHelper{
			HostKeyCallback: createHostKeyCallback(),
		},
	}, nil
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