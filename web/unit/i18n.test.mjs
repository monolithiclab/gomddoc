import { afterEach, describe, expect, it, vi } from "vitest";
import { i18n } from "/_assets/gmd-i18n.mjs";

// docWith builds a fresh document holding one data block per namespace.
function docWith(blocks, lang = "en-US") {
  const doc = document.implementation.createHTMLDocument("t");
  doc.documentElement.lang = lang;
  for (const [ns, data] of Object.entries(blocks)) {
    const s = doc.createElement("script");
    s.type = "application/json";
    s.dataset.gmdI18n = ns;
    s.textContent = typeof data === "string" ? data : JSON.stringify(data);
    doc.head.append(s);
  }
  return doc;
}

afterEach(() => vi.restoreAllMocks());

describe("t", () => {
  it("returns the translation, or the key when absent", () => {
    const { t } = i18n("a", docWith({ a: { hello: "Bonjour" } }));
    expect(t("hello")).toBe("Bonjour");
    expect(t("nope")).toBe("nope");
  });

  it("fills placeholders", () => {
    const { t } = i18n(
      "a",
      docWith({
        a: {
          seq: "%s has %d",
          idx: "%[2]d sur %[1]d",
          pct: "100%%",
          miss: "[%s][%s]",
        },
      }),
    );
    expect(t("seq", "a", 2)).toBe("a has 2");
    expect(t("idx", 5, 2)).toBe("2 sur 5");
    expect(t("pct")).toBe("100%");
    expect(t("miss", "x")).toBe("[x][]");
  });
});

describe("tn", () => {
  const block = {
    n_one: "%d note",
    n: "%d notes",
    f_few: "few %d",
    f: "other %d",
    g: "other %d",
  };

  it("en-US picks one vs other", () => {
    const { tn } = i18n("a", docWith({ a: block }));
    expect(tn("n", 1)).toBe("1 note");
    expect(tn("n", 0)).toBe("0 notes");
    expect(tn("n", 2)).toBe("2 notes");
  });

  it("fr-FR treats 0 and 1 as one", () => {
    const { tn } = i18n("a", docWith({ a: block }, "fr-FR"));
    expect(tn("n", 0)).toBe("0 note");
    expect(tn("n", 1)).toBe("1 note");
    expect(tn("n", 2)).toBe("2 notes");
  });

  it("ar uses few and many, falling back to the bare key", () => {
    const { tn } = i18n(
      "a",
      docWith({ a: { ...block, f_many: "many %d" } }, "ar"),
    );
    expect(tn("f", 3)).toBe("few 3");
    expect(tn("f", 11)).toBe("many 11");
    expect(tn("g", 3)).toBe("other 3");
  });

  it("survives an invalid or empty lang", () => {
    for (const lang of ["not a tag", ""]) {
      const { tn } = i18n("a", docWith({ a: block }, lang));
      expect(typeof tn("n", 1)).toBe("string");
    }
  });
});

describe("blocks", () => {
  it("returns keys for a missing block or invalid JSON", () => {
    const { t, tn } = i18n("a", docWith({}));
    expect(t("k")).toBe("k");
    expect(tn("k", 2)).toBe("k");
    const bad = i18n("a", docWith({ a: "{not json" }));
    expect(bad.t("k")).toBe("k");
  });

  it("first of two same-namespace blocks wins, warning once", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const doc = docWith({ a: { k: "first" } });
    const second = doc.createElement("script");
    second.type = "application/json";
    second.dataset.gmdI18n = "a";
    second.textContent = '{"k":"second"}';
    doc.head.append(second);
    const { t } = i18n("a", doc);
    expect(t("k")).toBe("first");
    expect(warn).toHaveBeenCalledTimes(1);
  });

  it("keeps namespaces apart", () => {
    const doc = docWith({ a: { x: "A" }, b: { x: "B", y: "Y" } });
    expect(i18n("a", doc).t("y")).toBe("y");
    expect(i18n("b", doc).t("x")).toBe("B");
  });

  it("parses once per document and namespace", () => {
    const doc = docWith({ a: { k: "old" } });
    i18n("a", doc);
    doc.querySelector("[data-gmd-i18n]").textContent = '{"k":"new"}';
    expect(i18n("a", doc).t("k")).toBe("old");
  });
});
