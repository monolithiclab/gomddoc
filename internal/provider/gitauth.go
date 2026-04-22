package provider

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

	hostKeyCallback, err := createHostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("SSH host key verification: %w", err)
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
			HostKeyCallback: hostKeyCallback,
		},
	}, nil
}

// createHostKeyCallback creates a host key verification callback.
// Returns an error when known_hosts is unavailable (fail closed).
func createHostKeyCallback() (gossh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	callback, err := ssh.NewKnownHostsCallback(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("known_hosts required for SSH: %w", err)
	}
	slog.Debug("Using known_hosts for host key verification", slog.String("path", knownHostsPath))
	return callback, nil
}
