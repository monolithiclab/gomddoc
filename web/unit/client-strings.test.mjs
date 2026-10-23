import { beforeEach, describe, expect, it, vi } from "vitest";

// The code-copy, search and color-chip modules read their strings from the
// page's data-gmd-i18n blocks. Each test writes the page, then imports the
// module afresh (resetModules also resets gmd-i18n.mjs's per-document cache).

function page(blocks, body = "") {
  document.head.innerHTML = "";
  document.body.innerHTML =
    Object.entries(blocks)
      .map(
        ([ns, dict]) =>
          `<script type="application/json" data-gmd-i18n="${ns}">${JSON.stringify(dict)}</script>`,
      )
      .join("") + body;
}

beforeEach(() => {
  vi.resetModules();
});

describe("code-copy.mjs", () => {
  it("labels each button from the code_copy block", async () => {
    page(
      {
        code_copy: {
          code_copy_label: "Copier",
          code_copy_done: "Copié !",
          code_copy_aria: "Copier le code",
        },
      },
      "<pre><code>x</code></pre>",
    );
    await import("../../cmd/gomddoc/assets/shared/code-copy.mjs");

    const btn = document.querySelector("pre > .copy-btn");
    expect(btn.textContent).toBe("Copier");
    expect(btn.getAttribute("aria-label")).toBe("Copier le code");
  });
});

describe("search.mjs", () => {
  it("labels the dialog from the search block and escapes every label", async () => {
    const hostile = `"><img src=x onerror=alert(1)>`;
    page({
      search: {
        search_label: "Rechercher dans la doc",
        search_query: "Requête",
        search_results: "Résultats",
        search_placeholder: hostile,
        search_navigate: "<b>Naviguer</b>",
        search_tag_tip: "Astuce & tag",
      },
    });
    await import("../../cmd/gomddoc/assets/shared/search.mjs");

    const modal = document.getElementById("search-modal");
    expect(modal.getAttribute("aria-label")).toBe("Rechercher dans la doc");
    expect(document.getElementById("search-input").getAttribute("aria-label")).toBe("Requête");
    expect(document.getElementById("search-input").getAttribute("placeholder")).toBe(hostile);
    expect(document.getElementById("search-results").getAttribute("aria-label")).toBe("Résultats");
    expect(modal.querySelector("img")).toBeNull();
    expect(modal.querySelector(".search-footer b")).toBeNull();
    expect(modal.querySelector(".search-footer").textContent).toContain("<b>Naviguer</b>");
    expect(modal.querySelector(".search-tip").textContent).toBe("Astuce & tag");
  });

  it("omits the tip when the translation is empty", async () => {
    page({ search: { search_tag_tip: "" } });
    await import("../../cmd/gomddoc/assets/shared/search.mjs");

    expect(document.querySelector("#search-modal .search-tip")).toBeNull();
  });
});

describe("gmd-color-chip.mjs", () => {
  it("names the chip from color_chip_label", async () => {
    page({ color_chip: { color_chip_label: "Couleur %s" } }, "<gmd-color-chip>#FF5733</gmd-color-chip>");
    await import("../../cmd/gomddoc/assets/shared/gmd-color-chip.mjs");

    expect(document.querySelector("gmd-color-chip").getAttribute("aria-label")).toBe("Couleur #FF5733");
  });
});
