/**
 * Theme Toggle — shared dark/light mode state management.
 *
 * Finds the #theme-toggle button, manages localStorage persistence,
 * system preference detection, and button content updates.
 *
 * Button content strategies (checked in order):
 *   1. Children with [data-show-theme] attributes — shown/hidden by theme.
 *      Example: <button id="theme-toggle">
 *                 <svg data-show-theme="light">moon</svg>
 *                 <svg data-show-theme="dark">sun</svg>
 *               </button>
 *
 *   2. data-light / data-dark attributes — set as textContent.
 *      Example: <button id="theme-toggle" data-light="Dark Mode" data-dark="Light Mode">
 *
 *   3. Default emoji fallback: light shows moon, dark shows sun.
 *
 * Themes that want full override can place their own theme-toggle.mjs
 * in the theme directory — asset resolution checks theme first.
 */
(function() {
  'use strict';

  var toggle = document.getElementById('theme-toggle');
  if (!toggle) return;

  var html = document.documentElement;

  // Initialize theme: localStorage > existing data-theme > system preference
  var stored = localStorage.getItem('theme');
  if (stored) {
    html.setAttribute('data-theme', stored);
  } else if (!html.getAttribute('data-theme')) {
    html.setAttribute('data-theme',
      window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  }

  // Detect content strategy
  var themedChildren = toggle.querySelectorAll('[data-show-theme]');
  var hasThemedChildren = themedChildren.length > 0;
  var lightAttr = toggle.getAttribute('data-light');
  var darkAttr = toggle.getAttribute('data-dark');
  var hasDataAttrs = lightAttr !== null && darkAttr !== null;

  function currentTheme() {
    return html.getAttribute('data-theme') ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  }

  function updateToggle() {
    var theme = currentTheme();
    toggle.setAttribute('data-active-theme', theme);

    if (hasThemedChildren) {
      themedChildren.forEach(function(el) {
        el.style.display = el.getAttribute('data-show-theme') === theme ? '' : 'none';
      });
    } else if (hasDataAttrs) {
      toggle.textContent = theme === 'dark' ? darkAttr : lightAttr;
    } else {
      toggle.textContent = theme === 'dark' ? '\u2600\uFE0F' : '\uD83C\uDF19';
    }
  }

  updateToggle();

  toggle.addEventListener('click', function() {
    var next = currentTheme() === 'dark' ? 'light' : 'dark';
    html.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    updateToggle();
  });

  // Respond to system preference changes when no stored preference
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function() {
    if (!localStorage.getItem('theme')) {
      html.setAttribute('data-theme',
        window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
      updateToggle();
    }
  });
})();
