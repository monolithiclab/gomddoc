/**
 * Client-side strings — the one place modules read their translations.
 *
 * Served at /_assets/gmd-i18n.mjs (it sits under shared/static/, the directory
 * the static file system exposes); the other shared modules are inlined and
 * import it by that absolute path. The server emits one JSON data block per
 * namespace: <script type="application/json" data-gmd-i18n="search">. See
 * docs/specs/20261023.client-i18n-design.md.
 *
 * Placeholders follow Go's printf so the same strings work in templates: %s
 * and %d take the next argument, %[n]s and %[n]d take argument n (1-based) and
 * let a translator reorder, %% is a literal %. A missing argument renders as
 * an empty string. The result is plain text: callers set textContent or escape
 * before building markup.
 */

const PLACEHOLDER = /%(?:%|(?:\[(\d+)\])?[sd])/g;

// One parsed dictionary per document and namespace, so a block is read once.
const cache = new WeakMap();

function load(namespace, doc) {
  let byNamespace = cache.get(doc);
  if (!byNamespace) {
    byNamespace = new Map();
    cache.set(doc, byNamespace);
  }
  if (!byNamespace.has(namespace)) {
    const blocks = doc.querySelectorAll(
      `script[type="application/json"][data-gmd-i18n="${CSS.escape(namespace)}"]`,
    );
    if (blocks.length > 1) {
      console.warn(`gmd-i18n: several "${namespace}" blocks, using the first`);
    }
    let dict = {};
    try {
      if (blocks.length) dict = JSON.parse(blocks[0].textContent) ?? {};
    } catch {
      // Invalid JSON: every lookup falls back to its key.
    }
    byNamespace.set(namespace, dict);
  }
  return byNamespace.get(namespace);
}

function format(template, args) {
  let next = 0;
  return template.replace(PLACEHOLDER, (match, index) => {
    if (match === "%%") return "%";
    const i = index ? Number(index) - 1 : next;
    next = i + 1;
    return args[i] ?? "";
  });
}

function pluralRules(doc) {
  try {
    return new Intl.PluralRules(doc.documentElement.lang);
  } catch {
    return new Intl.PluralRules();
  }
}

export function i18n(namespace, doc = document) {
  const dict = load(namespace, doc);
  let rules;
  const t = (key, ...args) => format(dict[key] ?? key, args);
  const tn = (key, n, ...args) => {
    rules ??= pluralRules(doc);
    const variant = `${key}_${rules.select(n)}`;
    return t(variant in dict ? variant : key, n, ...args);
  };
  return { t, tn };
}
