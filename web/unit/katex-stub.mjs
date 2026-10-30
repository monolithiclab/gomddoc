// Stands in for KaTeX (vitest.config.mjs aliases the CDN URL here): records
// each call and renders a marker, so a test sees what gmd-math.mjs asked for.
export const calls = [];

export default {
  render(tex, element, options) {
    calls.push({ tex, element, options });
    element.innerHTML = `<span class="katex">${options.displayMode ? "D" : "I"}</span>`;
  },
};
