// End-to-end test of the table of contents sidebar (toc-highlight.mjs): on a
// page whose TOC overflows, clicking a link keeps that link active and in view
// instead of scrolling the TOC back to the top.
import { expect, test } from "@playwright/test";

const base = process.env.GMD_E2E_URL;

// Tests never touch the network: anything but the preview server is refused.
test.beforeEach(async ({ context }) => {
  await context.route(
    (url) => url.origin !== base,
    (route) => route.abort(),
  );
});

test.use({ viewport: { width: 1280, height: 720 } });

for (const n of [28, 14]) {
  test(`clicking the TOC link for section ${n} keeps it active and in view`, async ({ page }) => {
    await page.goto(`${base}/long-toc`);
    const sidebar = page.locator("#toc-sidebar");
    const link = sidebar.locator(`a[href="#section-${n}"]`);
    await link.click();

    // Wait for the page scroll to settle (smooth scrolling), then the TOC's.
    await expect(link).toHaveClass(/active/);
    await expect
      .poll(async () => {
        const a = await page.evaluate(() => window.scrollY);
        await page.waitForTimeout(200);
        return a === (await page.evaluate(() => window.scrollY));
      })
      .toBe(true);

    await expect
      .poll(async () => {
        const s = await sidebar.boundingBox();
        const l = await link.boundingBox();
        return l.y >= s.y && l.y + l.height <= s.y + s.height;
      })
      .toBe(true);
    expect(await sidebar.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  });
}
