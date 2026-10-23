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
// module's own UI), rendered output that is not prose (KaTeX, Mermaid) and
// elements whose text never renders (raw HTML <style> or <script>).
export const SKIP_SELECTOR =
  ".copy-btn, gmd-heading-anchor, gmd-color-chip, .katex, .language-mermaid, .mermaid, [data-gmd-annotation-ui], style, script, template";

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

const HIGHLIGHT_NAME = "gmd-annotation";
const LINE_TOLERANCE = 8; // px: first lines closer than this share an icon
const MARGIN_GAP = 8; // px between the content's right edge and the icons
const ICON_SIZE = 28; // px, matches gmd-annotations.css
const COPIED_MS = 2000;

const SVG_ATTRS =
  'aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"';
const PEN_SVG = `<svg ${SVG_ATTRS}><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>`;
const NOTE_SVG = `<svg ${SVG_ATTRS}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`;

// build parses one element of the annotation UI from markup. Every element in
// it carries data-gmd-annotation-ui, which keeps it out of the text model.
function build(doc, html) {
  const template = doc.createElement("template");
  template.innerHTML = html.trim();
  const root = template.content.firstElementChild;
  for (const node of [root, ...root.querySelectorAll("*")]) {
    node.setAttribute("data-gmd-annotation-ui", "");
  }
  return root;
}

const escapeHTML = (s) => s.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);

const button = (action, text, label) =>
  `<button type="button" data-action="${action}"${label ? ` aria-label="${label}"` : ""}>${text}</button>`;

function readConfig(doc) {
  try {
    return JSON.parse(doc.getElementById("gmd-annotations-config").textContent);
  } catch {
    return {};
  }
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

// trapTab keeps Tab and Shift+Tab cycling inside container.
function trapTab(event, container) {
  const focusable = [...container.querySelectorAll("button, textarea")].filter(
    (node) => !node.disabled && !node.closest("[hidden]"),
  );
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const active = container.ownerDocument.activeElement;
  if (event.shiftKey ? active === first : active === last) {
    event.preventDefault();
    (event.shiftKey ? last : first).focus();
  } else if (!container.contains(active)) {
    event.preventDefault();
    first.focus();
  }
}

// mount builds the interface over content once the page has loaded, so
// KaTeX, Mermaid and the other shared scripts are done changing the DOM
// before stored notes are re-anchored.
function mount(doc, win, content, notice, signal) {
  const on = (target, type, fn, opts = {}) =>
    target.addEventListener(type, fn, { ...opts, signal });
  const config = readConfig(doc);
  const L = (name, ...nums) => {
    let i = 0;
    const label = config.labels?.[name] ?? name;
    return label.replace(/%d/g, () => String(nums[i++] ?? ""));
  };
  const store = createStore(localStorageOf(win), storageKey(win.location));

  let notes = []; // stored fields plus the live range and its model start
  let pending = null; // the current annotatable selection
  let state = null; // the open popover
  let warnedNotKept = false;
  let exportEdited = false;
  let exportStale = true;
  let layoutQueued = false;

  // ---- Elements

  // P picks the name_one label for a count of 1 when the locale has one.
  const P = (name, n) =>
    L(n === 1 && config.labels?.[`${name}_one`] ? `${name}_one` : name, n);
  const E = (name, ...nums) => escapeHTML(L(name, ...nums));
  const layer = build(
    doc,
    `<div class="gmd-annotation-layer">
      <button type="button" data-gmd-annotation-pen aria-label="${E("annotate")}" title="${E("annotate")} (Alt+Shift+A)" hidden>${PEN_SVG}</button>
      <div></div>
      <div data-gmd-annotation-popover role="dialog" aria-modal="true" aria-label="${E("annotation_note")}" hidden>
        <div class="gmd-annotation-row">${button("prev", "‹", E("annotation_prev"))}<span data-gmd-annotation-position aria-live="polite"></span>${button("next", "›", E("annotation_next"))}</div>
        <textarea rows="5" aria-label="${E("annotation_note")}"></textarea>
        <div class="gmd-annotation-row">${button("delete", E("annotation_delete"))}<span class="gmd-annotation-spacer"></span>${button("discard", E("annotation_discard"))}${button("save", E("annotation_save"))}</div>
      </div>
    </div>`,
  );
  const dialog = build(
    doc,
    `<dialog data-gmd-annotations-dialog aria-labelledby="gmd-annotations-title">
      <h2 id="gmd-annotations-title">${E("annotations_open")}</h2>
      <p>${E("annotations_empty")}</p>
      <textarea aria-label="${E("annotations_open")}" spellcheck="false"></textarea>
      <div class="gmd-annotation-row">${button("clear", E("annotations_clear"))}<span class="gmd-annotation-spacer"></span>${button("copy", E("annotations_copy"))}${button("close", E("annotations_close"))}</div>
      <div class="gmd-annotation-row" hidden><span class="gmd-annotation-spacer">${E("annotations_clear_confirm")}</span>${button("cancel-clear", E("annotations_cancel"))}${button("confirm-clear", E("annotations_clear_yes"))}</div>
    </dialog>`,
  );
  const [pen, icons, popover] = layer.children;
  const [nav, textarea] = popover.children;
  const [prev, position, next] = nav.children;
  const act = (root, name) => root.querySelector(`[data-action="${name}"]`);
  const [del, discard] = [act(popover, "delete"), act(popover, "discard")];
  const [, empty, exportArea, , confirmRow] = dialog.children;
  const [copy, clear, close] = ["copy", "clear", "close"].map((name) =>
    act(dialog, name),
  );
  // In body, not next to content: a theme may clip its content column
  // (overflow: hidden), which would hide the margin icons.
  doc.body.append(layer);
  doc.body.append(dialog);
  let dialogOpener = null;

  const toggles = [...doc.querySelectorAll("[data-gmd-annotations-toggle]")];
  const badges = toggles.map((toggle) => {
    let badge = toggle.querySelector("[data-gmd-annotations-count]");
    if (!badge) {
      badge = build(doc, "<span data-gmd-annotations-count hidden></span>");
      toggle.append(badge);
    }
    badge.setAttribute("aria-hidden", "true");
    return badge;
  });
  doc.documentElement.setAttribute("data-gmd-annotations", "");

  // ---- Notes

  const byID = (id) => notes.find((n) => n.id === id);

  const persist = () => {
    // The live range and model start stay in memory only.
    store.save(notes.map(({ range, start, ...stored }) => stored));
    if (!store.persistent && !warnedNotKept) {
      warnedNotKept = true;
      announce(L("annotations_not_kept"));
    }
  };

  function announce(message) {
    notice.replaceChildren(
      build(
        doc,
        `<div class="gmd-annotation-notice-box"><span>${escapeHTML(message)}</span><button type="button" aria-label="${E("annotations_close")}">×</button></div>`,
      ),
    );
    on(notice.querySelector("button"), "click", () => notice.replaceChildren());
  }

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

  function place(node, x, top) {
    const origin = layer.getBoundingClientRect();
    node.style.left = `${x - origin.left}px`;
    node.style.top = `${top - origin.top}px`;
  }

  const iconIDs = new WeakMap(); // icon -> the note ids it opens
  const iconFor = (id) =>
    [...icons.children].find((icon) => iconIDs.get(icon).includes(id));

  function renderIcons() {
    const focused = doc.activeElement?.closest?.("[data-gmd-annotation-icon]");
    const focusedID = focused && iconIDs.get(focused)?.[0];
    const groups = groupByLine(
      notes.map((n) => ({ id: n.id, top: rangeTop(n.range) })),
      LINE_TOLERANCE,
    );
    const x = column();
    icons.replaceChildren(
      ...groups.map((group) => {
        group.ids.sort((a, b) => byID(a).start - byID(b).start);
        const n = group.ids.length;
        const label = n > 1 ? E("annotations_count", n) : E("annotation_note");
        const icon = build(
          doc,
          `<button type="button" data-gmd-annotation-icon data-count="${n}" aria-label="${label}" aria-haspopup="dialog" aria-expanded="false">${NOTE_SVG}${n > 1 ? `<span>${n}</span>` : ""}</button>`,
        );
        iconIDs.set(icon, group.ids);
        place(icon, x, group.top - 2);
        return icon;
      }),
    );
    if (state?.mode === "edit") {
      iconFor(state.ids[0])?.setAttribute("aria-expanded", "true");
    }
    if (focusedID) iconFor(focusedID)?.focus();
  }

  function layout() {
    renderIcons();
    if (pending) place(pen, column(), pending.top - 2);
    if (state) placePopover();
  }

  function scheduleLayout() {
    if (layoutQueued) return;
    layoutQueued = true;
    win.requestAnimationFrame(() => {
      layoutQueued = false;
      layout();
    });
  }

  // render reflects notes everywhere: highlights, icons, badge, export.
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
    for (const [i, toggle] of toggles.entries()) {
      badges[i].textContent = String(notes.length);
      badges[i].hidden = notes.length === 0;
      toggle.setAttribute(
        "aria-label",
        notes.length
          ? P("annotations_open_count", notes.length)
          : L("annotations_open"),
      );
    }
    exportStale = true;
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
    pen.hidden = !pending;
    if (pending) place(pen, column(), pending.top - 2);
  }

  // ---- Popover

  function placePopover() {
    const top =
      state.mode === "new" ? state.top : rangeTop(byID(state.ids[0]).range);
    const width = popover.offsetWidth;
    place(
      popover,
      Math.max(8, column() + ICON_SIZE - width),
      top + ICON_SIZE + 4,
    );
  }

  function showPopover() {
    const single = state.mode === "new" || state.ids.length < 2;
    nav.hidden = single;
    del.hidden = state.mode === "new";
    discard.hidden = state.mode !== "new";
    popover.hidden = false;
    placePopover();
    textarea.focus();
  }

  function openNew(opener) {
    if (!pending) return;
    state = { mode: "new", ...pending, opener };
    pen.hidden = true;
    textarea.value = "";
    showPopover();
  }

  function openExisting(icon) {
    state = { mode: "edit", ids: iconIDs.get(icon), index: 0, opener: icon };
    icon.setAttribute("aria-expanded", "true");
    showNote();
    showPopover();
  }

  function showNote() {
    const { ids, index } = state;
    textarea.value = byID(ids[index]).note;
    position.textContent = L("annotation_position", index + 1, ids.length);
    prev.disabled = index === 0;
    next.disabled = index === ids.length - 1;
  }

  // closePopover hides the popover; focus goes to the icon of focusID when
  // given, else back to the opener when restore is set.
  function closePopover({ restore = true, focusID } = {}) {
    if (!state) return;
    const { mode, ids } = state;
    // Icons are rebuilt on layout: a detached opener means its group's new icon.
    let { opener } = state;
    if (mode === "edit" && !opener.isConnected) opener = iconFor(ids[0]);
    state = null;
    popover.hidden = true;
    opener?.setAttribute?.("aria-expanded", "false");
    if (mode === "new") updatePen();
    if (focusID) iconFor(focusID)?.focus();
    else if (restore && opener?.isConnected && !opener.hidden) opener.focus();
  }

  function remove(id) {
    const rest = state.ids.filter((other) => other !== id);
    notes = notes.filter((n) => n.id !== id);
    persist();
    render();
    if (rest.length) return closePopover({ restore: false, focusID: rest[0] });
    closePopover({ restore: false });
    // The group's icon is gone: focus moves to the next control a reader
    // would look for, the header toggle, else the content itself.
    if (toggles[0]) toggles[0].focus();
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
      return closePopover({ focusID: id });
    }
    if (!text.trim()) return closePopover();
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
    state.opener = null;
    win.getSelection()?.removeAllRanges();
    closePopover({ focusID: note.id });
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
  // note changed since.
  function refreshDialog() {
    const has = notes.length > 0;
    empty.hidden = has;
    exportArea.hidden = !has;
    copy.disabled = clear.disabled = !has;
    confirmRow.hidden = true;
    if (exportStale || !exportEdited) {
      exportArea.value = has ? exportText() : "";
      exportEdited = exportStale = false;
    }
  }

  function openDialog(opener) {
    closePopover({ restore: false });
    dialogOpener = opener;
    refreshDialog();
    dialog.showModal();
    (notes.length ? exportArea : close).focus();
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
    discard: () => closePopover(),
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
      act(dialog, "cancel-clear").focus();
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
    close: () => dialog.close(),
  };
  const onAction = (event) => {
    const name = event.target.closest("[data-action]")?.dataset.action;
    if (name && actions[name]) actions[name]();
  };
  on(popover, "click", onAction);
  on(dialog, "click", onAction);

  on(pen, "mousedown", (e) => e.preventDefault()); // keep the selection
  on(pen, "click", () => openNew(pen));
  on(icons, "click", (e) => {
    const icon = e.target.closest("[data-gmd-annotation-icon]");
    if (icon) openExisting(icon);
  });
  for (const toggle of toggles) on(toggle, "click", () => openDialog(toggle));

  on(popover, "keydown", (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      closePopover();
    } else if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      saveNote();
    } else if (e.key === "Tab") trapTab(e, popover);
  });
  on(dialog, "keydown", (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      dialog.close();
    } else if (e.key === "Tab") trapTab(e, dialog);
  });
  on(dialog, "close", () => dialogOpener?.focus());
  on(exportArea, "input", () => (exportEdited = true));

  on(doc, "selectionchange", updatePen);
  on(
    doc,
    "pointerdown",
    (e) => {
      if (state && !popover.contains(e.target))
        closePopover({ restore: false });
    },
    { capture: true },
  );
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
    openNew(doc.activeElement);
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
    announce(P("annotations_removed", dropped));
  }
  render();

  return () => {
    observer?.disconnect();
    win.CSS?.highlights?.delete(HIGHLIGHT_NAME);
    layer.remove();
    dialog.remove();
  };
}

// init wires the interface on a page whose content carries
// data-gmd-annotatable and returns a handle whose destroy() removes it; on
// any other page it does nothing and returns null (tests import this module
// with no marker).
export function init(doc, win) {
  const content = doc.querySelector("[data-gmd-annotatable]");
  if (!content) return null;
  const controller = new AbortController();
  // The live region exists before any message so screen readers announce it.
  const notice = build(
    doc,
    '<div data-gmd-annotation-notice role="status"></div>',
  );
  doc.body.append(notice);
  let unmount = null;
  const start = () => {
    unmount = mount(doc, win, content, notice, controller.signal);
  };
  if (doc.readyState === "complete") start();
  else
    win.addEventListener("load", start, {
      once: true,
      signal: controller.signal,
    });
  return {
    destroy() {
      controller.abort();
      unmount?.();
      notice.remove();
      doc.documentElement.removeAttribute("data-gmd-annotations");
    },
  };
}

if (typeof document !== "undefined") init(document, window);
