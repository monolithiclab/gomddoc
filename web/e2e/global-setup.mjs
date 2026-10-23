// Builds the binary, copies the fixture into a temporary directory (a test
// edits a page there) and serves it with `gomddoc preview`. Tests read the
// server's address and the copy's path from GMD_E2E_URL and GMD_E2E_DIR.
import { execFileSync, spawn } from "node:child_process";
import { cpSync, mkdtempSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const repo = resolve(import.meta.dirname, "../..");

function freePort() {
  return new Promise((done, fail) => {
    const server = createServer().once("error", fail);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close(() => done(port));
    });
  });
}

// ready resolves once the server answers, and rejects if it exits first or
// stays silent for 10 s. It polls: the server logs its start before it
// listens.
async function ready(child, url) {
  let output = "";
  let exited = null;
  child.stdout.on("data", (chunk) => (output += chunk));
  child.stderr.on("data", (chunk) => (output += chunk));
  child.once("exit", (code) => (exited = code ?? "signal"));
  for (const deadline = Date.now() + 10_000; Date.now() < deadline;) {
    if (exited !== null) break;
    try {
      if ((await fetch(url)).ok) return;
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 25));
  }
  child.kill();
  throw new Error(`gomddoc preview did not start (${exited}):\n${output}`);
}

export default async function globalSetup() {
  const binary = join(repo, ".build/gomddoc");
  execFileSync("go", ["build", "-o", binary, "./cmd/gomddoc"], {
    cwd: repo,
    stdio: "inherit",
  });
  const dir = mkdtempSync(join(tmpdir(), "gomddoc-e2e-"));
  cpSync(join(import.meta.dirname, "fixture"), dir, { recursive: true });
  const port = await freePort();
  const child = spawn(binary, ["preview", dir, "--port", `127.0.0.1:${port}`], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  const url = `http://127.0.0.1:${port}`;
  await ready(child, url);
  process.env.GMD_E2E_URL = url;
  process.env.GMD_E2E_DIR = dir;
  return () => {
    child.kill();
    rmSync(dir, { recursive: true, force: true });
  };
}
