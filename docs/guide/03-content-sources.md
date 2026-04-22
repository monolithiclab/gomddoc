---
title: "Content Sources Guide"
description: "Documentation on configuring local and Git-based content sources."
author: "nicolasm"
---

# Content Sources

gomddoc abstracts the source of your documentation behind the `Provider` interface. Whether files are on your local
disk or in a remote repository, gomddoc serves them seamlessly.

## 1. Local Filesystem

This is the default mode. Pass the directory as an argument.

```bash
gomddoc serve /path/to/docs
```

### Features

- **Index Files:** If a user requests a directory (e.g., `/api/`), gomddoc looks for `README.md` (or the
  configured `default_index`) inside it and serves it.
- **Directory Listing:** When enabled via `dir_index: true` in `.gomddoc/config.yml` (or `GOMDDOC_SITE_DIR_INDEX`
  env var), and no index file exists, a Markdown list of files is generated. Default is `false` for security.
- **Hidden Files:** Files starting with `.` (like `.env`) are blocked for security.
- **Path Normalization:** Uses `path.Clean` (not `filepath`) for cross-platform `io/fs` compatibility.

## 2. Git Repositories

gomddoc can clone a repository and serve it. By default the clone is held **in memory** (no files written to disk).
For large repositories, use `--git-storage-dir` to clone to disk instead (see below). The repository is cloned
lazily on first request (not at startup), using a shallow clone for speed.

### URL Syntax

`git+[scheme]://[user@]host/path/to/repo[.git][#ref[:subdir]]`

- **Scheme:** `https`, `ssh` (via `git+ssh://`)
- **Ref (Optional):** Branch name, tag, or commit hash. Defaults to `HEAD`.
- **Subdir (Optional):** Serve only a specific folder within the repo.

### Examples

#### Public Repository (HTTPS)

```bash
gomddoc serve "git+https://github.com/user/repo.git"
```

#### Specific Branch

Serve the `develop` branch:

```bash
gomddoc serve "git+https://github.com/user/repo.git#develop"
```

#### Specific Subdirectory

Serve only the `/docs` folder from the `main` branch:

```bash
gomddoc serve "git+https://github.com/user/repo.git#main:docs"
```

### Private Repositories (SSH Authentication)

To serve private repositories, you must use the SSH protocol and provide a private key.

1. **Generate or Locate a Key:** Ensure you have a deploy key or user key that has read access to the repository.
2. **Run with Key:**

```bash
gomddoc serve \
  "git+ssh://git@github.com/my-org/private-docs.git" \
  --git-key-file ~/.ssh/id_rsa_deploy_key
```

**Environment Variable Example:**

```bash
export GOMDDOC_SERVER_GIT_SSH_KEY=/etc/secrets/ssh-key
gomddoc serve "git+ssh://git@gitlab.company.com/group/project.git"
```

### SSH Security

- The SSH Agent and default `~/.ssh/` key lookup are explicitly disabled for predictability.
  You *must* provide the key file path if using SSH.
- **Host key verification** uses `~/.ssh/known_hosts`. There is no Trust-On-First-Use (TOFU) fallback.
  If the host is not in known_hosts, the connection will fail. Add hosts beforehand:
  ```bash
  ssh-keyscan github.com >> ~/.ssh/known_hosts
  ```

### Clone Behavior

- **Shallow clone:** `depth=1`, single branch, no tags — minimizes bandwidth and memory.
- **Timeout:** Clone operations have a 60-second timeout (configurable via `WithCloneTimeout`). Unresponsive
  hosts will not block the server indefinitely.
- **File size limit:** Files larger than 50MB are rejected (configurable via `WithMaxFileSize`).
- **Git LFS:** LFS pointer files are detected and return a 501 Not Implemented error.
- **Lifecycle:** The provider is safe for concurrent use. Calling `Close()` marks the provider as closed; subsequent
  reads return an error rather than risking nil pointer dereferences.

### Disk-Based Storage

By default, Git repositories are cloned into memory. For large repositories (monorepos, repos with many files),
this can cause out-of-memory errors. Use `--git-storage-dir` to clone to disk instead:

```bash
gomddoc serve \
  "git+https://github.com/large-org/monorepo.git#main:docs" \
  --git-storage-dir /var/cache/gomddoc
```

Or via environment variable:

```bash
export GOMDDOC_SERVER_GIT_STORAGE_DIR=/var/cache/gomddoc
gomddoc serve "git+https://github.com/large-org/monorepo.git"
```

Each repository URL gets a unique subdirectory (SHA-256 hash of the URL), so multiple repos can safely share the
same cache directory. The cache persists across restarts — subsequent startups skip the clone if the cache exists.
