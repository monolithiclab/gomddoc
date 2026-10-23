import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
// platform.mjs first: it adds the features the module checks for before
// defining its elements.
import { deepActive, popoverOpen } from "./platform.mjs";
import {
  buildTextModel,
  platformSupported,
  selectorFromRange,
  storageKey,
} from "../../cmd/gomddoc/assets/shared/gmd-annotations.mjs";

const LABELS = {
  annotation_add: "Annotate selection",
  annotation_note: "Note",
  annotation_save: "Save",
  annotation_discard: "Discard",
  annotation_delete: "Delete",
  annotation_prev: "Previous note",
  annotation_next: "Next note",
  annotation_position: "%d of %d",
  annotations_open: "Annotations",
  annotations_count: "%d notes",
  annotations_count_one: "1 note",
  annotations_open_count: "Annotations (%d notes)",
  annotations_open_count_one: "Annotations (1 note)",
  annotations_copy: "Copy",
  annotations_copied: "Copied",
  annotations_clear: "Clear all",
  annotations_clear_confirm: "Delete every note on this page?",
  annotations_clear_yes: "Delete all",
  annotations_cancel: "Cancel",
  annotations_close: "Close",
  annotations_empty: "No notes on this page yet.",
  annotations_removed: "%d notes no longer match this page and were removed.",
  annotations_removed_one:
    "1 note no longer matches this page and was removed.",
  annotations_not_kept:
    "Notes can't be saved in this browser and will be lost on reload.",
  annotations_export_title: "Review notes",
  annotations_export_source: "Source file",
  annotations_export_page: "Page",
  annotations_export_intro: "Each note quotes the passage.",
  annotations_export_top: "(top of page)",
};

const PAGE =
  "<h1>Spec</h1><p id=a>alpha beta gamma delta</p><h2>Part</h2><p id=b>one two three four</p>";

// Line tops for getBoundingClientRect: the paragraph a range starts in
// decides its visual line.
const TOPS = { a: 100, b: 200 };

let highlights;

// page renders a page as the default theme does: the toggle in the header,
// the content wrapped in gmd-annotations (unless wrapped is false), and the
// configuration. Connecting the wrapper mounts the interface; a second call
// replaces the page, as a reload does.
function page({ wrapped = true, html = PAGE } = {}) {
  const config = { sourcePath: "docs/spec.md", title: "Spec", labels: LABELS };
  const content = wrapped
    ? `<gmd-annotations>${html}</gmd-annotations>`
    : `<article>${html}</article>`;
  // Through a template, so the elements connect with their children in
  // place, as when the deferred module upgrades a parsed page.
  const template = document.createElement("template");
  template.innerHTML =
    `<script type="application/json" id="gmd-annotations-config">${JSON.stringify(config)}</script>` +
    `<header><gmd-annotations-toggle></gmd-annotations-toggle></header>${content}`;
  document.body.replaceChildren(template.content);
  return $("gmd-annotations, article");
}

const tick = () => new Promise((r) => setTimeout(r, 0));
// frame waits past the next animation frame, where the module handles
// selectionchange.
const frame = () =>
  new Promise((r) => requestAnimationFrame(() => setTimeout(r, 0)));
const $ = (s) => document.querySelector(s);
const $$ = (s) => [...document.querySelectorAll(s)];
const key = () => storageKey(window.location);
const stored = () => JSON.parse(localStorage.getItem(key()))?.notes ?? [];

const margin = () => $("gmd-annotation-margin");
const pen = () => margin().shadowRoot.querySelector('[part="pen"]');
const icons = () => [...margin().shadowRoot.querySelectorAll('[part="icon"]')];
const editor = () => $("gmd-annotation-editor");
const textarea = () => editor().querySelector("textarea");
const dialog = () => $("dialog#gmd-annotations-dialog");
const toggle = () => $("gmd-annotations-toggle button");
const action = (root, name) => root.querySelector(`[data-action=${name}]`);

// press focuses a button and clicks it, as a pointer does in a browser.
function press(button) {
  button.focus();
  button.click();
}

// select puts text a..b of the paragraph's text node into the selection.
async function select(id, a, b) {
  const text = document.getElementById(id).firstChild;
  const range = document.createRange();
  range.setStart(text, a);
  range.setEnd(text, b);
  const sel = window.getSelection();
  sel.removeAllRanges();
  sel.addRange(range);
  await frame();
}

async function collapse() {
  window.getSelection().removeAllRanges();
  await frame();
}

function keydown(target, k, mods = {}) {
  const event = new KeyboardEvent("keydown", {
    key: k,
    bubbles: true,
    cancelable: true,
    ...mods,
  });
  target.dispatchEvent(event);
  return event;
}

async function addNote(id, a, b, text) {
  await select(id, a, b);
  press(pen());
  textarea().value = text;
  action(editor(), "save").click();
}

beforeEach(() => {
  localStorage.clear();
  highlights = new Map();
  // happy-dom has no Custom Highlight API and no CSS.supports; CSS itself is
  // read-only. Without supports, the editor takes its JavaScript placement.
  vi.stubGlobal("CSS", { highlights });
  vi.stubGlobal(
    "Highlight",
    class {
      constructor(...ranges) {
        this.ranges = ranges;
      }
    },
  );
  vi.spyOn(Range.prototype, "getBoundingClientRect").mockImplementation(
    function () {
      const p = this.startContainer.parentElement?.closest("p");
      return {
        top: TOPS[p?.id] ?? 0,
        bottom: 0,
        left: 0,
        right: 0,
        width: 0,
        height: 0,
      };
    },
  );
});

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("elements", () => {
  it("does nothing without the gmd-annotations wrapper", () => {
    page({ wrapped: false });
    for (const name of [
      "gmd-annotation-margin",
      "gmd-annotation-editor",
      "gmd-annotation-notice",
      "dialog",
    ]) {
      expect($$(name)).toHaveLength(0);
    }
    expect(toggle().getAttribute("aria-label")).toBe("Annotations");
  });

  it("mounts its interface in the body and removes it on disconnect", () => {
    const content = page();
    expect(margin().shadowRoot).not.toBeNull();
    expect(editor().getAttribute("popover")).toBe("auto");
    expect($("gmd-annotation-notice").getAttribute("popover")).toBe("manual");
    expect(dialog()).not.toBeNull();
    const counts = [];
    document.addEventListener("gmd-annotations-change", (e) =>
      counts.push(e.detail.count),
    );
    content.remove();
    expect($$("gmd-annotation-margin, gmd-annotation-editor")).toHaveLength(0);
    expect(dialog()).toBeNull();
    expect($(".gmd-annotation-status")).toBeNull();
    expect(counts).toEqual([0]);
  });

  it("needs the Popover API and Invoker Commands to define anything", () => {
    const fake = (element, button) => ({
      HTMLElement: { prototype: element },
      HTMLButtonElement: { prototype: button },
    });
    const popover = { showPopover() {} };
    const invoker = { commandForElement: null };
    expect(platformSupported(fake({}, invoker))).toBe(false);
    expect(platformSupported(fake(popover, {}))).toBe(false);
    expect(platformSupported(fake(popover, invoker))).toBe(true);
    expect(platformSupported(window)).toBe(true);
  });

  it("the toggle completes pre-filled markup", () => {
    page();
    const host = document.createElement("gmd-annotations-toggle");
    host.innerHTML = '<button type="button">Notes</button>';
    document.body.append(host);
    const badge = host.querySelector("button .gmd-annotations-count");
    expect(badge.hidden).toBe(true);
    expect(host.querySelector("button").getAttribute("aria-label")).toBe(
      "Annotations",
    );
  });

  it("defines each element once when the module runs twice", async () => {
    const before = customElements.get("gmd-annotations");
    await import("../../cmd/gomddoc/assets/shared/gmd-annotations.mjs?again");
    expect(customElements.get("gmd-annotations")).toBe(before);
  });

  it("the toggle opens the dialog through Invoker Commands", () => {
    page();
    expect(toggle().getAttribute("commandfor")).toBe("gmd-annotations-dialog");
    expect(toggle().getAttribute("command")).toBe("show-modal");
    expect(action(dialog(), "close").getAttribute("command")).toBe("close");
  });
});

describe("creating a note", () => {
  beforeEach(() => page());

  it("selection shows the pen, collapse hides it", async () => {
    expect(pen().hidden).toBe(true);
    await select("a", 6, 10);
    expect(pen().hidden).toBe(false);
    expect(pen().getAttribute("aria-label")).toBe("Annotate selection");
    await collapse();
    expect(pen().hidden).toBe(true);
  });

  it("rejects a selection outside the content", async () => {
    const outside = document.createElement("p");
    outside.id = "out";
    outside.textContent = "elsewhere";
    document.body.append(outside);
    await select("out", 0, 4);
    expect(pen().hidden).toBe(true);
  });

  it("save stores a note, shows one icon and announces the count", async () => {
    const counts = [];
    document.addEventListener("gmd-annotations-change", (e) =>
      counts.push(e.detail.count),
    );
    await select("a", 6, 10);
    press(pen());
    expect(popoverOpen(editor())).toBe(true);
    expect(deepActive()).toBe(textarea());
    textarea().value = "Why beta?";
    action(editor(), "save").click();

    expect(popoverOpen(editor())).toBe(false);
    expect(stored()).toMatchObject([{ quote: "beta", note: "Why beta?" }]);
    expect(icons()).toHaveLength(1);
    expect(icons()[0].dataset.count).toBe("1");
    expect(deepActive()).toBe(icons()[0]);
    expect(highlights.get("gmd-annotation").ranges.map(String)).toEqual([
      "beta",
    ]);
    expect(pen().hidden).toBe(true);
    expect(counts.at(-1)).toBe(1);
    expect($("gmd-annotations").count).toBe(1);
  });

  it("save with whitespace discards", async () => {
    await addNote("a", 6, 10, "  \n ");
    expect(localStorage.getItem(key())).toBeNull();
    expect(icons()).toHaveLength(0);
    expect(popoverOpen(editor())).toBe(false);
  });

  it("Escape discards a new note and returns focus to the pen", async () => {
    await select("a", 6, 10);
    press(pen());
    textarea().value = "draft";
    keydown(textarea(), "Escape");
    expect(popoverOpen(editor())).toBe(false);
    expect(localStorage.getItem(key())).toBeNull();
    expect(deepActive()).toBe(pen());
  });

  it("Escape after the selection collapsed focuses the content", async () => {
    await select("a", 6, 10);
    press(pen());
    // A browser collapses the selection when the textarea takes focus.
    await collapse();
    keydown(textarea(), "Escape");
    expect(pen().hidden).toBe(true);
    expect(deepActive()).toBe($("gmd-annotations"));
  });

  it("Ctrl+Enter saves", async () => {
    await select("a", 6, 10);
    press(pen());
    textarea().value = "quick";
    keydown(textarea(), "Enter", { ctrlKey: true });
    expect(stored()).toMatchObject([{ note: "quick" }]);
  });

  it("Alt+Shift+A opens the editor for the selection", async () => {
    await select("b", 4, 7);
    keydown(document.body, "Å", { code: "KeyA", altKey: true, shiftKey: true });
    expect(popoverOpen(editor())).toBe(true);
    expect(deepActive()).toBe(textarea());
  });

  it("the editor is non-modal: no aria-modal, Tab is not trapped", async () => {
    await select("a", 6, 10);
    press(pen());
    expect(editor().getAttribute("role")).toBe("dialog");
    expect(editor().hasAttribute("aria-modal")).toBe(false);
    expect(keydown(action(editor(), "save"), "Tab").defaultPrevented).toBe(
      false,
    );
  });

  it("without CSS anchor positioning the editor is placed by script", async () => {
    await select("a", 6, 10);
    press(pen());
    expect(editor().style.position).toBe("absolute");
  });

  it("ignored showPopover source falls back to script placement", async () => {
    vi.stubGlobal("CSS", { highlights, supports: () => true });
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
      function () {
        // The pen at the margin; the editor left where the UA puts it.
        return this.localName === "gmd-annotation-editor"
          ? { top: 300, bottom: 400, left: 300, right: 620 }
          : { top: 100, bottom: 128, left: 600, right: 628 };
      },
    );
    await select("a", 6, 10);
    press(pen());
    expect(editor().style.position).toBe("absolute");
    expect(editor().style.top).toBe(`${128 + 4}px`);
  });

  it("script placement flips above the opener near the viewport bottom", async () => {
    vi.spyOn(pen(), "getBoundingClientRect").mockReturnValue({
      top: 700,
      bottom: 728,
      left: 600,
      right: 628,
    });
    vi.stubGlobal("innerHeight", 768);
    Object.defineProperty(editor(), "offsetHeight", { value: 200 });
    await select("a", 6, 10);
    press(pen());
    expect(editor().style.top).toBe(`${700 - 4 - 200}px`);
  });

  it("with CSS anchor positioning the stylesheet places it", async () => {
    vi.stubGlobal("CSS", { highlights, supports: () => true });
    await select("a", 6, 10);
    press(pen());
    expect(editor().style.position).toBe("");
  });
});

describe("existing notes", () => {
  beforeEach(() => page());

  it("two notes on one line share an icon with count 2 and prev/next cycles them", async () => {
    await addNote("a", 11, 16, "second");
    await addNote("a", 0, 5, "first");
    await addNote("b", 4, 7, "other line");

    expect(icons().map((i) => i.dataset.count)).toEqual(["2", "1"]);
    expect(icons()[0].getAttribute("aria-label")).toBe("2 notes");

    press(icons()[0]);
    const position = editor().querySelector(".gmd-annotation-position");
    expect(textarea().value).toBe("first");
    expect(position.textContent).toBe("1 of 2");
    expect(action(editor(), "prev").disabled).toBe(true);
    action(editor(), "next").click();
    expect(textarea().value).toBe("second");
    expect(position.textContent).toBe("2 of 2");
    expect(action(editor(), "next").disabled).toBe(true);
    action(editor(), "prev").click();
    expect(textarea().value).toBe("first");
  });

  it("editing an existing note to empty deletes it", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    textarea().value = " ";
    action(editor(), "save").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect(icons()).toHaveLength(0);
    expect(highlights.has("gmd-annotation")).toBe(false);
  });

  it("Save updates the text", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    textarea().value = "changed";
    action(editor(), "save").click();
    expect(stored()).toMatchObject([{ note: "changed" }]);
  });

  it("Delete removes it", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    expect(action(editor(), "discard").hidden).toBe(true);
    action(editor(), "delete").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect(icons()).toHaveLength(0);
  });

  it("Escape keeps the stored text and returns focus to the icon", async () => {
    await addNote("a", 6, 10, "note");
    const icon = icons()[0];
    press(icon);
    expect(icon.getAttribute("aria-expanded")).toBe("true");
    textarea().value = "unsaved";
    keydown(textarea(), "Escape");
    expect(popoverOpen(editor())).toBe(false);
    expect(stored()).toMatchObject([{ note: "note" }]);
    expect(deepActive()).toBe(icon);
    expect(icon.getAttribute("aria-expanded")).toBe("false");
  });

  it("a second click on the open note's icon closes the editor", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    press(icons()[0]);
    expect(popoverOpen(editor())).toBe(false);
  });
});

describe("focus", () => {
  beforeEach(() => page());

  it("a layout keeps the open note's icon, so Escape returns to it", async () => {
    await addNote("a", 6, 10, "note");
    const before = icons()[0];
    press(before);
    window.dispatchEvent(new Event("resize"));
    await new Promise((r) => requestAnimationFrame(r));
    expect(icons()[0]).toBe(before);
    expect(before.getAttribute("aria-expanded")).toBe("true");
    keydown(textarea(), "Escape");
    expect(deepActive()).toBe(before);
  });

  it("deleting the last note moves focus to the header toggle", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    action(editor(), "delete").click();
    expect(deepActive()).toBe(toggle());
  });

  it("deleting the last note without a toggle focuses the content", async () => {
    $("gmd-annotations-toggle").remove();
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    action(editor(), "delete").click();
    expect(deepActive()).toBe($("gmd-annotations"));
  });

  it("light dismiss (platform shim) reaches beforetoggle: nothing stored", async () => {
    await select("a", 6, 10);
    press(pen());
    document.body.dispatchEvent(
      new PointerEvent("pointerdown", { bubbles: true, composed: true }),
    );
    expect(popoverOpen(editor())).toBe(false);
    expect(localStorage.getItem(key())).toBeNull();
  });

  it("Escape closes the dialog and focus returns to the toggle", async () => {
    await addNote("a", 6, 10, "note");
    press(toggle());
    expect(dialog().open).toBe(true);
    expect(document.activeElement).toBe(dialog().querySelector("textarea"));
    keydown(dialog().querySelector("textarea"), "Escape");
    expect(dialog().open).toBe(false);
    expect(document.activeElement).toBe(toggle());
  });

  it("opening the dialog closes the editor", async () => {
    await addNote("a", 6, 10, "note");
    press(icons()[0]);
    press(toggle());
    expect(popoverOpen(editor())).toBe(false);
    expect(dialog().open).toBe(true);
  });
});

describe("reload", () => {
  it.each([
    [1, "1 note no longer matches this page and was removed."],
    [2, "2 notes no longer match this page and were removed."],
  ])("drops %i stale notes and announces the count", (count, message) => {
    page({ wrapped: false });
    const article = $("article");
    const p = document.getElementById("a").firstChild;
    const range = document.createRange();
    range.setStart(p, 6);
    range.setEnd(p, 10);
    const live = selectorFromRange(buildTextModel(article), range);
    const stale = Array.from({ length: count }, (_, i) => ({
      id: `s${i}`,
      quote: `zeta${i}`,
      prefix: "",
      suffix: "",
      offset: 0,
      note: "gone",
    }));
    localStorage.setItem(
      key(),
      JSON.stringify({
        v: 1,
        notes: [{ id: "1", ...live, note: "keep" }, ...stale],
      }),
    );

    page();

    expect(stored()).toMatchObject([{ id: "1", note: "keep" }]);
    expect(icons()).toHaveLength(1);
    expect(toggle().getAttribute("aria-label")).toBe("Annotations (1 note)");
    const notice = $("gmd-annotation-notice");
    expect(popoverOpen(notice)).toBe(true);
    expect(notice.textContent).toContain(message);
    expect($(".gmd-annotation-status").textContent).toBe(message);

    notice.querySelector("button").click();
    expect(popoverOpen(notice)).toBe(false);
  });

  it("says nothing when every note matches", () => {
    page();
    const notice = $("gmd-annotation-notice");
    expect(popoverOpen(notice)).toBe(false);
    expect(notice.textContent).toBe("");
  });

  it("the live region is rendered and empty before any message", () => {
    page();
    const status = $(".gmd-annotation-status");
    expect(status.getAttribute("role")).toBe("status");
    expect(status.textContent).toBe("");
    expect(status.hidden).toBe(false);
    expect(status.closest("[popover]")).toBeNull();
    expect($("gmd-annotation-notice").hasAttribute("role")).toBe(false);
  });

  it("combines the not-kept and removed messages of one load", () => {
    const entry = JSON.stringify({
      v: 1,
      notes: [
        {
          id: "s",
          quote: "zeta",
          prefix: "",
          suffix: "",
          offset: 0,
          note: "x",
        },
      ],
    });
    vi.stubGlobal("localStorage", {
      getItem: () => entry,
      setItem() {
        throw new Error("quota");
      },
      removeItem() {
        throw new Error("quota");
      },
    });
    page();
    const both =
      "Notes can't be saved in this browser and will be lost on reload. " +
      "1 note no longer matches this page and was removed.";
    expect($(".gmd-annotation-status").textContent).toBe(both);
    expect($("gmd-annotation-notice span").textContent).toBe(both);
  });
});

describe("header", () => {
  beforeEach(() => page());

  it("header dialog shows the export; clear all needs confirm-clear", async () => {
    await addNote("b", 4, 7, "Why two?");
    press(toggle());
    expect(dialog().open).toBe(true);
    const area = dialog().querySelector("textarea");
    expect(document.activeElement).toBe(area);
    expect(area.value).toContain("# Review notes: Spec");
    expect(area.value).toContain("Source file: `docs/spec.md`");
    expect(area.value).toContain("## 1. Spec › Part");
    expect(area.value).toContain("**two**");
    expect(area.value).toContain("Why two?");

    action(dialog(), "clear").click();
    expect(stored()).toHaveLength(1);
    action(dialog(), "cancel-clear").click();
    expect(
      action(dialog(), "confirm-clear").closest("[hidden]"),
    ).not.toBeNull();
    expect(stored()).toHaveLength(1);

    action(dialog(), "clear").click();
    action(dialog(), "confirm-clear").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect(icons()).toHaveLength(0);
    expect(action(dialog(), "copy").disabled).toBe(true);
    expect(dialog().textContent).toContain("No notes on this page yet.");

    action(dialog(), "close").click();
    expect(dialog().open).toBe(false);
    expect(document.activeElement).toBe(toggle());
  });

  it("with no notes the dialog focuses Close", () => {
    press(toggle());
    expect(document.activeElement).toBe(action(dialog(), "close"));
  });

  it("copy writes the textarea's current content", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    vi.spyOn(navigator, "clipboard", "get").mockReturnValue({ writeText });
    await addNote("a", 6, 10, "n");
    press(toggle());
    dialog().querySelector("textarea").value = "edited export";
    action(dialog(), "copy").click();
    await tick();
    expect(writeText).toHaveBeenCalledWith("edited export");
    expect(action(dialog(), "copy").textContent).toBe("Copied");
  });

  it("badge hidden at zero, shows the count otherwise", async () => {
    const badge = $(".gmd-annotations-count");
    expect(badge.hidden).toBe(true);
    await addNote("a", 6, 10, "one");
    expect(toggle().getAttribute("aria-label")).toBe("Annotations (1 note)");
    await addNote("b", 4, 7, "two");
    expect(badge.hidden).toBe(false);
    expect(badge.textContent).toBe("2");
    expect(toggle().getAttribute("aria-label")).toBe("Annotations (2 notes)");
  });
});

describe("storage that throws", () => {
  it("keeps notes in memory and says they will not be kept", async () => {
    vi.stubGlobal("localStorage", {
      getItem: () => null,
      setItem() {
        throw new Error("quota");
      },
      removeItem() {},
    });
    page();
    await addNote("a", 6, 10, "memory");
    expect(icons()).toHaveLength(1);
    expect($("gmd-annotation-notice").textContent).toContain(
      "can't be saved in this browser",
    );
  });
});
