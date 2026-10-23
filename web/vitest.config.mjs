import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    // Shared modules import the helper by its served URL.
    alias: {
      "/_assets/gmd-i18n.mjs": fileURLToPath(
        new URL(
          "../cmd/gomddoc/assets/shared/static/gmd-i18n.mjs",
          import.meta.url,
        ),
      ),
    },
  },
  test: {
    environment: "happy-dom",
    include: ["unit/**/*.test.mjs"],
  },
});
