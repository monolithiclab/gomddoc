---
title: "Content Sources Guide"
description: "Documentation on configuring local and Git-based content sources."
author: "nicolasm"
---

# Content Sources

gomddoc abstracts the source of your documentation behind the `Provider` interface. The `DIR` argument of `serve`,
`preview`, `build`, `mcp` and `doctor` (or `GOMDDOC_SERVER_DIR`) selects the source: a URL starting with `git://`,
`git+ssh://` or `git+https://` is a Git repository, anything else is a local directory.

## 1. Local Filesystem

This is the default mode. Pass the directory as an argument.

```bash
gomddoc serve /path/to/docs
```

### Features

- **Index Files:** If a user requests a directory (e.g., `/api/`), gomddoc looks for `README.md` (or the
  configured `default_index`) inside it and serves it.
- **Directory Listing:** When enabled via `dir_index: true` in `.gomddoc/config.yml` (or `GOMDDOC_SITE_DIR_INDEX`
  env var, or `preview --dir-index`), and no index file exists, a Markdown list of files is generated. Default is
  `false`: the request redirects (302) to the directory's first page in the navigation tree, or returns 403 when the
  directory has none.
- **Hidden Files:** Any path with a segment starting with `.` (like `.env`, `.git/`, `.gomddoc/`) is blocked and
  returns 404. `.well-known/` is the one exception.
- **Exclusions:** Paths matching `exclude` patterns are blocked the same way. See
  [Configuration](02-configuration.md#content-behavior).
- **Path Jail:** Files are read through `os.DirFS`, so a request cannot reach outside the content directory.

## 2. Git Repositories

gomddoc can clone a repository and serve it. By default the clone is held **in memory** (no files written to disk).
For large repositories, use `--git-storage-dir` to clone to disk instead (see below). The clone is a shallow clone of
one branch. It happens during startup, because `serve`, `preview`, `build`, `mcp` and `doctor` all walk the content to
build their indexes before they are ready. The repository is not re-fetched while the process runs: restart to pick up
new commits.

The site configuration file is not read from the repository. A `.gomddoc/config.yml` committed to the repository is
ignored; set site values through environment variables such as `GOMDDOC_SITE_THEME_NAME`, and flags (see
[Configuration](02-configuration.md)). `exclude`, `strip_extensions` and `theme.vars` have no environment variable,
so a Git source runs with their defaults. Theme assets under the repository's `.gomddoc/assets/` are used. The default
site title is derived from the URL, so set `GOMDDOC_SITE_META_TITLE`.

### URL Syntax

`git+[scheme]://[user@]host/path/to/repo[.git][#ref[:subdir]]`

- **Scheme:** `git+https://` (anonymous HTTPS), `git+ssh://` (SSH key authentication) or `git://` (anonymous Git
  protocol). Plain `https://` URLs and scp-style `git@host:org/repo.git` addresses are not recognized as Git sources.
- **Ref (Optional):** Branch name. Defaults to the remote's `HEAD` (its default branch). Tags and commit hashes are
  not supported: the clone asks for `refs/heads/<ref>` and fails with `couldn't find remote ref`.
- **Subdir (Optional):** Serve only a specific folder within the repo. `#:docs` serves `docs/` of the default branch.

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
- **Host key verification** checks the `SSH_KNOWN_HOSTS` environment variable first, then falls
  back to `~/.ssh/known_hosts`. There is no Trust-On-First-Use (TOFU) fallback. If the known_hosts
  file is missing or the host is not listed, the connection will fail. Add hosts beforehand:
  ```bash
  ssh-keyscan github.com >> ~/.ssh/known_hosts
  ```
  In CI/CD or containers without a home directory, set the env var:
  ```bash
  export SSH_KNOWN_HOSTS=/etc/ssh/known_hosts
  ```

### Clone Behavior

- **Shallow clone:** `depth=1`, single branch, no tags — minimizes bandwidth and memory.
- **Timeout:** Clone operations have a 60-second timeout. Unresponsive hosts do not block startup indefinitely.
- **File size limit:** Files larger than 50MB are rejected with 413 Payload Too Large.
- **Git LFS:** LFS pointer files are detected and return 501 Not Implemented.
- **Modification time:** Every file reports the commit's author time as its modification time, so every page in
  `sitemap.xml` (`<lastmod>`), `feed.xml` (`<updated>`) and JSON-LD (`dateModified`) carries that one timestamp.
- **Errors:** A failed clone stops startup with the reason: authentication failure, repository not found, unknown
  branch, or connection refused.

The timeout and size limit are not configurable from the CLI.

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

Each repository URL gets its own subdirectory, named after the first 16 hex digits of the URL's SHA-256 hash, so
multiple repos can share the same storage directory. The directory is created if it does not exist and is not removed
on exit. An existing clone is not reused: when the subdirectory already holds one, startup fails with
`repository already exists`. Delete the subdirectory (or the whole storage directory) before the next start.
