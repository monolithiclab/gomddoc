/**
 * Page annotations — reader notes anchored to passages of the page content.
 *
 * Inlined as <script type="module"> when the annotations feature is on; it
 * defines the custom elements below and does nothing until a gmd-annotations
 * element (the theme's wrapper around the content) connects. Notes live in
 * localStorage only; the export is Markdown for a coding harness. See
 * docs/specs/20261023.annotations-design.md.
 *
 * The text model is the concatenated text of the content element minus
 * SKIP_SELECTOR subtrees. Anchors are {prefix, quote, suffix} found exactly
 * once in that text; anything else is stale (no fuzzy matching, no
 * tie-breaking by position).
 */

// Everything excluded from the text model, in one place: UI other scripts
// inject (code-copy buttons, heading anchors, color-chip swatches, this
// module's own UI), rendered output that is not prose (KaTeX, Mermaid) and
// elements whose text never renders (raw HTML <style> or <script>).
export const SKIP_SELECTOR =
  ".copy-btn, gmd-heading-anchor, gmd-color-chip, .katex, .language-mermaid, .mermaid, gmd-annotation-margin, gmd-annotation-editor, gmd-annotation-notice, style, script, template";

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
      } else if (
        child.nodeType === Node.ELEMENT_NODE &&
        !child.matches(SKIP_SELECTOR)
      ) {
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

// rangeFromOffsets needs a model with at least one text node; offsets from
// anchor() or selectorFromRange() always come from such a model.
export function rangeFromOffsets(model, start, end) {
  // A start lands in the node holding its character; an end in the node
  // holding the character before it, so neither strays into a neighbour.
  const locate = (pos, isEnd) => {
    const entry =
      model.nodes.find((n) =>
        isEnd
          ? pos > n.start && pos <= n.start + n.node.data.length
          : pos < n.start + n.node.data.length,
      ) || model.nodes[model.nodes.length - 1];
    return [
      entry.node,
      Math.min(Math.max(pos - entry.start, 0), entry.node.data.length),
    ];
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
    .replace(
      /^(\s*)([#>+-]|\d+\.)/,
      (_, ws, mark) => ws + mark.replace(/^(\d*)(.)$/, "$1\\$2"),
    );
}

export function formatExport(page, notes, labels) {
  const out = [
    `# ${labels.exportTitle}: ${escapeMarkdown(page.title)}`,
    "",
    `${labels.exportSource}: \`${page.sourcePath}\``,
    `${labels.exportPage}: ${page.url}`,
    "",
    labels.exportIntro,
  ];
  notes.forEach((n, i) => {
    const heading = n.headingPath.length
      ? n.headingPath.map(escapeMarkdown).join(" › ")
      : labels.exportTop;
    // CommonMark only bolds **x** when x neither starts nor ends with
    // whitespace, so the quote's edge whitespace (a triple-click selects the
    // trailing newline) moves into the context outside the markers.
    const [, lead, body, trail] = n.quote.match(/^(\s*)([\s\S]*?)(\s*)$/);
    const quoted = `…${escapeMarkdown(n.prefix + lead)}**${escapeMarkdown(body)}**${escapeMarkdown(trail + n.suffix)}…`;
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
      const entry = JSON.stringify({ v: STORE_VERSION, notes });
      if (notes.length) storage.setItem(key, entry);
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
    return entry && entry.v === STORE_VERSION && Array.isArray(entry.notes)
      ? entry.notes
      : [];
  } catch {
    return [];
  }
}

// ---- Interface -------------------------------------------------------------
//
// gmd-annotations (light DOM, wraps the content) owns the controller and
// mounts the rest in the body: gmd-annotation-margin (shadow DOM: the pen and
// the note icons), gmd-annotation-editor (an auto popover), the notes dialog
// and gmd-annotation-notice (a manual popover). gmd-annotations-toggle sits
// in the theme's header and follows the gmd-annotations-change events. The
// platform does the rest: top layer, light dismiss, Esc, focus return, the
// dialog's focus trap, Invoker Commands for the toggle and the Close buttons.

const HIGHLIGHT_NAME = "gmd-annotation";
const CHANGE_EVENT = "gmd-annotations-change";
const DIALOG_ID = "gmd-annotations-dialog";
const NOTICE_ID = "gmd-annotation-notice";
const LINE_TOLERANCE = 8; // px: first lines closer than this share an icon
const MARGIN_GAP = 8; // px between the content's right edge and the icons
const ICON_SIZE = 28; // px, matches MARGIN_STYLE
const COPIED_MS = 2000;

const SVG_ATTRS =
  'aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"';
const PEN_SVG = `<svg ${SVG_ATTRS}><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>`;
const NOTE_SVG = `<svg ${SVG_ATTRS}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`;

// parse builds one element from markup.
function parse(doc, html) {
  const template = doc.createElement("template");
  template.innerHTML = html.trim();
  return template.content.firstElementChild;
}

const escapeHTML = (s) => s.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);

const button = (action, text, label, extra = "") =>
  `<button type="button" data-action="${action}"${label ? ` aria-label="${label}"` : ""}${extra}>${text}</button>`;

function readConfig(doc) {
  try {
    return JSON.parse(doc.getElementById("gmd-annotations-config").textContent);
  } catch {
    return {};
  }
}

// labeler returns the label lookups over a configuration: L fills each %d in
// order, P picks the name_one variant for a count of 1 when the locale has
// one, E escapes for markup.
function labeler(config) {
  const L = (name, ...nums) => {
    let i = 0;
    const label = config.labels?.[name] ?? name;
    return label.replace(/%d/g, () => String(nums[i++] ?? ""));
  };
  const P = (name, n) =>
    L(n === 1 && config.labels?.[`${name}_one`] ? `${name}_one` : name, n);
  const E = (name, ...nums) => escapeHTML(L(name, ...nums));
  return { L, P, E };
}

// localStorageOf returns the window's storage, or null where merely reading
// the property throws (storage disabled for the site).
function localStorageOf(win) {
  try {
    return win.localStorage;
  } catch {
    return null;
  }
}

// rangeTop is the viewport top of a range's first line box.
function rangeTop(range) {
  const rects = range.getClientRects();
  return (rects.length ? rects[0] : range.getBoundingClientRect()).top;
}

function newID(win) {
  return win.crypto?.randomUUID
    ? win.crypto.randomUUID()
    : `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
}

// Elements are defined only where a DOM exists; the base keeps the module
// importable without one.
const Base = globalThis.HTMLElement ?? class {};

// ---- gmd-annotation-margin

const MARGIN_STYLE = `
  :host { position: absolute; top: 0; left: 0; width: 0; height: 0; }
  button {
    position: absolute;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.15rem;
    min-width: 1.75rem;
    height: 1.75rem;
    padding: 0 0.3rem;
    border: 1px solid var(--gmd-annotation-border);
    border-radius: 6px;
    background: var(--gmd-annotation-surface);
    color: var(--gmd-annotation-text);
    font: 600 0.75rem/1 system-ui, sans-serif;
    cursor: pointer;
    transition: background-color 0.15s, color 0.15s;
  }
  button[hidden] { display: none; }
  [part="pen"] {
    background: var(--gmd-annotation-accent);
    border-color: var(--gmd-annotation-accent);
    color: var(--gmd-annotation-accent-text);
  }
  [part="icon"]:hover, [part="icon"][aria-expanded="true"] {
    background: var(--gmd-annotation-highlight);
    color: var(--gmd-annotation-highlight-text);
  }
  svg { width: 1rem; height: 1rem; flex: none; }
  @media (prefers-reduced-motion: reduce) { button { transition: none; } }
`;

// GmdAnnotationMargin renders the pen and the note icons at viewport
// positions the controller computes. A click on either dispatches
// gmd-annotation-open with the button as source and the note ids it opens
// (null for the pen). The pen comes last so it paints over an icon on the
// same line.
class GmdAnnotationMargin extends Base {
  #ids = new WeakMap(); // icon -> the note ids it opens

  constructor() {
    super();
    this.attachShadow({ mode: "open" }).innerHTML =
      `<style>${MARGIN_STYLE}</style><div></div><button type="button" part="pen" hidden>${PEN_SVG}</button>`;
    // Keep the selection: the pen annotates it.
    this.pen.addEventListener("mousedown", (e) => e.preventDefault());
    this.shadowRoot.addEventListener("click", (e) => {
      const source = e.target.closest("button");
      if (!source) return;
      const ids = this.#ids.get(source) ?? null;
      this.dispatchEvent(
        new CustomEvent("gmd-annotation-open", { detail: { source, ids } }),
      );
    });
  }

  get pen() {
    return this.shadowRoot.lastElementChild;
  }

  get #box() {
    return this.shadowRoot.querySelector("div");
  }

  #place(node, x, top) {
    const origin = this.getBoundingClientRect();
    node.style.left = `${x - origin.left}px`;
    node.style.top = `${top - origin.top}px`;
  }

  showPen(x, top) {
    this.pen.hidden = false;
    this.#place(this.pen, x, top - 2);
  }

  hidePen() {
    this.pen.hidden = true;
  }

  iconFor(id) {
    return [...this.#box.children].find((icon) =>
      this.#ids.get(icon).includes(id),
    );
  }

  // renderIcons shows one icon per group ({ids, top, label}) at column x. An
  // icon whose group is unchanged is kept, so focus and an open editor's
  // anchor survive a layout.
  renderIcons(groups, x) {
    const box = this.#box;
    const old = new Map(
      [...box.children].map((icon) => [this.#ids.get(icon).join(), icon]),
    );
    const focusedID = this.#ids.get(this.shadowRoot.activeElement)?.[0];
    const icons = groups.map((group) => {
      let icon = old.get(group.ids.join());
      if (!icon) {
        const n = group.ids.length;
        icon = parse(
          this.ownerDocument,
          `<button type="button" part="icon" data-count="${n}" aria-haspopup="dialog" aria-expanded="false">${NOTE_SVG}${n > 1 ? `<span>${n}</span>` : ""}</button>`,
        );
        this.#ids.set(icon, group.ids);
      }
      icon.setAttribute("aria-label", group.label);
      this.#place(icon, x, group.top - 2);
      return icon;
    });
    const keep = new Set(icons);
    for (const icon of [...box.children]) if (!keep.has(icon)) icon.remove();
    icons.forEach((icon, i) => {
      if (box.children[i] !== icon) box.insertBefore(icon, box.children[i]);
    });
    if (focusedID && !this.shadowRoot.activeElement) {
      this.iconFor(focusedID)?.focus();
    }
  }
}

// ---- gmd-annotation-editor

// GmdAnnotationEditor is the note popover: one textarea, the
// [data-action=save|discard|delete|prev|next] buttons and the "n of m"
// position. Being non-modal it traps nothing; the controller opens it with
// showPopover({source}) so it anchors to, and returns focus to, its opener.
class GmdAnnotationEditor extends Base {
  connectedCallback() {
    if (this.firstChild) return;
    const { L, E } = labeler(readConfig(this.ownerDocument));
    this.setAttribute("role", "dialog");
    this.setAttribute("aria-label", L("annotation_note"));
    this.innerHTML = `
      <div class="gmd-annotation-row">${button("prev", "‹", E("annotation_prev"))}<span class="gmd-annotation-position" aria-live="polite"></span>${button("next", "›", E("annotation_next"))}</div>
      <textarea rows="5" aria-label="${E("annotation_note")}"></textarea>
      <div class="gmd-annotation-row">${button("delete", E("annotation_delete"))}<span class="gmd-annotation-spacer"></span>${button("discard", E("annotation_discard"))}${button("save", E("annotation_save"))}</div>`;
  }

  get textarea() {
    return this.querySelector("textarea");
  }

  action(name) {
    return this.querySelector(`[data-action="${name}"]`);
  }

  open(source) {
    this.showPopover({ source });
    this.place(source);
    this.textarea.focus();
  }

  // place puts the editor under source, right-aligned with it, where the
  // browser lacks CSS anchor positioning (gmd-annotations.css does it
  // otherwise). Absolute in the top layer means document coordinates, so the
  // editor scrolls with the page.
  place(source) {
    const win = this.ownerDocument.defaultView;
    if (win.CSS?.supports?.("anchor-name: --a")) return;
    const r = source.getBoundingClientRect();
    this.style.position = "absolute";
    this.style.left = `${Math.max(8, r.right + win.scrollX - this.offsetWidth)}px`;
    this.style.top = `${r.bottom + win.scrollY}px`;
  }
}

// ---- gmd-annotation-notice

// GmdAnnotationNotice shows one dismissible message in the top layer; its
// role="status" makes screen readers announce it.
class GmdAnnotationNotice extends Base {
  announce(message, closeLabel) {
    this.showPopover();
    this.innerHTML = `<span>${escapeHTML(message)}</span><button type="button" commandfor="${this.id}" command="hide-popover" aria-label="${escapeHTML(closeLabel)}">×</button>`;
  }
}

// ---- gmd-annotations-toggle

// GmdAnnotationsToggle renders the header button that opens the notes
// dialog through Invoker Commands, and keeps its count and label in step
// with the gmd-annotations-change events.
class GmdAnnotationsToggle extends Base {
  #abort = null;

  connectedCallback() {
    const doc = this.ownerDocument;
    const { L, P, E } = labeler(readConfig(doc));
    if (!this.firstElementChild) {
      this.innerHTML = `<button type="button" commandfor="${DIALOG_ID}" command="show-modal" aria-haspopup="dialog" title="${E("annotations_open")}">${NOTE_SVG.replace("<svg ", '<svg width="18" height="18" ')}<span class="gmd-annotations-count" aria-hidden="true" hidden></span></button>`;
    }
    const toggle = this.firstElementChild;
    const badge = toggle.querySelector(".gmd-annotations-count");
    const update = (count) => {
      badge.textContent = String(count);
      badge.hidden = count === 0;
      toggle.setAttribute(
        "aria-label",
        count ? P("annotations_open_count", count) : L("annotations_open"),
      );
    };
    this.#abort = new AbortController();
    doc.addEventListener(CHANGE_EVENT, (e) => update(e.detail.count), {
      signal: this.#abort.signal,
    });
    update(doc.querySelector("gmd-annotations")?.count ?? 0);
  }

  disconnectedCallback() {
    this.#abort?.abort();
  }
}

// ---- gmd-annotations

// mount builds the interface over content once the page has loaded, so
// KaTeX, Mermaid and the other shared scripts are done changing the DOM
// before stored notes are re-anchored.
function mount(doc, win, content, notice, signal) {
  const on = (target, type, fn, opts = {}) =>
    target.addEventListener(type, fn, { ...opts, signal });
  const config = readConfig(doc);
  const { L, P, E } = labeler(config);
  const store = createStore(localStorageOf(win), storageKey(win.location));

  let notes = []; // stored fields plus the live range and its model start
  let pending = null; // the current annotatable selection
  let state = null; // the open editor
  let warnedNotKept = false;
  let exportEdited = false;
  let exportStale = true;
  let layoutQueued = false;

  // ---- Elements

  const margin = doc.createElement("gmd-annotation-margin");
  const editor = doc.createElement("gmd-annotation-editor");
  editor.setAttribute("popover", "auto");
  const dialog = parse(
    doc,
    `<dialog id="${DIALOG_ID}" aria-labelledby="gmd-annotations-title">
      <h2 id="gmd-annotations-title">${E("annotations_open")}</h2>
      <p>${E("annotations_empty")}</p>
      <textarea aria-label="${E("annotations_open")}" spellcheck="false"></textarea>
      <div class="gmd-annotation-row">${button("clear", E("annotations_clear"))}<span class="gmd-annotation-spacer"></span>${button("copy", E("annotations_copy"))}${button("close", E("annotations_close"), "", ` commandfor="${DIALOG_ID}" command="close"`)}</div>
      <div class="gmd-annotation-row" hidden><span class="gmd-annotation-spacer">${E("annotations_clear_confirm")}</span>${button("cancel-clear", E("annotations_cancel"))}${button("confirm-clear", E("annotations_clear_yes"))}</div>
    </dialog>`,
  );
  // In body, not next to content: a theme may clip its content column
  // (overflow: hidden), which would hide the margin icons.
  doc.body.append(margin, editor, dialog);

  const { pen } = margin;
  pen.setAttribute("aria-label", L("annotate"));
  pen.title = `${L("annotate")} (Alt+Shift+A)`;
  const { textarea } = editor;
  const [prev, next, del, discard] = ["prev", "next", "delete", "discard"].map(
    (name) => editor.action(name),
  );
  const nav = prev.parentElement;
  const position = nav.querySelector(".gmd-annotation-position");
  const act = (name) => dialog.querySelector(`[data-action="${name}"]`);
  const [, empty, exportArea, , confirmRow] = dialog.children;
  const [copy, clear, close] = ["copy", "clear", "close"].map(act);

  // ---- Notes

  const byID = (id) => notes.find((n) => n.id === id);

  const persist = () => {
    // The live range and model start stay in memory only.
    store.save(notes.map(({ range, start, ...stored }) => stored));
    if (!store.persistent && !warnedNotKept) {
      warnedNotKept = true;
      notice.announce(L("annotations_not_kept"), L("annotations_close"));
    }
  };

  // ---- Layout

  // column is the viewport x of the margin column: right of the content, or
  // over its right edge when the viewport leaves no margin.
  function column() {
    const right = content.getBoundingClientRect().right;
    const x = right + MARGIN_GAP;
    return x + ICON_SIZE > doc.documentElement.clientWidth
      ? right - ICON_SIZE
      : x;
  }

  function renderIcons() {
    const groups = groupByLine(
      notes.map((n) => ({ id: n.id, top: rangeTop(n.range) })),
      LINE_TOLERANCE,
    );
    for (const group of groups) {
      group.ids.sort((a, b) => byID(a).start - byID(b).start);
      const n = group.ids.length;
      group.label = n > 1 ? L("annotations_count", n) : L("annotation_note");
    }
    margin.renderIcons(groups, column());
  }

  function layout() {
    renderIcons();
    if (pending) margin.showPen(column(), pending.top);
    if (state) editor.place(state.source);
  }

  function scheduleLayout() {
    if (layoutQueued) return;
    layoutQueued = true;
    win.requestAnimationFrame(() => {
      layoutQueued = false;
      layout();
    });
  }

  // render reflects notes everywhere: highlights, icons, export, and the
  // count for the toggles.
  function render() {
    notes.sort((a, b) => a.start - b.start);
    const registry = win.CSS?.highlights;
    if (registry && win.Highlight) {
      if (notes.length)
        registry.set(
          HIGHLIGHT_NAME,
          new win.Highlight(...notes.map((n) => n.range)),
        );
      else registry.delete(HIGHLIGHT_NAME);
    }
    renderIcons();
    exportStale = true;
    content.dispatchEvent(
      new CustomEvent(CHANGE_EVENT, {
        bubbles: true,
        detail: { count: notes.length },
      }),
    );
  }

  // ---- Selection and pen

  function updatePen() {
    if (state) return;
    pending = null;
    const sel = win.getSelection();
    if (sel && sel.rangeCount && !sel.isCollapsed) {
      const range = sel.getRangeAt(0);
      const model = buildTextModel(content);
      const selector = selectorFromRange(model, range);
      // A passage whose context repeats elsewhere would not re-anchor on
      // reload; refusing it now beats a note that vanishes later.
      if (selector && anchor(model, selector)?.start === selector.offset) {
        pending = { selector, model, top: rangeTop(range) };
      }
    }
    if (pending) margin.showPen(column(), pending.top);
    else margin.hidePen();
  }

  // ---- Editor

  // openNew edits a new note for the pending selection; the pen stays shown
  // as the editor's anchor.
  function openNew() {
    if (!pending) return;
    state = { mode: "new", ...pending, source: pen };
    textarea.value = "";
    nav.hidden = true;
    del.hidden = true;
    discard.hidden = false;
    editor.open(pen);
  }

  function openExisting(icon, ids) {
    state = { mode: "edit", ids, index: 0, source: icon };
    icon.setAttribute("aria-expanded", "true");
    nav.hidden = ids.length < 2;
    del.hidden = false;
    discard.hidden = true;
    showNote();
    editor.open(icon);
  }

  function showNote() {
    const { ids, index } = state;
    textarea.value = byID(ids[index]).note;
    position.textContent = L("annotation_position", index + 1, ids.length);
    prev.disabled = index === 0;
    next.disabled = index === ids.length - 1;
  }

  // closeEditor hides the editor, whose beforetoggle handler below resets
  // the state; the platform returns focus to the opener, and focusID, when
  // given, moves it to that note's icon instead.
  function closeEditor(focusID) {
    if (state) editor.hidePopover();
    if (focusID) margin.iconFor(focusID)?.focus();
  }

  // Every way the editor closes passes here: the buttons, Esc, light
  // dismiss, a modal dialog opening.
  on(editor, "beforetoggle", (e) => {
    if (e.newState !== "closed" || !state) return;
    const { mode, ids } = state;
    state = null;
    if (mode === "new") updatePen();
    else margin.iconFor(ids[0])?.setAttribute("aria-expanded", "false");
  });

  function remove(id) {
    const rest = state.ids.filter((other) => other !== id);
    notes = notes.filter((n) => n.id !== id);
    persist();
    render();
    if (rest.length) return closeEditor(rest[0]);
    closeEditor();
    // The group's icon is gone: focus moves to the next control a reader
    // would look for, the header toggle, else the content itself.
    const toggle = doc.querySelector("gmd-annotations-toggle button");
    if (toggle) toggle.focus();
    else {
      content.setAttribute("tabindex", "-1");
      content.focus();
    }
  }

  function saveNote() {
    const text = textarea.value;
    if (state.mode === "edit") {
      const id = state.ids[state.index];
      if (!text.trim()) return remove(id);
      byID(id).note = text;
      persist();
      exportStale = true;
      return closeEditor(id);
    }
    if (!text.trim()) return closeEditor();
    const { selector, model } = state;
    const note = {
      id: newID(win),
      ...selector,
      note: text,
      start: selector.offset,
      range: rangeFromOffsets(
        model,
        selector.offset,
        selector.offset + selector.quote.length,
      ),
    };
    notes.push(note);
    persist();
    render();
    win.getSelection()?.removeAllRanges();
    closeEditor(note.id);
  }

  // ---- Export dialog

  function exportText() {
    const model = buildTextModel(content);
    const page = {
      title:
        config.title ||
        content.querySelector("h1")?.textContent.trim() ||
        doc.title,
      sourcePath: config.sourcePath || "",
      url: win.location.origin + win.location.pathname + win.location.search,
    };
    const items = notes.map((n) => ({
      ...n,
      headingPath: headingPath(
        content,
        model,
        anchor(model, n)?.start ?? n.start,
      ),
    }));
    return formatExport(page, items, {
      exportTitle: L("annotations_export_title"),
      exportSource: L("annotations_export_source"),
      exportPage: L("annotations_export_page"),
      exportIntro: L("annotations_export_intro"),
      exportTop: L("annotations_export_top"),
    });
  }

  // refreshDialog regenerates the export unless the reader edited it and no
  // note changed since. The autofocus attribute picks what the dialog
  // focuses when it opens.
  function refreshDialog() {
    const has = notes.length > 0;
    empty.hidden = has;
    exportArea.hidden = !has;
    copy.disabled = clear.disabled = !has;
    confirmRow.hidden = true;
    exportArea.toggleAttribute("autofocus", has);
    close.toggleAttribute("autofocus", !has);
    if (exportStale || !exportEdited) {
      exportArea.value = has ? exportText() : "";
      exportEdited = exportStale = false;
    }
  }

  async function copyExport() {
    try {
      await win.navigator.clipboard.writeText(exportArea.value);
    } catch {
      return;
    }
    copy.textContent = L("annotations_copied");
    win.setTimeout(() => (copy.textContent = L("annotations_copy")), COPIED_MS);
  }

  // ---- Events

  const actions = {
    save: saveNote,
    discard: () => closeEditor(),
    delete: () => remove(state.ids[state.index]),
    prev: () => {
      state.index--;
      showNote();
    },
    next: () => {
      state.index++;
      showNote();
    },
    copy: copyExport,
    clear: () => {
      confirmRow.hidden = false;
      act("cancel-clear").focus();
    },
    "cancel-clear": () => {
      confirmRow.hidden = true;
      clear.focus();
    },
    "confirm-clear": () => {
      notes = [];
      store.clear();
      render();
      refreshDialog();
      close.focus();
    },
  };
  const onAction = (event) => {
    const name = event.target.closest("[data-action]")?.dataset.action;
    if (name && actions[name]) actions[name]();
  };
  on(editor, "click", onAction);
  on(dialog, "click", onAction);

  // A click on the editor's own opener toggles it: being the source, the
  // opener does not light-dismiss it.
  on(margin, "gmd-annotation-open", (e) => {
    const { source, ids } = e.detail;
    const reopen = state?.source !== source;
    closeEditor();
    if (!reopen) return;
    if (ids) openExisting(source, ids);
    else openNew();
  });

  on(editor, "keydown", (e) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      saveNote();
    }
  });
  on(dialog, "command", (e) => {
    if (e.command === "show-modal") refreshDialog();
  });
  on(exportArea, "input", () => (exportEdited = true));

  on(doc, "selectionchange", updatePen);
  on(doc, "keydown", (e) => {
    // e.code, not e.key: on macOS Alt+Shift+A types "Å".
    if (!(
      e.altKey &&
      e.shiftKey &&
      !e.ctrlKey &&
      !e.metaKey &&
      e.code === "KeyA"
    ))
      return;
    if (state) return;
    updatePen();
    if (!pending) return;
    e.preventDefault();
    openNew();
  });

  on(win, "resize", scheduleLayout);
  doc.fonts?.ready.then(() => !signal.aborted && layout());
  const observer = win.ResizeObserver
    ? new win.ResizeObserver(scheduleLayout)
    : null;
  observer?.observe(content);

  // ---- Re-anchor stored notes

  const model = buildTextModel(content);
  let dropped = 0;
  for (const stored of store.load()) {
    const at =
      typeof stored?.quote === "string" &&
      typeof stored.note === "string" &&
      anchor(model, stored);
    if (!at) {
      dropped++;
      continue;
    }
    notes.push({
      ...stored,
      start: at.start,
      range: rangeFromOffsets(model, at.start, at.end),
    });
  }
  if (dropped) {
    persist();
    notice.announce(P("annotations_removed", dropped), L("annotations_close"));
  }
  render();

  return {
    count: () => notes.length,
    unmount() {
      observer?.disconnect();
      win.CSS?.highlights?.delete(HIGHLIGHT_NAME);
      margin.remove();
      editor.remove();
      dialog.remove();
    },
  };
}

// GmdAnnotations wraps the page content and owns the controller. The notice
// exists from connection on, before any message; the rest mounts after load.
class GmdAnnotations extends Base {
  #abort = null;
  #mounted = null;
  #notice = null;

  get count() {
    return this.#mounted?.count() ?? 0;
  }

  connectedCallback() {
    const doc = this.ownerDocument;
    const win = doc.defaultView;
    this.#abort = new AbortController();
    this.#notice = doc.createElement("gmd-annotation-notice");
    this.#notice.id = NOTICE_ID;
    this.#notice.setAttribute("popover", "manual");
    this.#notice.setAttribute("role", "status");
    doc.body.append(this.#notice);
    const start = () => {
      this.#mounted = mount(doc, win, this, this.#notice, this.#abort.signal);
    };
    if (doc.readyState === "complete") start();
    else
      win.addEventListener("load", start, {
        once: true,
        signal: this.#abort.signal,
      });
  }

  disconnectedCallback() {
    this.#abort.abort();
    this.#mounted?.unmount();
    this.#mounted = null;
    this.#notice.remove();
  }
}

// The module may be inlined more than once on a page; the first run defines
// all five elements, the others skip.
if (
  typeof customElements !== "undefined" &&
  !customElements.get("gmd-annotations")
) {
  customElements.define("gmd-annotations", GmdAnnotations);
  customElements.define("gmd-annotations-toggle", GmdAnnotationsToggle);
  customElements.define("gmd-annotation-margin", GmdAnnotationMargin);
  customElements.define("gmd-annotation-editor", GmdAnnotationEditor);
  customElements.define("gmd-annotation-notice", GmdAnnotationNotice);
}
