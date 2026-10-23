/**
 * Page annotations — reader notes anchored to passages of the page content.
 *
 * Inlined as <script type="module"> on pages whose content element carries
 * data-gmd-annotatable. Notes live in localStorage only; the export is
 * Markdown for a coding harness. See docs/specs/20261023.annotations-design.md.
 *
 * The text model is the concatenated text of the content element minus
 * SKIP_SELECTOR subtrees. Anchors are {prefix, quote, suffix} found exactly
 * once in that text; anything else is stale (no fuzzy matching, no
 * tie-breaking by position).
 */

// Everything excluded from the text model, in one place: UI other scripts
// inject (code-copy buttons, heading anchors, color-chip swatches, this
// module's own UI) and rendered output that is not prose (KaTeX, Mermaid).
export const SKIP_SELECTOR =
  ".copy-btn, gmd-heading-anchor, gmd-color-chip, .katex, .language-mermaid, .mermaid, [data-gmd-annotation-ui]";

const CONTEXT_CHARS = 32;
const STORAGE_PREFIX = "gomddoc:annotations:";
const STORE_VERSION = 1;

export function buildTextModel(root) {
  const nodes = [];
  let text = "";
  const walk = (el) => {
    for (let child = el.firstChild; child; child = child.nextSibling) {
      if (child.nodeType === Node.TEXT_NODE) {
        nodes.push({ node: child, start: text.length });
        text += child.data;
      } else if (child.nodeType === Node.ELEMENT_NODE && !child.matches(SKIP_SELECTOR)) {
        walk(child);
      }
    }
  };
  walk(root);
  return { root, text, nodes };
}

// modelOffset maps a DOM boundary point to an offset in the model's text, or
// -1 when the point is outside the root or inside a skipped subtree.
function modelOffset(model, container, offset) {
  if (container.nodeType === Node.TEXT_NODE) {
    const entry = model.nodes.find((n) => n.node === container);
    return entry ? entry.start + offset : -1;
  }
  if (!model.root.contains(container)) return -1;
  if (container !== model.root && container.closest(SKIP_SELECTOR)) return -1;
  const point = container.ownerDocument.createRange();
  point.setStart(container, offset);
  const next = model.nodes.find((n) => point.comparePoint(n.node, 0) >= 0);
  return next ? next.start : model.text.length;
}

export function selectorFromRange(model, range) {
  if (range.collapsed) return null;
  const start = modelOffset(model, range.startContainer, range.startOffset);
  const end = modelOffset(model, range.endContainer, range.endOffset);
  if (start < 0 || end <= start) return null;
  const quote = model.text.slice(start, end);
  if (!quote.trim()) return null;
  return {
    quote,
    prefix: model.text.slice(Math.max(0, start - CONTEXT_CHARS), start),
    suffix: model.text.slice(end, end + CONTEXT_CHARS),
    offset: start,
  };
}

export function anchor(model, sel) {
  const needle = sel.prefix + sel.quote + sel.suffix;
  const at = model.text.indexOf(needle);
  if (at < 0 || model.text.indexOf(needle, at + 1) >= 0) return null;
  const start = at + sel.prefix.length;
  return { start, end: start + sel.quote.length };
}

export function rangeFromOffsets(model, start, end) {
  // A start lands in the node holding its character; an end in the node
  // holding the character before it, so neither strays into a neighbour.
  const locate = (pos, isEnd) => {
    const entry =
      model.nodes.find((n) =>
        isEnd ? pos > n.start && pos <= n.start + n.node.data.length : pos < n.start + n.node.data.length,
      ) || model.nodes[model.nodes.length - 1];
    return [entry.node, Math.min(Math.max(pos - entry.start, 0), entry.node.data.length)];
  };
  const range = model.root.ownerDocument.createRange();
  range.setStart(...locate(start, false));
  range.setEnd(...locate(end, true));
  return range;
}

export function headingPath(root, model, offset) {
  const path = [];
  for (const h of root.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
    if (h.closest(SKIP_SELECTOR)) continue;
    const start = modelOffset(model, h, 0);
    if (start < 0 || start > offset) break;
    const level = Number(h.tagName[1]);
    const title = model.nodes
      .filter((n) => h.contains(n.node))
      .map((n) => n.node.data)
      .join("")
      .replace(/\s+/g, " ")
      .trim();
    while (path.length && path[path.length - 1].level >= level) path.pop();
    path.push({ level, title });
  }
  return path.map((p) => p.title);
}

export function groupByLine(items, tolerance) {
  const groups = [];
  for (const item of [...items].sort((a, b) => a.top - b.top)) {
    const last = groups[groups.length - 1];
    if (last && item.top - last.top <= tolerance) last.ids.push(item.id);
    else groups.push({ top: item.top, ids: [item.id] });
  }
  return groups;
}

export function escapeMarkdown(s) {
  return s
    .replace(/\s+/g, " ")
    .replace(/[\\*_`[\]<>]/g, "\\$&")
    .replace(/^(\s*)([#>+-]|\d+\.)/, (_, ws, mark) => ws + mark.replace(/^(\d*)(.)$/, "$1\\$2"));
}

export function formatExport(page, notes, labels) {
  const out = [
    `# ${labels.exportTitle}: ${page.title}`,
    "",
    `${labels.exportSource}: \`${page.sourcePath}\``,
    `${labels.exportPage}: ${page.url}`,
    "",
    labels.exportIntro,
  ];
  notes.forEach((n, i) => {
    const heading = n.headingPath.length ? n.headingPath.join(" › ") : labels.exportTop;
    const quoted = `…${escapeMarkdown(n.prefix)}**${escapeMarkdown(n.quote)}**${escapeMarkdown(n.suffix)}…`;
    out.push("", `## ${i + 1}. ${heading}`, "", `> ${quoted}`, "", n.note);
  });
  return out.join("\n") + "\n";
}

export function storageKey(loc) {
  return STORAGE_PREFIX + loc.origin + loc.pathname + loc.search;
}

// createStore persists notes under key. The first storage call that throws
// (private mode, quota, disabled) turns persistence off for the session; the
// notes then live in memory only.
export function createStore(storage, key) {
  let persistent = Boolean(storage);
  let memory = [];
  const attempt = (fn) => {
    if (!persistent) return;
    try {
      fn();
    } catch {
      persistent = false;
    }
  };
  const save = (notes) => {
    memory = notes.slice();
    attempt(() => {
      if (notes.length) storage.setItem(key, JSON.stringify({ v: STORE_VERSION, notes }));
      else storage.removeItem(key);
    });
  };
  return {
    get persistent() {
      return persistent;
    },
    load() {
      attempt(() => {
        memory = parseEntry(storage.getItem(key));
      });
      return memory.slice();
    },
    save,
    clear: () => save([]),
  };
}

function parseEntry(raw) {
  if (!raw) return [];
  try {
    const entry = JSON.parse(raw);
    return entry && entry.v === STORE_VERSION && Array.isArray(entry.notes) ? entry.notes : [];
  } catch {
    return [];
  }
}

// init wires the interface on a page whose content carries
// data-gmd-annotatable, and does nothing anywhere else (tests import this
// module with no marker). The interface itself arrives with the UI.
export function init(doc, win) {
  if (!doc.querySelector("[data-gmd-annotatable]")) return;
}

if (typeof document !== "undefined") init(document, window);
