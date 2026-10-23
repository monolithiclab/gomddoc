// End-to-end tests of page annotations (gmd-annotations.mjs) in the default
// theme, against the preview server global-setup.mjs starts. Every test gets
// a fresh browser context, so localStorage starts empty; they wait on DOM
// state, never on time.
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const base = process.env.GMD_E2E_URL;

// Playwright's CSS locators pierce the margin's open shadow root.
const pen = (page) => page.locator("gmd-annotation-margin [part=pen]");
const icons = (page) => page.locator("gmd-annotation-margin [part=icon]");
const popover = (page) => page.locator("gmd-annotation-editor");
const dialog = (page) => page.locator("dialog#gmd-annotations-dialog");
const toggle = (page) => page.locator("gmd-annotations-toggle button");
const count = (page) => page.locator(".gmd-annotations-count");
const action = (root, name) => root.locator(`[data-action="${name}"]`);
const highlights = (page) =>
  page.evaluate(() => CSS.highlights.get("gmd-annotation")?.size ?? 0);

// Tests never touch the network: anything but the preview server is refused
// (the theme links Google Fonts).
test.beforeEach(async ({ context }) => {
  await context.route(
    (url) => url.origin !== base,
    (route) => route.abort(),
  );
});

// open loads a page and waits for the interface to mount (after load).
async function open(page, path) {
  await page.goto(base + path);
  await expect(page.locator("gmd-annotation-margin")).toHaveCount(1);
}

// select drags the mouse across the first occurrence of text in the content,
// as a reader would.
async function select(page, text) {
  const box = await page.evaluate((text) => {
    const root = document.querySelector("gmd-annotations");
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    for (let node; (node = walker.nextNode());) {
      const at = node.data.indexOf(text);
      if (at < 0) continue;
      const range = document.createRange();
      range.setStart(node, at);
      range.setEnd(node, at + text.length);
      const rects = range.getClientRects();
      const [first, last] = [rects[0], rects[rects.length - 1]];
      return {
        x1: first.left + 1,
        y1: first.top + first.height / 2,
        x2: last.right - 1,
        y2: last.top + last.height / 2,
      };
    }
    return null;
  }, text);
  expect(box, `"${text}" on the page`).not.toBeNull();
  await page.mouse.move(box.x1, box.y1);
  await page.mouse.down();
  await page.mouse.move(box.x2, box.y2, { steps: 4 });
  await page.mouse.up();
  await expect
    .poll(() => page.evaluate(() => getSelection().toString()))
    .toBe(text);
}

async function annotate(page, text, note) {
  await select(page, text);
  await pen(page).click();
  await expect(popover(page)).toBeVisible();
  await popover(page).locator("textarea").fill(note);
  await action(popover(page), "save").click();
  await expect(popover(page)).toBeHidden();
}

test("create, edit and delete a note", async ({ page }) => {
  await open(page, "/");

  // Esc on a new note: the selection collapsed when the textarea took focus,
  // so the pen is gone and focus lands on the content.
  await select(page, "bold claim");
  await pen(page).click();
  await popover(page).locator("textarea").fill("draft");
  await page.keyboard.press("Escape");
  await expect(popover(page)).toBeHidden();
  await expect(page.locator("gmd-annotations")).toBeFocused();
  await expect(icons(page)).toHaveCount(0);

  // A passage whose context repeats elsewhere on the page gets no pen.
  await select(page, "quick brown");
  await expect(pen(page)).toBeVisible();
  await select(page, "word for word");
  await expect(pen(page)).toBeHidden();

  await annotate(page, "bold claim", "Source?");
  await expect(icons(page)).toHaveCount(1);
  await expect.poll(() => highlights(page)).toBe(1);
  await expect(count(page)).toHaveText("1");

  await icons(page).click();
  const textarea = popover(page).locator("textarea");
  await expect(textarea).toHaveValue("Source?");
  await textarea.fill("Needs a source.");
  await action(popover(page), "save").click();
  await icons(page).click();
  await expect(textarea).toHaveValue("Needs a source.");

  await action(popover(page), "delete").click();
  await expect(icons(page)).toHaveCount(0);
  await expect.poll(() => highlights(page)).toBe(0);
  await expect(count(page)).toBeHidden();
});

test("two notes on one line: one icon, prev/next", async ({ page }) => {
  await open(page, "/");
  await annotate(page, "quick brown", "first");
  await annotate(page, "lazy dog", "second");
  await expect(icons(page)).toHaveCount(1);
  await expect(icons(page)).toHaveAttribute("data-count", "2");
  await expect.poll(() => highlights(page)).toBe(2);

  await icons(page).click();
  const position = popover(page).locator(".gmd-annotation-position");
  const textarea = popover(page).locator("textarea");
  await expect(position).toHaveText("1 of 2");
  await expect(textarea).toHaveValue("first");
  await expect(action(popover(page), "prev")).toBeDisabled();
  await action(popover(page), "next").click();
  await expect(position).toHaveText("2 of 2");
  await expect(textarea).toHaveValue("second");
  await expect(action(popover(page), "next")).toBeDisabled();
  await action(popover(page), "prev").click();
  await expect(textarea).toHaveValue("first");
});

test("the editor and the dialog use the top layer", async ({ page }) => {
  await open(page, "/");
  await annotate(page, "bold claim", "Top layer.");

  await icons(page).click();
  await expect(popover(page)).toBeVisible();
  expect(
    await popover(page).evaluate((el) => el.matches(":popover-open")),
  ).toBe(true);
  // Anchored under its icon by CSS (no script fallback styles), right edges
  // aligned.
  await expect(popover(page)).not.toHaveAttribute("style");
  const [icon, editor] = await Promise.all([
    icons(page).boundingBox(),
    popover(page).boundingBox(),
  ]);
  expect(editor.y).toBeGreaterThanOrEqual(icon.y + icon.height);
  expect(
    Math.abs(editor.x + editor.width - (icon.x + icon.width)),
  ).toBeLessThan(2);
  // Non-modal: Tab leaves the textarea for the editor's buttons, and a click
  // outside dismisses it.
  await page.keyboard.press("Tab");
  await expect(action(popover(page), "delete")).toBeFocused();
  await page.mouse.click(5, 5);
  await expect(popover(page)).toBeHidden();

  await toggle(page).click();
  await expect(dialog(page)).toBeVisible();
  expect(await dialog(page).evaluate((el) => el.matches(":modal"))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(dialog(page)).toBeHidden();
  await expect(toggle(page)).toBeFocused();
});

test("header dialog copies the export and clear all asks first", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"], {
    origin: base,
  });
  await open(page, "/");
  await annotate(page, "bold claim", "Cite the study.");

  await toggle(page).click();
  await expect(dialog(page)).toBeVisible();
  await action(dialog(page), "copy").click();
  await expect(action(dialog(page), "copy")).toHaveText("Copied");
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  expect(clipboard).toBe(
    [
      "# Review notes: Home",
      "",
      "Source file: `README.md`",
      `Page: ${base}/`,
      "",
      "Each note quotes the passage it refers to, with the words just before and after it to disambiguate.",
      "",
      "## 1. Annotation fixture",
      "",
      "> … fox jumps over the lazy dog. A **bold claim** sits in this paragraph too. Rep…",
      "",
      "Cite the study.",
      "",
    ].join("\n"),
  );

  await action(dialog(page), "clear").click();
  await expect(action(dialog(page), "cancel-clear")).toBeFocused();
  await expect.poll(() => highlights(page)).toBe(1);
  await action(dialog(page), "confirm-clear").click();
  await expect.poll(() => highlights(page)).toBe(0);
  await expect(icons(page)).toHaveCount(0);
  await action(dialog(page), "close").click();
  await expect(dialog(page)).toBeHidden();
});

test("editing the source drops a stale note and shows the notice", async ({
  page,
}) => {
  const file = join(process.env.GMD_E2E_DIR, "edited.md");
  const source = readFileSync(file, "utf8");
  await open(page, "/edited");
  await annotate(page, "rewrites this sentence", "Stale soon.");
  await expect(icons(page)).toHaveCount(1);

  writeFileSync(file, source.replace("rewrites this sentence", "edits it"));
  try {
    await page.reload();
    await expect(page.locator("gmd-annotation-margin")).toHaveCount(1);
    const message = "1 note no longer matches this page and was removed.";
    await expect(page.locator("gmd-annotation-notice span")).toHaveText(
      message,
    );
    await expect(page.getByRole("status")).toHaveText(message);
    await expect(icons(page)).toHaveCount(0);
    await expect.poll(() => highlights(page)).toBe(0);
  } finally {
    writeFileSync(file, source);
  }
});

test("note in a heading section survives reload", async ({ page }) => {
  await open(page, "/");
  await annotate(page, "package manager", "Which ones?");
  await page.reload();
  await expect(page.locator("gmd-annotation-margin")).toHaveCount(1);
  await expect(icons(page)).toHaveCount(1);
  await expect.poll(() => highlights(page)).toBe(1);

  await toggle(page).click();
  await expect(dialog(page).locator("textarea")).toHaveValue(
    /## 1\. Annotation fixture › Setup section\n\n> .*\*\*package manager\*\*.*\n\nWhich ones\?\n$/,
  );
});

test("feature off renders no annotation markup", async ({ page }) => {
  await page.goto(base + "/off");
  await expect(page.locator("h1")).toContainText("Annotations off");
  for (const selector of [
    "gmd-annotations",
    "gmd-annotations-toggle",
    "#gmd-annotations-config",
    "gmd-annotation-margin",
    "gmd-annotation-editor",
    "gmd-annotation-notice",
    "dialog",
  ]) {
    await expect(page.locator(selector)).toHaveCount(0);
  }
});

test("axe finds no violations with the popover open and with the dialog open", async ({
  page,
}) => {
  const axe = () => new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]);
  await open(page, "/");
  await annotate(page, "bold claim", "A note.");

  await icons(page).click();
  await expect(popover(page)).toBeVisible();
  expect((await axe().analyze()).violations).toEqual([]);
  await page.keyboard.press("Escape");
  await expect(popover(page)).toBeHidden();

  await toggle(page).click();
  await expect(dialog(page)).toBeVisible();
  expect((await axe().analyze()).violations).toEqual([]);
});
