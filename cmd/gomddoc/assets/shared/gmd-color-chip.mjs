/**
 * <gmd-color-chip> Web Component
 *
 * Renders inline color swatches from hex codes.
 * Usage: <gmd-color-chip>#FF5733</gmd-color-chip>
 *
 * Features:
 *   - Shadow DOM encapsulation
 *   - Inherits theme colors via CSS custom properties
 *   - Click to copy hex value
 *   - Accessible (role="img", aria-label)
 *   - Exposed parts: ::part(swatch), ::part(label)
 */

const template = document.createElement("template");
template.innerHTML = `
  <style>
    :host {
      display: inline-block;
      vertical-align: middle;
      font-size: 0.875em;
      cursor: pointer;
    }
    .chip {
      display: inline-flex;
      align-items: center;
      gap: 0.4em;
      padding: 0.15em 0.5em;
      background: var(--color-bg-secondary, rgba(255, 255, 255, 0.6));
      border: 1px solid var(--color-border, rgba(46, 52, 64, 0.1));
      border-radius: 6px;
      font-family: var(--font-family-mono, ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace);
      vertical-align: baseline;
      backdrop-filter: blur(8px);
      -webkit-backdrop-filter: blur(8px);
    }
    .swatch {
      display: inline-block;
      width: 1em;
      height: 1em;
      border-radius: 50%;
      border: 1px solid var(--color-border, rgba(46, 52, 64, 0.1));
      flex-shrink: 0;
      box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.1);
    }
    .label {
      font-weight: 600;
      color: var(--color-text, #1f2328);
    }
  </style>
  <span class="chip" part="chip">
    <span class="swatch" part="swatch"></span>
    <code class="label" part="label"><slot></slot></code>
  </span>
`;

class GmdColorChip extends HTMLElement {
  static get observedAttributes() {
    return ["value"];
  }

  constructor() {
    super();
    this.attachShadow({ mode: "open" });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
    this._swatch = this.shadowRoot.querySelector(".swatch");
  }

  connectedCallback() {
    this._update();
    if (!this.hasAttribute("role")) this.setAttribute("role", "img");
    this._updateLabel();
    this.addEventListener("click", this._copy);
  }

  disconnectedCallback() {
    this.removeEventListener("click", this._copy);
  }

  attributeChangedCallback() {
    this._update();
  }

  get value() {
    return this.getAttribute("value") || this.textContent.trim();
  }

  _update() {
    const color = this.value;
    if (color && this._swatch) {
      this._swatch.style.background = color;
    }
  }

  _updateLabel() {
    if (!this.getAttribute("aria-label")) {
      this.setAttribute("aria-label", `Color ${this.value}`);
    }
  }

  _copy = async () => {
    try {
      await navigator.clipboard.writeText(this.value);
      const label = this.shadowRoot.querySelector(".label");
      const original = label.textContent;
      label.textContent = "Copied!";
      setTimeout(() => {
        label.textContent = original;
      }, 1500);
    } catch {
      /* clipboard not available */
    }
  };
}

customElements.define("gmd-color-chip", GmdColorChip);
