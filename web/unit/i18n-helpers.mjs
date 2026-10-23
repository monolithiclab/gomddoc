// block returns the markup of one data-gmd-i18n block, as the server emits it.
// A string dict is written as is, so a test can supply invalid JSON.
export function block(ns, dict) {
  const json = typeof dict === "string" ? dict : JSON.stringify(dict);
  return `<script type="application/json" data-gmd-i18n="${ns}">${json}</script>`;
}
