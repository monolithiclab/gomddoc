---
title: "Security Features Guide"
description: "Overview of gomddoc's security features, including path traversal protection and hidden file blocking."
author: "nicolasm"
---

# Security

gomddoc is designed to be secure enough to be exposed to the internet, although running it behind a reverse proxy (like Nginx or Cloudflare) is recommended for TLS and DDoS protection.

## 1. Path Traversal Protection
We use Go's `os.DirFS` (and internal equivalents for Git) to create a "jail" around the content directory. It is impossible for a user to request `../../etc/passwd`. The file system provider strictly limits access to the root directory specified by `-d`.

## 2. Hidden File Blocking
gomddoc automatically blocks HTTP access to "hidden" files and directories (those starting with a dot `.`).

*   **Blocked:** `.env`, `.git/`, `.gomddoc/`, `.ssh/`, `.config/`
*   **Allowed:** `/.well-known/` (Standard for SSL verification and security.txt)

This prevents accidental exposure of configuration files, secrets, or git history.

## 3. Git Isolation
When serving from a Git repository:
*   The repository is cloned into **memory**. It is never written to disk, preventing residue data.
*   SSH keys used for authentication are handled in memory by the Go process and are not accessible via the HTTP interface or file system operations.

## 4. HTTP Headers
The server adds standard security headers to every response:
*   `X-Content-Type-Options: nosniff`
*   `X-Frame-Options: DENY`

## 5. Timeouts
To protect against Slowloris attacks and resource exhaustion, the server has default timeouts:
*   **Read Header:** 5 seconds
*   **Write:** 30 seconds
*   **Idle:** 120 seconds
