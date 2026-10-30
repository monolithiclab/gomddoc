import { beforeEach, describe, expect, it, vi } from "vitest";
import { calls } from "./katex-stub.mjs";
import "../../cmd/gomddoc/assets/shared/gmd-math.mjs";

// The module runs after the page is parsed, so a <gmd-math> already holds its
// TeX when it upgrades. happy-dom's innerHTML connects an element before its
// children, so pages are inserted as parsed fragments instead.
function page(html) {
  document.body.append(document.createRange().createContextualFragment(html));
}

beforeEach(() => {
  calls.length = 0;
  document.body.innerHTML = "";
});

describe("gmd-math", () => {
  it("typesets inline and display math from the element's TeX", async () => {
    page(
      "<p><gmd-math>a &lt; b</gmd-math></p><gmd-math display>\\int_0^1 x</gmd-math>",
    );

    await vi.waitFor(() => expect(calls).toHaveLength(2));
    expect(calls.map(({ tex, options }) => [tex, options])).toEqual([
      ["a < b", { displayMode: false, throwOnError: false }],
      ["\\int_0^1 x", { displayMode: true, throwOnError: false }],
    ]);
    const [inline, display] = document.querySelectorAll("gmd-math");
    expect(inline.innerHTML).toBe('<span class="katex">I</span>');
    expect(display.innerHTML).toBe('<span class="katex">D</span>');
  });

  it("typesets once when the element moves", async () => {
    page("<p><gmd-math>x</gmd-math></p><div></div>");
    const math = document.querySelector("gmd-math");

    await vi.waitFor(() => expect(calls).toHaveLength(1));

    document.querySelector("div").append(math);

    await new Promise((resolve) => setTimeout(resolve));
    expect(calls).toHaveLength(1);
    expect(math.innerHTML).toBe('<span class="katex">I</span>');
  });
});
