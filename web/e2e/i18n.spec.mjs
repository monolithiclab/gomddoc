// End-to-end test of client-side strings (gmd-i18n.mjs): on a fr-FR page the
// code-copy button, the search dialog and the annotations header button show
// the site locale's French strings, read from the page's data-gmd-i18n blocks.
import { expect, test } from "@playwright/test";

const base = process.env.GMD_E2E_URL;

// Tests never touch the network: anything but the preview server is refused.
test.beforeEach(async ({ context }) => {
  await context.route(
    (url) => url.origin !== base,
    (route) => route.abort(),
  );
});

test("a fr-FR page shows the site locale's French strings", async ({ page }) => {
  await page.goto(`${base}/fr-FR/`);
  await expect(page.locator("html")).toHaveAttribute("lang", "fr-FR");
  await expect(page.locator("pre > .copy-btn")).toHaveText("Copier");
  await expect(page.locator("#search-input")).toHaveAttribute(
    "placeholder",
    "Rechercher dans la documentation...",
  );
  await expect(page.locator("gmd-annotations-toggle button")).toHaveAttribute(
    "aria-label",
    "Notes de lecture",
  );
});
