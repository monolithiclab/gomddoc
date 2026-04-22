/**
 * Code Copy Button — adds copy-to-clipboard buttons to code blocks.
 *
 * Finds all <pre><code> blocks and appends a .copy-btn button.
 * Themes control all visual styling via CSS on .copy-btn and .copied.
 *
 * Customization via CSS custom properties:
 *   --copy-btn-font-size, --copy-btn-padding, etc. — set in theme CSS.
 *
 * Customization via data attributes on <html>:
 *   data-copy-label     — button text (default: "Copy")
 *   data-copied-label   — success text (default: "Copied!")
 *
 * Themes that want full override can place their own code-copy.mjs
 * in the theme directory — asset resolution checks theme first.
 */
(function() {
  'use strict';

  var root = document.documentElement;
  var copyLabel = root.getAttribute('data-copy-label') || 'Copy';
  var copiedLabel = root.getAttribute('data-copied-label') || 'Copied!';

  document.querySelectorAll('pre > code').forEach(function(codeBlock) {
    var pre = codeBlock.parentNode;
    var btn = document.createElement('button');
    btn.className = 'copy-btn';
    btn.textContent = copyLabel;
    btn.setAttribute('aria-label', 'Copy code to clipboard');

    btn.addEventListener('click', function() {
      navigator.clipboard.writeText(codeBlock.textContent).then(function() {
        btn.textContent = copiedLabel;
        btn.classList.add('copied');
        setTimeout(function() {
          btn.textContent = copyLabel;
          btn.classList.remove('copied');
        }, 2000);
      }).catch(function(err) {
        console.error('Failed to copy:', err);
      });
    });

    pre.appendChild(btn);
  });
})();
