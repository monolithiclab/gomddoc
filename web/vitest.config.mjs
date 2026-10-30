import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    // Shared modules import the helper by its served URL, and KaTeX from the
    // CDN, which tests never reach: a stub stands in.
    alias: [
      {
        find: "/_assets/gmd-i18n.mjs",
        replacement: fileURLToPath(
          new URL(
            "../cmd/gomddoc/assets/shared/static/gmd-i18n.mjs",
            import.meta.url,
          ),
        ),
      },
      {
        find: /^https:\/\/cdn\.jsdelivr\.net\/npm\/katex@[^/]+\/dist\/katex\.mjs$/,
        replacement: fileURLToPath(
          new URL("unit/katex-stub.mjs", import.meta.url),
        ),
      },
    ],
  },
  test: {
    environment: "happy-dom",
    include: ["unit/**/*.test.mjs"],
  },
});
