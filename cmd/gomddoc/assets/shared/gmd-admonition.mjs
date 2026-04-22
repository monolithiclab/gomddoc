/**
 * <gmd-admonition> Web Component
 *
 * Renders styled admonition blocks from type/title attributes.
 * Usage: <gmd-admonition type="note" title="Note">Content</gmd-admonition>
 *
 * Features:
 *   - Light DOM (theme CSS applies to inner content)
 *   - Reads type and title attributes
 *   - Prepends title element and applies CSS classes
 */

class GmdAdmonition extends HTMLElement {
  connectedCallback() {
    const type = this.getAttribute("type") || "note";
    const title = this.getAttribute("title") || type;

    this.classList.add("admonition", `admonition-${type}`);

    const titleEl = document.createElement("p");
    titleEl.className = "admonition-title";
    titleEl.textContent = title;
    this.prepend(titleEl);
  }
}

customElements.define("gmd-admonition", GmdAdmonition);
