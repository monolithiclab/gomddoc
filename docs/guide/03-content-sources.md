# Content Sources

gomddoc abstracts the source of your documentation. Whether files are on your local disk or in a remote repository, gomddoc serves them seamlessly.

## 1. Local Filesystem

This is the default mode. Point `-d` to any directory.

```bash
gomddoc -d /path/to/docs
```

### Features
*   **Index Files:** If a user requests a directory (e.g., `/api/`), gomddoc looks for `README.md` inside it and serves it.
*   **Directory Listing:** If enabled (currently requires code modification to `ServerConfig.DirIndex`, default is `false`), and no `README.md` exists, a Markdown list of files is generated.
*   **Hidden Files:** Files starting with `.` (like `.env`) are blocked for security.

## 2. Git Repositories

gomddoc can clone a repository into **memory** and serve it. No files are written to disk. The repository is cloned once on startup (or restart).

### URL Syntax

`git+[scheme]://[user@]host/path/to/repo[.git][#ref[:subdir]]`

*   **Scheme:** `https`, `ssh` (via `git+ssh://`)
*   **Ref (Optional):** Branch name, tag, or commit hash. Defaults to `HEAD`.
*   **Subdir (Optional):** Serve only a specific folder within the repo.

### Examples

#### Public Repository (HTTPS)
```bash
gomddoc -d "git+https://github.com/user/repo.git"
```

#### Specific Branch
Serve the `develop` branch:
```bash
gomddoc -d "git+https://github.com/user/repo.git#develop"
```

#### Specific Subdirectory
Serve only the `/docs` folder from the `main` branch:
```bash
gomddoc -d "git+https://github.com/user/repo.git#main:docs"
```

### Private Repositories (SSH Authentication)

To serve private repositories, you must use the SSH protocol and provide a private key.

**Requirement:** You must specify the key file using the `--git-key-file` flag or `GOMDDOC_GIT_SSH_KEY_FILE` environment variable.

1.  **Generate or Locate a Key:** Ensure you have a deploy key or user key that has read access to the repository.
2.  **Run with Key:**

```bash
gomddoc \
  -d "git+ssh://git@github.com/my-org/private-docs.git" \
  --git-key-file ~/.ssh/id_rsa_deploy_key
```

**Environment Variable Example:**

```bash
export GOMDDOC_GIT_SSH_KEY_FILE=/etc/secrets/ssh-key
gomddoc -d "git+ssh://git@gitlab.company.com/group/project.git"
```

> **Note:** The SSH Agent and default `~/.ssh/` lookup are explicitly disabled for security and predictability. You *must* provide the key file path if using SSH.

```