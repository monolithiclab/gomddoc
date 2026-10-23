import { beforeEach, describe, expect, it } from "vitest";
import {
  anchor,
  buildTextModel,
  createStore,
  escapeMarkdown,
  formatExport,
  groupByLine,
  headingPath,
  rangeFromOffsets,
  selectorFromRange,
  storageKey,
} from "../../cmd/gomddoc/assets/shared/gmd-annotations.mjs";

function content(html) {
  document.body.innerHTML = `<main id="c">${html}</main>`;
  return document.getElementById("c");
}

// rangeOver selects the text of node from character a to b.
function rangeOver(node, a, b, endNode = node) {
  const r = document.createRange();
  r.setStart(node, a);
  r.setEnd(endNode, b);
  return r;
}

beforeEach(() => {
  document.body.innerHTML = "";
});

describe("buildTextModel", () => {
  it("skips injected UI", () => {
    const root = content(
      '<p>Intro</p><pre><code>x</code><button class="copy-btn">Copy</button></pre>' +
        '<h2>T<gmd-heading-anchor href="#t">#</gmd-heading-anchor></h2>',
    );
    const model = buildTextModel(root);
    expect(model.text).toBe("IntroxT");
    expect(model.text).not.toContain("Copy");
    expect(model.nodes.map((n) => n.start)).toEqual([0, 5, 6]);
  });
});

describe("selectorFromRange", () => {
  it("spans inline elements", () => {
    const root = content("<p>foo bar <strong>baz</strong> qux end</p>");
    const p = root.querySelector("p");
    const model = buildTextModel(root);
    const range = rangeOver(p.firstChild, 4, 4, p.lastChild);
    const sel = selectorFromRange(model, range);
    expect(sel).toEqual({ quote: "bar baz qux", prefix: "foo ", suffix: " end", offset: 4 });
  });

  it("caps the context at 32 characters", () => {
    const long = "a".repeat(40);
    const root = content(`<p>${long}XY${long}</p>`);
    const t = root.querySelector("p").firstChild;
    const sel = selectorFromRange(buildTextModel(root), rangeOver(t, 40, 42));
    expect(sel.prefix).toBe("a".repeat(32));
    expect(sel.suffix).toBe("a".repeat(32));
    expect(sel.offset).toBe(40);
  });

  it("rejects ranges outside or inside skipped UI", () => {
    document.body.innerHTML =
      '<p id="out">outside</p><main id="c"><pre><code>x</code><button class="copy-btn">Copy</button></pre><p>in</p></main>';
    const root = document.getElementById("c");
    const model = buildTextModel(root);
    const out = document.getElementById("out").firstChild;
    const inside = root.querySelector("p").firstChild;
    const btn = root.querySelector(".copy-btn").firstChild;
    expect(selectorFromRange(model, rangeOver(out, 0, 3))).toBeNull();
    expect(selectorFromRange(model, rangeOver(out, 0, 2, inside))).toBeNull();
    expect(selectorFromRange(model, rangeOver(btn, 0, 4))).toBeNull();
    expect(selectorFromRange(model, rangeOver(btn, 1, 2, inside))).toBeNull();
  });

  it("rejects collapsed and whitespace-only ranges", () => {
    const root = content("<p>a   b</p>");
    const t = root.querySelector("p").firstChild;
    const model = buildTextModel(root);
    expect(selectorFromRange(model, rangeOver(t, 1, 1))).toBeNull();
    expect(selectorFromRange(model, rangeOver(t, 1, 4))).toBeNull();
  });
});

describe("anchor", () => {
  const model = (text) => ({ text, nodes: [] });

  it("finds the unique match", () => {
    const m = model("one two three four");
    expect(anchor(m, { prefix: "one ", quote: "two", suffix: " three" })).toEqual({ start: 4, end: 7 });
  });

  it("returns null when the passage is gone", () => {
    const m = model("one 2 three four");
    expect(anchor(m, { prefix: "one ", quote: "two", suffix: " three" })).toBeNull();
  });

  it("returns null on two matches", () => {
    const m = model("a two b. a two b.");
    expect(anchor(m, { prefix: "a ", quote: "two", suffix: " b" })).toBeNull();
  });

  it("distinguishes by context", () => {
    const m = model("first two here, second two here");
    expect(anchor(m, { prefix: "second ", quote: "two", suffix: " here" })).toEqual({ start: 23, end: 26 });
  });

  it("anchors at content start and end", () => {
    const m = model("start middle end");
    expect(anchor(m, { prefix: "", quote: "start", suffix: " middle" })).toEqual({ start: 0, end: 5 });
    expect(anchor(m, { prefix: "middle ", quote: "end", suffix: "" })).toEqual({ start: 13, end: 16 });
  });
});

describe("rangeFromOffsets", () => {
  it("maps model offsets back onto text nodes", () => {
    const root = content("<p>foo bar <strong>baz</strong> qux</p>");
    const model = buildTextModel(root);
    const range = rangeFromOffsets(model, 4, 11);
    expect(range.toString()).toBe("bar baz");
    expect(range.startContainer).toBe(root.querySelector("p").firstChild);
    expect(range.endContainer).toBe(root.querySelector("strong").firstChild);
  });
});

describe("headingPath", () => {
  it("returns the nearest heading and its ancestors", () => {
    const root = content(
      "<p>before</p><h1>Intent</h1><h2>Other</h2><p>x</p><h2>Goals</h2><h3>Deep</h3><p>y</p>" +
        "<h2>Goals<gmd-heading-anchor></gmd-heading-anchor></h2><p>target text</p>",
    );
    const model = buildTextModel(root);
    expect(headingPath(root, model, model.text.indexOf("target"))).toEqual(["Intent", "Goals"]);
    expect(headingPath(root, model, model.text.indexOf("y"))).toEqual(["Intent", "Goals", "Deep"]);
  });

  it("is empty before any heading", () => {
    const root = content("<p>before</p><h1>Intent</h1>");
    const model = buildTextModel(root);
    expect(headingPath(root, model, 2)).toEqual([]);
  });
});

describe("groupByLine", () => {
  it("groups items within the tolerance of the group's first item", () => {
    const items = [
      { id: "c", top: 140 },
      { id: "b", top: 104 },
      { id: "a", top: 100 },
    ];
    expect(groupByLine(items, 8)).toEqual([
      { top: 100, ids: ["a", "b"] },
      { top: 140, ids: ["c"] },
    ]);
  });
});

const labels = {
  exportTitle: "Review notes",
  exportSource: "Source file",
  exportPage: "Page",
  exportIntro: "Each note quotes the passage it refers to, with the words just before and after it to disambiguate.",
  exportTop: "(top of page)",
};

describe("formatExport", () => {
  it("matches the spec example", () => {
    const page = {
      title: "Client-Side Page Annotations",
      sourcePath: "specs/20261023.annotations-design.md",
      url: "http://localhost:8080/specs/20261023.annotations-design",
    };
    const notes = [
      {
        headingPath: ["Intent", "Goals"],
        prefix: "dropped, and the reader is told ",
        quote: "how many",
        suffix: ". On by default",
        note: 'How many is "how many"? Give the exact wording of the notice.',
      },
      {
        headingPath: ["User experience", "The header button"],
        prefix: "gains an\n  ",
        quote: "Annotations",
        suffix: " button with a count",
        note: 'Call it "Review notes" instead.',
      },
    ];
    const expected = `# Review notes: Client-Side Page Annotations

Source file: \`specs/20261023.annotations-design.md\`
Page: http://localhost:8080/specs/20261023.annotations-design

Each note quotes the passage it refers to, with the words just before and after it to disambiguate.

## 1. Intent › Goals

> …dropped, and the reader is told **how many**. On by default…

How many is "how many"? Give the exact wording of the notice.

## 2. User experience › The header button

> …gains an **Annotations** button with a count…

Call it "Review notes" instead.
`;
    expect(formatExport(page, notes, labels)).toBe(expected);
  });

  it("escapes the quote", () => {
    const out = formatExport(
      { title: "T", sourcePath: "t.md", url: "u" },
      [{ headingPath: [], prefix: "", quote: "*a* _b_ `c`", suffix: "", note: "n" },],
      labels,
    );
    expect(out).toContain("## 1. (top of page)\n\n> …**\\*a\\* \\_b\\_ \\`c\\`**…\n");
  });
});

describe("escapeMarkdown", () => {
  it("escapes inline syntax and a leading block marker", () => {
    expect(escapeMarkdown("a\\b [c] <d>")).toBe("a\\\\b \\[c\\] \\<d\\>");
    expect(escapeMarkdown("  # title")).toBe(" \\# title");
    expect(escapeMarkdown("- item")).toBe("\\- item");
    expect(escapeMarkdown("+ item")).toBe("\\+ item");
    expect(escapeMarkdown("> q")).toBe("\\> q");
    expect(escapeMarkdown("1. one")).toBe("1\\. one");
    expect(escapeMarkdown("a - b\n\n1. c")).toBe("a - b 1. c");
  });
});

describe("storageKey", () => {
  it("drops the fragment", () => {
    const loc = new URL("http://localhost:8080/guide/intro?lang=fr#goals");
    expect(storageKey(loc)).toBe("gomddoc:annotations:http://localhost:8080/guide/intro?lang=fr");
  });
});

function memoryStorage() {
  const m = new Map();
  return {
    getItem: (k) => (m.has(k) ? m.get(k) : null),
    setItem: (k, v) => m.set(k, String(v)),
    removeItem: (k) => m.delete(k),
    has: (k) => m.has(k),
  };
}

const note = { id: "n1", quote: "q", prefix: "p", suffix: "s", offset: 3, note: "text" };

describe("createStore", () => {
  it("round-trips", () => {
    const storage = memoryStorage();
    createStore(storage, "k").save([note]);
    expect(JSON.parse(storage.getItem("k"))).toEqual({ v: 1, notes: [note] });
    const store = createStore(storage, "k");
    expect(store.persistent).toBe(true);
    expect(store.load()).toEqual([note]);
  });

  it("save([]) removes the key", () => {
    const storage = memoryStorage();
    const store = createStore(storage, "k");
    store.save([note]);
    store.save([]);
    expect(storage.has("k")).toBe(false);
  });

  it("ignores entries of another version or malformed", () => {
    const storage = memoryStorage();
    storage.setItem("k", JSON.stringify({ v: 2, notes: [note] }));
    expect(createStore(storage, "k").load()).toEqual([]);
    storage.setItem("k", "{not json");
    expect(createStore(storage, "k").load()).toEqual([]);
  });

  it("falls back to memory when storage throws", () => {
    const storage = memoryStorage();
    storage.setItem = () => {
      throw new Error("QuotaExceededError");
    };
    const store = createStore(storage, "k");
    store.save([note]);
    expect(store.persistent).toBe(false);
    expect(store.load()).toEqual([note]);
    store.clear();
    expect(store.load()).toEqual([]);
  });

  it("works in memory without storage", () => {
    const store = createStore(null, "k");
    expect(store.persistent).toBe(false);
    store.save([note]);
    expect(store.load()).toEqual([note]);
  });
});
