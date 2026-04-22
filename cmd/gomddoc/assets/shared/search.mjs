// Search Modal — shared across all gomddoc themes
// Loaded via {{ inlineJSAsset "search.mjs" }} in scripts.html.tmpl
(function () {
  'use strict';

  // Inject styles using CSS custom properties for cross-theme compatibility
  const style = document.createElement('style');
  style.textContent = `
    #search-modal {
      display: none;
      position: fixed;
      inset: 0;
      z-index: 200;
      align-items: flex-start;
      justify-content: center;
      padding-top: 12vh;
    }
    #search-modal.open {
      display: flex;
    }
    .search-backdrop {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      backdrop-filter: blur(4px);
      -webkit-backdrop-filter: blur(4px);
    }
    .search-dialog {
      position: relative;
      width: 90%;
      max-width: 560px;
      background: var(--color-bg, #ffffff);
      border: 1px solid var(--color-border, #e2e8f0);
      border-radius: 12px;
      box-shadow: var(--shadow-lg, 0 20px 60px rgba(0, 0, 0, 0.3));
      overflow: hidden;
      display: flex;
      flex-direction: column;
      max-height: 70vh;
    }
    .search-input-wrapper {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      border-bottom: 1px solid var(--color-border, #e2e8f0);
    }
    .search-input-wrapper svg {
      flex-shrink: 0;
      color: var(--color-text-muted, #94a3b8);
    }
    #search-input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      color: var(--color-text, #1e293b);
      font-family: var(--font-family, system-ui, sans-serif);
      font-size: 16px;
      line-height: 1.5;
    }
    #search-input::placeholder {
      color: var(--color-text-muted, #94a3b8);
    }
    .search-shortcut {
      flex-shrink: 0;
      padding: 2px 6px;
      border: 1px solid var(--color-border, #e2e8f0);
      border-radius: 4px;
      font-size: 11px;
      font-family: var(--font-family-mono, monospace);
      color: var(--color-text-muted, #94a3b8);
      background: var(--color-bg-secondary, #f8fafc);
    }
    #search-results {
      overflow-y: auto;
      max-height: calc(70vh - 100px);
    }
    #search-results:empty {
      display: none;
    }
    .search-result {
      display: block;
      padding: 10px 16px;
      text-decoration: none;
      color: var(--color-text, #1e293b);
      border-bottom: 1px solid var(--color-border, #e2e8f0);
      transition: background 0.1s;
    }
    .search-result:last-child {
      border-bottom: none;
    }
    .search-result:hover,
    .search-result[aria-selected="true"] {
      background: var(--color-primary-bg, var(--color-bg-secondary, #f1f5f9));
    }
    .search-result-title {
      font-weight: 600;
      font-size: 14px;
      margin-bottom: 2px;
    }
    .search-result-path {
      font-size: 12px;
      color: var(--color-text-muted, #94a3b8);
      margin-bottom: 4px;
    }
    .search-result-snippet {
      font-size: 13px;
      color: var(--color-text-secondary, #64748b);
      line-height: 1.4;
    }
    .search-result-snippet mark {
      background: var(--color-primary-bg, rgba(37, 99, 235, 0.15));
      color: var(--color-primary, #2563eb);
      padding: 1px 2px;
      border-radius: 2px;
    }
    .search-empty {
      padding: 24px 16px;
      text-align: center;
      color: var(--color-text-muted, #94a3b8);
      font-size: 14px;
    }
    .search-footer {
      display: flex;
      gap: 16px;
      padding: 8px 16px;
      border-top: 1px solid var(--color-border, #e2e8f0);
      font-size: 11px;
      color: var(--color-text-muted, #94a3b8);
    }
    .search-footer kbd {
      padding: 1px 4px;
      border: 1px solid var(--color-border, #e2e8f0);
      border-radius: 3px;
      font-family: var(--font-family-mono, monospace);
      font-size: 10px;
      background: var(--color-bg-secondary, #f8fafc);
    }
    @media (max-width: 640px) {
      #search-modal.open {
        padding-top: 0;
      }
      .search-dialog {
        width: 100%;
        max-width: 100%;
        max-height: 100vh;
        height: 100vh;
        border-radius: 0;
        border: none;
      }
      #search-results {
        max-height: calc(100vh - 100px);
      }
      .search-footer {
        display: none;
      }
    }
  `;
  document.head.appendChild(style);

  // Read localized strings from <html> data attributes
  const root = document.documentElement;
  const i18n = {
    placeholder: root.getAttribute('data-search-placeholder') || 'Search documentation...',
    noResults: root.getAttribute('data-search-no-results') || 'No results found',
    navigate: root.getAttribute('data-search-navigate') || 'Navigate',
    open: root.getAttribute('data-search-open') || 'Open',
    close: root.getAttribute('data-search-close') || 'Close',
    ariaLabel: root.getAttribute('data-search-aria') || 'Search documentation',
  };

  // Create modal DOM
  const modal = document.createElement('div');
  modal.id = 'search-modal';
  modal.setAttribute('role', 'dialog');
  modal.setAttribute('aria-modal', 'true');
  modal.setAttribute('aria-label', i18n.ariaLabel);
  modal.innerHTML = `
    <div class="search-backdrop"></div>
    <div class="search-dialog">
      <div class="search-input-wrapper">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
        </svg>
        <input type="search" id="search-input" placeholder="${i18n.placeholder}"
               autocomplete="off" aria-label="Search query">
        <kbd class="search-shortcut">Esc</kbd>
      </div>
      <div id="search-results" role="listbox" aria-label="Search results"></div>
      <div class="search-footer">
        <span><kbd>&uarr;</kbd><kbd>&darr;</kbd> ${i18n.navigate}</span>
        <span><kbd>&crarr;</kbd> ${i18n.open}</span>
        <span><kbd>Esc</kbd> ${i18n.close}</span>
      </div>
    </div>
  `;
  document.body.appendChild(modal);

  const backdrop = modal.querySelector('.search-backdrop');
  const input = modal.querySelector('#search-input');
  const resultsContainer = modal.querySelector('#search-results');

  let activeIndex = -1;
  let debounceTimer;
  let currentResults = [];

  function open() {
    modal.classList.add('open');
    input.value = '';
    resultsContainer.innerHTML = '';
    activeIndex = -1;
    currentResults = [];
    requestAnimationFrame(() => input.focus());
  }

  function close() {
    modal.classList.remove('open');
    activeIndex = -1;
  }

  function isOpen() {
    return modal.classList.contains('open');
  }

  function search(query) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(async () => {
      const q = query.trim();
      if (!q) {
        resultsContainer.innerHTML = '';
        currentResults = [];
        activeIndex = -1;
        return;
      }

      try {
        const res = await fetch('/api/search?q=' + encodeURIComponent(q) + '&limit=10');
        if (!res.ok) return;
        currentResults = await res.json();
        renderResults();
      } catch {
        // silently ignore fetch errors
      }
    }, 200);
  }

  function renderResults() {
    activeIndex = -1;

    if (currentResults.length === 0) {
      resultsContainer.innerHTML = '<div class="search-empty">' + i18n.noResults + '</div>';
      return;
    }

    resultsContainer.innerHTML = currentResults.map((r, i) =>
      '<a class="search-result" href="' + escapeAttr(r.path) + '" ' +
        'role="option" data-index="' + i + '">' +
        '<div class="search-result-title">' + escapeHTML(r.title) + '</div>' +
        '<div class="search-result-path">' + escapeHTML(r.path) + '</div>' +
        (r.snippet ? '<div class="search-result-snippet">' + r.snippet + '</div>' : '') +
      '</a>'
    ).join('');
  }

  function setActive(index) {
    const items = resultsContainer.querySelectorAll('.search-result');
    if (items.length === 0) return;

    // Clamp index
    if (index < 0) index = items.length - 1;
    if (index >= items.length) index = 0;

    items.forEach(el => el.removeAttribute('aria-selected'));
    items[index].setAttribute('aria-selected', 'true');
    items[index].scrollIntoView({ block: 'nearest' });
    activeIndex = index;
  }

  function navigateToActive() {
    const items = resultsContainer.querySelectorAll('.search-result');
    if (items.length === 0) return;

    // Default to first result if none selected
    const index = activeIndex >= 0 ? activeIndex : 0;
    if (index < items.length) {
      close();
      window.location.href = items[index].href;
    }
  }

  function escapeHTML(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  function escapeAttr(str) {
    return str.replace(/&/g, '&amp;').replace(/"/g, '&quot;');
  }

  // Event: search button
  const toggle = document.getElementById('search-toggle');
  if (toggle) {
    toggle.addEventListener('click', (e) => {
      e.preventDefault();
      open();
    });
  }

  // Event: keyboard shortcut (Ctrl+K / Cmd+K)
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      if (isOpen()) {
        close();
      } else {
        open();
      }
      return;
    }

    if (!isOpen()) return;

    if (e.key === 'Escape') {
      e.preventDefault();
      close();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive(activeIndex + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive(activeIndex - 1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      navigateToActive();
    }
  });

  // Event: backdrop click
  backdrop.addEventListener('click', close);

  // Event: input
  input.addEventListener('input', () => search(input.value));
})();
