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

// Per document: its parsed dictionaries (a block is read once) and its plural
// rules (constructed once).
const cache = new WeakMap();

function entry(doc) {
  let e = cache.get(doc);
  if (!e) {
    let rules;
    try {
      rules = new Intl.PluralRules(doc.documentElement.lang);
    } catch {
      rules = new Intl.PluralRules();
    }
    e = { dicts: new Map(), rules };
    cache.set(doc, e);
  }
  return e;
}

function load(namespace, doc, dicts) {
  if (!dicts.has(namespace)) {
    const blocks = [
      ...doc.querySelectorAll('script[type="application/json"][data-gmd-i18n]'),
    ].filter((el) => el.dataset.gmdI18n === namespace);
    if (blocks.length > 1) {
      console.warn(`gmd-i18n: several "${namespace}" blocks, using the first`);
    }
    let dict = {};
    try {
      const parsed = blocks.length ? JSON.parse(blocks[0].textContent) : {};
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        dict = parsed;
      }
    } catch {
      // Invalid JSON: every lookup falls back to its key.
    }
    dicts.set(namespace, dict);
  }
  return dicts.get(namespace);
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

export function i18n(namespace, doc = document) {
  const { dicts, rules } = entry(doc);
  const dict = load(namespace, doc, dicts);
  // Own string values only: "constructor" or a numeric value is not a string.
  const has = (k) => Object.hasOwn(dict, k) && typeof dict[k] === "string";
  const t = (key, ...args) => format(has(key) ? dict[key] : key, args);
  const tn = (key, n, ...args) => {
    const category = rules.select(n);
    // "other" is the bare key; a stray key_other is not a variant.
    const variant = `${key}_${category}`;
    return t(category !== "other" && has(variant) ? variant : key, n, ...args);
  };
  return { t, tn };
}
