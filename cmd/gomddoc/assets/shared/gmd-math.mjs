/**
 * <gmd-math> Web Component
 *
 * Typesets the TeX it contains with KaTeX. The Markdown renderer emits it for
 * $`…`$ (inline) and $$ blocks (<gmd-math display>), so only those become math
 * and a price like "$1,200" is never touched (D52).
 * Usage: <gmd-math>x^2</gmd-math>, <gmd-math display>\int_0^1 x\,dx</gmd-math>
 *
 * Features:
 *   - Light DOM: KaTeX's stylesheet (head-katex) styles the output
 *   - KaTeX is fetched only on pages with math; the TeX source stays readable
 *     until it loads, and if it never does
 *   - An invalid expression renders in place as KaTeX's error, not a thrown one
 *   - Accessible: KaTeX adds MathML next to the visual rendering
 */

// KaTeX is fetched when the first <gmd-math> connects, so a page without math
// loads no script. The URL stays a literal (the unit tests alias it) on the
// same version as the head-katex stylesheet, which TestKaTeXVersionsMatch checks.
let katex;

class GmdMath extends HTMLElement {
  #typeset = false;

  async connectedCallback() {
    // A script moving the element reconnects it: its text is KaTeX output by then.
    if (this.#typeset) return;
    this.#typeset = true;
    const tex = this.textContent;
    katex ??= import(
      "https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.mjs"
    ).then((module) => module.default);
    (await katex).render(tex, this, {
      displayMode: this.hasAttribute("display"),
      throwOnError: false,
    });
  }
}

customElements.define("gmd-math", GmdMath);
