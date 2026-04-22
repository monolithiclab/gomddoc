/**
 * <gmd-heading-anchor> Web Component
 *
 * Renders a heading anchor link (#) revealed on hover.
 * Usage: <gmd-heading-anchor href="#section-id"></gmd-heading-anchor>
 *
 * Features:
 *   - Shadow DOM encapsulation
 *   - Inherits theme colors via CSS custom properties
 *   - Exposed part: ::part(link)
 *   - Accessible: aria-hidden, tabindex=-1
 */

const template = document.createElement("template");
template.innerHTML = `
  <style>
    :host {
      display: inline;
    }
    a {
      color: var(--color-text-muted, #64748b);
      text-decoration: none;
      opacity: 0;
      margin-left: 0.5rem;
      font-weight: 400;
      transition: opacity 0.2s, color 0.2s;
    }
    :host-context(h1:hover) a,
    :host-context(h2:hover) a,
    :host-context(h3:hover) a,
    :host-context(h4:hover) a,
    :host-context(h5:hover) a,
    :host-context(h6:hover) a {
      opacity: 1;
    }
    a:hover {
      color: var(--color-primary, #2563eb);
    }
    @media (hover: none) {
      a { opacity: 1; }
    }
  </style>
  <a part="link" aria-hidden="true" tabindex="-1">#</a>
`;

class GmdHeadingAnchor extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: "open" });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
  }

  connectedCallback() {
    const href = this.getAttribute("href");
    if (href) {
      this.shadowRoot.querySelector("a").href = href;
    }
  }
}

customElements.define("gmd-heading-anchor", GmdHeadingAnchor);
