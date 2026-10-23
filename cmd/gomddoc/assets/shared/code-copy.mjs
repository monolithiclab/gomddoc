/**
 * Code Copy Button — adds copy-to-clipboard buttons to code blocks.
 *
 * Finds all <pre><code> blocks and appends a .copy-btn button.
 * Themes control all visual styling via CSS on .copy-btn and .copied.
 *
 * Customization via CSS custom properties:
 *   --copy-btn-font-size, --copy-btn-padding, etc. — set in theme CSS.
 *
 * Strings come from the page's "code_copy" block (gmd-i18n.mjs):
 *   code_copy_label — button text
 *   code_copy_done  — success text
 *   code_copy_aria  — the button's accessible name
 *
 * Themes that want full override can place their own code-copy.mjs
 * in the theme directory — asset resolution checks theme first.
 */
import { i18n } from "/_assets/gmd-i18n.mjs";

(function() {
  var t = i18n('code_copy').t;
  var copyLabel = t('code_copy_label');
  var copiedLabel = t('code_copy_done');
  var copyAria = t('code_copy_aria');

  document.querySelectorAll('pre > code').forEach(function(codeBlock) {
    var pre = codeBlock.parentNode;
    var btn = document.createElement('button');
    btn.className = 'copy-btn';
    btn.textContent = copyLabel;
    btn.setAttribute('aria-label', copyAria);

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
