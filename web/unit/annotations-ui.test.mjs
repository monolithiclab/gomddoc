import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  buildTextModel,
  init,
  selectorFromRange,
  storageKey,
} from "../../cmd/gomddoc/assets/shared/gmd-annotations.mjs";

const LABELS = {
  annotate: "Annotate selection",
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

let ui;
let highlights;

function page({ marker = true, html = PAGE } = {}) {
  const config = { sourcePath: "docs/spec.md", title: "Spec", labels: LABELS };
  document.body.innerHTML =
    '<header><button data-gmd-annotations-toggle aria-label="Annotations">' +
    "<span data-gmd-annotations-count hidden></span></button></header>" +
    `<article ${marker ? "data-gmd-annotatable" : ""}>${html}</article>` +
    `<script type="application/json" id="gmd-annotations-config">${JSON.stringify(config)}</script>`;
  return document.querySelector("article");
}

const tick = () => new Promise((r) => setTimeout(r, 0));
const $ = (s) => document.querySelector(s);
const $$ = (s) => [...document.querySelectorAll(s)];
const key = () => storageKey(window.location);
const stored = () => JSON.parse(localStorage.getItem(key()))?.notes ?? [];

// select puts text a..b of the paragraph's text node into the selection.
async function select(id, a, b) {
  const text = document.getElementById(id).firstChild;
  const range = document.createRange();
  range.setStart(text, a);
  range.setEnd(text, b);
  const sel = window.getSelection();
  sel.removeAllRanges();
  sel.addRange(range);
  await tick();
}

async function collapse() {
  window.getSelection().removeAllRanges();
  await tick();
}

function key_(target, k, mods = {}) {
  target.dispatchEvent(
    new KeyboardEvent("keydown", {
      key: k,
      bubbles: true,
      cancelable: true,
      ...mods,
    }),
  );
}

const popover = () => $("[data-gmd-annotation-popover]");
const textarea = () => popover().querySelector("textarea");
const action = (root, name) => root.querySelector(`[data-action=${name}]`);

async function addNote(id, a, b, text) {
  await select(id, a, b);
  $("[data-gmd-annotation-pen]").click();
  textarea().value = text;
  action(popover(), "save").click();
}

beforeEach(() => {
  localStorage.clear();
  highlights = new Map();
  // happy-dom has no Custom Highlight API; CSS itself is read-only.
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
  ui?.destroy();
  ui = null;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("init", () => {
  it("does nothing without the marker", () => {
    page({ marker: false });
    ui = init(document, window);
    expect(ui).toBeNull();
    expect($$("[data-gmd-annotation-ui]")).toHaveLength(0);
    expect(document.documentElement.hasAttribute("data-gmd-annotations")).toBe(
      false,
    );
  });
});

describe("creating a note", () => {
  beforeEach(() => {
    page();
    ui = init(document, window);
  });

  it("selection shows the pen, collapse hides it", async () => {
    const pen = $("[data-gmd-annotation-pen]");
    expect(pen.hidden).toBe(true);
    await select("a", 6, 10);
    expect(pen.hidden).toBe(false);
    expect(pen.getAttribute("aria-label")).toBe("Annotate selection");
    await collapse();
    expect(pen.hidden).toBe(true);
  });

  it("rejects a selection outside the content", async () => {
    const outside = document.createElement("p");
    outside.id = "out";
    outside.textContent = "elsewhere";
    document.body.append(outside);
    await select("out", 0, 4);
    expect($("[data-gmd-annotation-pen]").hidden).toBe(true);
  });

  it("save stores a note and shows one icon", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    expect(popover().hidden).toBe(false);
    expect(document.activeElement).toBe(textarea());
    textarea().value = "Why beta?";
    action(popover(), "save").click();

    expect(popover().hidden).toBe(true);
    expect(stored()).toMatchObject([{ quote: "beta", note: "Why beta?" }]);
    const icons = $$("[data-gmd-annotation-icon]");
    expect(icons).toHaveLength(1);
    expect(icons[0].dataset.count).toBe("1");
    expect(icons[0].hasAttribute("data-gmd-annotation-ui")).toBe(true);
    expect(highlights.get("gmd-annotation").ranges.map(String)).toEqual([
      "beta",
    ]);
    expect($("[data-gmd-annotation-pen]").hidden).toBe(true);
  });

  it("save with whitespace discards", async () => {
    await addNote("a", 6, 10, "  \n ");
    expect(localStorage.getItem(key())).toBeNull();
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(0);
    expect(popover().hidden).toBe(true);
  });

  it("Escape discards a new note", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    textarea().value = "draft";
    key_(textarea(), "Escape");
    expect(popover().hidden).toBe(true);
    expect(localStorage.getItem(key())).toBeNull();
  });

  it("Ctrl+Enter saves", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    textarea().value = "quick";
    key_(textarea(), "Enter", { ctrlKey: true });
    expect(stored()).toMatchObject([{ note: "quick" }]);
  });

  it("Alt+Shift+A opens the popover for the selection", async () => {
    await select("b", 4, 7);
    key_(document.body, "Å", { code: "KeyA", altKey: true, shiftKey: true });
    expect(popover().hidden).toBe(false);
    expect(document.activeElement).toBe(textarea());
  });

  it("Tab stays inside the popover", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    action(popover(), "save").focus();
    key_(action(popover(), "save"), "Tab");
    expect(document.activeElement).toBe(textarea());
  });
});

describe("existing notes", () => {
  beforeEach(() => {
    page();
    ui = init(document, window);
  });

  it("two notes on one line share an icon with count 2 and prev/next cycles them", async () => {
    await addNote("a", 11, 16, "second");
    await addNote("a", 0, 5, "first");
    await addNote("b", 4, 7, "other line");

    const icons = $$("[data-gmd-annotation-icon]");
    expect(icons.map((i) => i.dataset.count)).toEqual(["2", "1"]);
    expect(icons[0].getAttribute("aria-label")).toBe("2 notes");

    icons[0].click();
    const position = popover().querySelector("[data-gmd-annotation-position]");
    expect(textarea().value).toBe("first");
    expect(position.textContent).toBe("1 of 2");
    expect(action(popover(), "prev").disabled).toBe(true);
    action(popover(), "next").click();
    expect(textarea().value).toBe("second");
    expect(position.textContent).toBe("2 of 2");
    expect(action(popover(), "next").disabled).toBe(true);
    action(popover(), "prev").click();
    expect(textarea().value).toBe("first");
  });

  it("editing an existing note to empty deletes it", async () => {
    await addNote("a", 6, 10, "note");
    $("[data-gmd-annotation-icon]").click();
    textarea().value = " ";
    action(popover(), "save").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(0);
    expect(highlights.has("gmd-annotation")).toBe(false);
  });

  it("Save updates the text", async () => {
    await addNote("a", 6, 10, "note");
    $("[data-gmd-annotation-icon]").click();
    textarea().value = "changed";
    action(popover(), "save").click();
    expect(stored()).toMatchObject([{ note: "changed" }]);
  });

  it("Delete removes it", async () => {
    await addNote("a", 6, 10, "note");
    $("[data-gmd-annotation-icon]").click();
    expect(action(popover(), "discard").hidden).toBe(true);
    action(popover(), "delete").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(0);
  });

  it("Escape keeps the stored text and returns focus to the icon", async () => {
    await addNote("a", 6, 10, "note");
    const icon = $("[data-gmd-annotation-icon]");
    icon.click();
    textarea().value = "unsaved";
    key_(textarea(), "Escape");
    expect(popover().hidden).toBe(true);
    expect(stored()).toMatchObject([{ note: "note" }]);
    expect(document.activeElement).toBe(icon);
  });
});

describe("focus", () => {
  beforeEach(() => {
    page();
    ui = init(document, window);
  });

  it("Escape after a layout returns focus to the note's rebuilt icon", async () => {
    await addNote("a", 6, 10, "note");
    const before = $("[data-gmd-annotation-icon]");
    before.click();
    window.dispatchEvent(new Event("resize"));
    await new Promise((r) => requestAnimationFrame(r));
    const after = $("[data-gmd-annotation-icon]");
    expect(after).not.toBe(before);
    expect(after.getAttribute("aria-expanded")).toBe("true");
    key_(textarea(), "Escape");
    expect(document.activeElement).toBe(after);
    expect(after.getAttribute("aria-expanded")).toBe("false");
  });

  it("deleting the last note moves focus to the header toggle", async () => {
    await addNote("a", 6, 10, "note");
    $("[data-gmd-annotation-icon]").click();
    action(popover(), "delete").click();
    expect(document.activeElement).toBe($("[data-gmd-annotations-toggle]"));
  });

  it("a click outside closes the popover", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    expect(popover().getAttribute("aria-modal")).toBe("true");
    document.body.dispatchEvent(
      new PointerEvent("pointerdown", { bubbles: true }),
    );
    expect(popover().hidden).toBe(true);
    expect(localStorage.getItem(key())).toBeNull();
  });

  it("Shift+Tab wraps inside the popover", async () => {
    await select("a", 6, 10);
    $("[data-gmd-annotation-pen]").click();
    expect(document.activeElement).toBe(textarea());
    key_(textarea(), "Tab", { shiftKey: true });
    expect(document.activeElement).toBe(action(popover(), "save"));
  });

  it("Tab stays inside the dialog and Escape returns focus to the toggle", async () => {
    await addNote("a", 6, 10, "note");
    const toggle = $("[data-gmd-annotations-toggle]");
    toggle.click();
    const dialog = $("dialog[data-gmd-annotations-dialog]");
    action(dialog, "close").focus();
    key_(action(dialog, "close"), "Tab");
    expect(document.activeElement).toBe(dialog.querySelector("textarea"));
    key_(dialog.querySelector("textarea"), "Escape");
    await tick();
    expect(dialog.open).toBe(false);
    expect(document.activeElement).toBe(toggle);
  });
});

describe("reload", () => {
  it.each([
    [1, "1 note no longer matches this page and was removed."],
    [2, "2 notes no longer match this page and were removed."],
  ])("drops %i stale notes and announces the count", (count, message) => {
    const article = page();
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

    ui = init(document, window);

    expect(stored()).toMatchObject([{ id: "1", note: "keep" }]);
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(1);
    const notice = $("[data-gmd-annotation-notice]");
    expect(notice.getAttribute("role")).toBe("status");
    expect(notice.textContent).toContain(message);
  });

  it("says nothing when every note matches", () => {
    page();
    ui = init(document, window);
    expect($("[data-gmd-annotation-notice]").textContent).toBe("");
  });
});

describe("header", () => {
  beforeEach(() => {
    page();
    ui = init(document, window);
  });

  it("header dialog shows the export; clear all needs confirm-clear", async () => {
    await addNote("b", 4, 7, "Why two?");
    $("[data-gmd-annotations-toggle]").click();
    const dialog = $("dialog[data-gmd-annotations-dialog]");
    expect(dialog.open).toBe(true);
    const area = dialog.querySelector("textarea");
    expect(document.activeElement).toBe(area);
    expect(area.value).toContain("# Review notes: Spec");
    expect(area.value).toContain("Source file: `docs/spec.md`");
    expect(area.value).toContain("## 1. Spec › Part");
    expect(area.value).toContain("**two**");
    expect(area.value).toContain("Why two?");

    action(dialog, "clear").click();
    expect(stored()).toHaveLength(1);
    action(dialog, "cancel-clear").click();
    expect(action(dialog, "confirm-clear").closest("[hidden]")).not.toBeNull();
    expect(stored()).toHaveLength(1);

    action(dialog, "clear").click();
    action(dialog, "confirm-clear").click();
    expect(localStorage.getItem(key())).toBeNull();
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(0);
    expect(action(dialog, "copy").disabled).toBe(true);
    expect(dialog.textContent).toContain("No notes on this page yet.");

    action(dialog, "close").click();
    expect(dialog.open).toBe(false);
    expect(document.activeElement).toBe($("[data-gmd-annotations-toggle]"));
  });

  it("copy writes the textarea's current content", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    vi.spyOn(navigator, "clipboard", "get").mockReturnValue({ writeText });
    await addNote("a", 6, 10, "n");
    $("[data-gmd-annotations-toggle]").click();
    const dialog = $("dialog[data-gmd-annotations-dialog]");
    dialog.querySelector("textarea").value = "edited export";
    action(dialog, "copy").click();
    await tick();
    expect(writeText).toHaveBeenCalledWith("edited export");
    expect(action(dialog, "copy").textContent).toBe("Copied");
  });

  it("badge hidden at zero, shows the count otherwise", async () => {
    const badge = $("[data-gmd-annotations-count]");
    const toggle = $("[data-gmd-annotations-toggle]");
    expect(badge.hidden).toBe(true);
    await addNote("a", 6, 10, "one");
    expect(toggle.getAttribute("aria-label")).toBe("Annotations (1 note)");
    await addNote("b", 4, 7, "two");
    expect(badge.hidden).toBe(false);
    expect(badge.textContent).toBe("2");
    expect(toggle.getAttribute("aria-label")).toBe("Annotations (2 notes)");
    expect(document.documentElement.hasAttribute("data-gmd-annotations")).toBe(
      true,
    );
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
    ui = init(document, window);
    await addNote("a", 6, 10, "memory");
    expect($$("[data-gmd-annotation-icon]")).toHaveLength(1);
    expect($("[data-gmd-annotation-notice]").textContent).toContain(
      "can't be saved in this browser",
    );
  });
});
