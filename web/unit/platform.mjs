// happy-dom lacks parts of the web platform the annotation UI relies on. This
// one helper adds just enough of them: the Popover API (showPopover/
// hidePopover, beforetoggle, light dismiss, Esc, focus return) and Invoker
// Commands, each only where missing, and a modal <dialog>'s initial focus,
// Esc and focus return, by wrapping showModal/close unconditionally (happy-dom
// has both methods, without that behaviour). It installs itself on import, so
// import it before gmd-annotations.mjs: the module defines its elements only
// where these features exist. The Playwright suite proves the real behaviour
// in Chrome.

const showing = new Set();
const sources = new WeakMap(); // popover -> the source it was shown with
const returnFocus = new WeakMap(); // popover or dialog -> previous focus

// popoverOpen stands in for el.matches(":popover-open").
export const popoverOpen = (el) => showing.has(el);

// deepActive is the focused element, through open shadow roots.
export function deepActive(doc = document) {
  let el = doc.activeElement;
  while (el?.shadowRoot?.activeElement) el = el.shadowRoot.activeElement;
  return el;
}

const toggleEvent = (newState) =>
  Object.assign(new Event("beforetoggle"), { newState });

function focusFirst(container) {
  const target =
    container.querySelector("[autofocus]") ??
    container.querySelector("button:not([disabled]), textarea, input");
  target?.focus();
}

function restoreFocus(el) {
  const previous = returnFocus.get(el);
  returnFocus.delete(el);
  if (el.contains(deepActive(el.ownerDocument)) && previous?.isConnected) {
    previous.focus();
  }
}

function installPlatform(win = window) {
  const doc = win.document;
  const proto = win.HTMLElement.prototype;

  if (!("showPopover" in proto)) {
    proto.showPopover = function (options = {}) {
      if (showing.has(this)) return;
      this.dispatchEvent(toggleEvent("open"));
      showing.add(this);
      sources.set(this, options.source ?? null);
      returnFocus.set(this, deepActive(doc));
    };
    proto.hidePopover = function () {
      if (!showing.has(this)) return;
      this.dispatchEvent(toggleEvent("closed"));
      showing.delete(this);
      restoreFocus(this);
    };
    const autoPopovers = () =>
      [...showing].filter((el) => el.getAttribute("popover") === "auto");

    // Light dismiss: a pointerdown outside an auto popover and its source.
    doc.addEventListener(
      "pointerdown",
      (e) => {
        const path = e.composedPath();
        for (const el of autoPopovers()) {
          if (path.includes(el) || path.includes(sources.get(el))) continue;
          returnFocus.delete(el);
          el.hidePopover();
        }
      },
      { capture: true },
    );
    // Esc closes the topmost auto popover, else an open modal dialog.
    doc.addEventListener("keydown", (e) => {
      if (e.key !== "Escape" || e.defaultPrevented) return;
      const top = autoPopovers().pop();
      if (top) return top.hidePopover();
      const dialog = doc.querySelector("dialog[open]");
      if (dialog && returnFocus.has(dialog)) dialog.close();
    });
  }

  const { showModal, close } = win.HTMLDialogElement.prototype;
  win.HTMLDialogElement.prototype.showModal = function () {
    returnFocus.set(this, deepActive(doc));
    for (const el of showing) {
      if (el.getAttribute("popover") === "auto") el.hidePopover();
    }
    showModal.call(this);
    focusFirst(this);
  };
  win.HTMLDialogElement.prototype.close = function (value) {
    const wasOpen = this.open;
    close.call(this, value);
    if (wasOpen) restoreFocus(this);
  };

  if (!("commandForElement" in win.HTMLButtonElement.prototype)) {
    Object.defineProperty(
      win.HTMLButtonElement.prototype,
      "commandForElement",
      {
        configurable: true,
        get() {
          return (
            this.getRootNode().getElementById?.(
              this.getAttribute("commandfor"),
            ) ?? null
          );
        },
      },
    );
    doc.addEventListener("click", (e) => {
      const invoker = e.target.closest?.("button[commandfor]");
      if (!invoker) return;
      const target = invoker.commandForElement;
      const command = invoker.getAttribute("command");
      const event = Object.assign(new Event("command", { cancelable: true }), {
        command,
        source: invoker,
      });
      if (!target || !target.dispatchEvent(event)) return;
      if (command === "show-modal" && !target.open) target.showModal();
      else if (command === "close" && target.open) target.close();
      else if (command === "hide-popover") target.hidePopover();
      else if (command === "show-popover") target.showPopover();
    });
  }
}

installPlatform();
