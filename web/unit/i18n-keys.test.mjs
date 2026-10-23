import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// Key drift: a key a shared module asks for but en-US.yml lacks reaches the
// page as its raw name, and nothing else fails. Every literal key the modules
// pass to their lookups must ship, under the namespace the module reads (the
// server sends a module only its namespace's keys), and every key the spec's
// namespace table promises must ship too.

const repo = join(import.meta.dirname, "../..");
const shared = join(repo, "cmd/gomddoc/assets/shared");

const shipped = new Set(
  readFileSync(join(repo, "cmd/gomddoc/assets/locales/en-US.yml"), "utf8")
    .split("\n")
    .map((line) => line.match(/^([a-z_]+):/)?.[1])
    .filter(Boolean),
);

// The lookups as the modules name them: t and tn from gmd-i18n.mjs, and the
// escaping wrappers over t (search.mjs's label, gmd-annotations.mjs's E).
const LOOKUP = /\b(?:t|tn|label|E)\(\s*["']([a-z_]+)["']/g;
const NAMESPACE = /\bi18n\(\s*["']([a-z_]+)["']/g;

const modules = readdirSync(shared)
  .filter((f) => f.endsWith(".mjs"))
  .map((file) => {
    const src = readFileSync(join(shared, file), "utf8");
    return {
      file,
      namespaces: [...src.matchAll(NAMESPACE)].map((m) => m[1]),
      keys: [...src.matchAll(LOOKUP)].map((m) => m[1]),
    };
  })
  .filter((m) => m.namespaces.length);

// The spec's namespace table (docs/specs/20261023.client-i18n-design.md).
const NAMESPACE_TABLE = {
  code_copy: ["code_copy_label", "code_copy_done", "code_copy_aria"],
  search: ["search_label", "search_query", "search_results"],
  color_chip: ["color_chip_label", "color_chip_done"],
};

describe("client string keys", () => {
  it("finds the modules that read strings", () => {
    expect(modules.map((m) => m.file).sort()).toEqual([
      "code-copy.mjs",
      "gmd-annotations.mjs",
      "gmd-color-chip.mjs",
      "search.mjs",
    ]);
  });

  for (const { file, namespaces, keys } of modules) {
    it(`${file}: every key ships under its namespace`, () => {
      expect(namespaces).toHaveLength(1);
      expect(keys.length).toBeGreaterThan(0);
      for (const key of keys) {
        expect(shipped, `${key} missing from en-US.yml`).toContain(key);
        expect(key.startsWith(namespaces[0]), `${key} outside ${namespaces[0]}`).toBe(true);
      }
    });
  }

  it("ships every key the namespace table lists", () => {
    const namespaces = modules.flatMap((m) => m.namespaces);
    for (const [ns, keys] of Object.entries(NAMESPACE_TABLE)) {
      expect(namespaces).toContain(ns);
      for (const key of keys) expect(shipped, key).toContain(key);
    }
  });
});
