/**
 * TOC Scroll Highlighting — tracks scroll position and highlights
 * the active heading in the table of contents sidebar.
 *
 * Looks for #toc-sidebar (standard) or #toc (minimal theme fallback).
 * Finds all headings in <article> with [id] attributes and matches
 * them against TOC links.
 *
 * Uses the .active class on matching TOC links. Themes control all
 * visual styling via CSS on #toc-sidebar a.active or equivalent.
 *
 * Themes that want full override can place their own toc-highlight.mjs
 * in the theme directory — asset resolution checks theme first.
 */
(function() {
  'use strict';

  var tocSidebar = document.getElementById('toc-sidebar') || document.getElementById('toc');
  if (!tocSidebar) return;

  var headings = document.querySelectorAll('article h1[id], article h2[id], article h3[id], article h4[id]');
  var tocLinks = tocSidebar.querySelectorAll('a');
  if (!headings.length || !tocLinks.length) return;

  // Build a set of heading IDs that have TOC links for quick lookup
  var tocIds = new Set();
  tocLinks.forEach(function(link) {
    var href = link.getAttribute('href');
    if (href && href.startsWith('#')) tocIds.add(href.slice(1));
  });

  var prevId = null;

  function updateActive() {
    // Find the last heading that scrolled past the top of the viewport
    var current = null;
    for (var i = 0; i < headings.length; i++) {
      if (headings[i].getBoundingClientRect().top <= 100) current = headings[i];
    }

    // If current heading isn't in the TOC (e.g. h3/h4 sub-heading),
    // walk backwards to find the nearest ancestor heading that is
    if (current && !tocIds.has(current.id)) {
      var arr = Array.from(headings);
      var idx = arr.indexOf(current);
      while (idx >= 0 && !tocIds.has(arr[idx].id)) idx--;
      current = idx >= 0 ? arr[idx] : null;
    }

    var id = current ? current.id : null;
    if (id === prevId) return;
    prevId = id;

    tocLinks.forEach(function(link) {
      var isActive = id && link.getAttribute('href') === '#' + id;
      link.classList.toggle('active', isActive);
      if (isActive) {
        var linkTop = link.offsetTop - tocSidebar.offsetTop;
        tocSidebar.scrollTo({ top: linkTop - tocSidebar.clientHeight / 2, behavior: 'smooth' });
      }
    });
  }

  window.addEventListener('scroll', updateActive, { passive: true });
  updateActive();
})();
