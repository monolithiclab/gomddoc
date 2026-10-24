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
 *   3. Fallback for an empty button: line-icon SVGs (moon in light mode, sun in
 *      dark mode), drawn in currentColor and sized 1em.
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

  var ICON_OPEN = '<svg aria-hidden="true" width="1em" height="1em" viewBox="0 0 24 24" fill="none" ' +
    'stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">';
  var ICONS = {
    light: ICON_OPEN + '<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>',
    dark: ICON_OPEN + '<circle cx="12" cy="12" r="5"></circle><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42' +
      'M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"></path></svg>'
  };

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
      toggle.innerHTML = ICONS[theme === 'dark' ? 'dark' : 'light'];
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
