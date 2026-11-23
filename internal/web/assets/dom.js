"use strict";

// Dom: the small DOM helpers of every page with script (web.go puts this file
// first, before logic.js and the page's own script).
const Dom = (() => {
  // A new element with an optional class and text.
  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  // Copies text to the clipboard; false if that was not possible, after
  // offering the text to copy by hand. navigator.clipboard needs a secure
  // context (HTTPS or localhost), which a home server like
  // http://raspberrypi:6369 is not, so fall back to the old execCommand route.
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch { /* fall through */ }
    const ta = el("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.append(ta);
    ta.focus();
    ta.select();
    let ok = false;
    try { ok = document.execCommand("copy"); } catch { ok = false; }
    ta.remove();
    if (!ok) window.prompt("Kopieren nicht möglich, bitte selbst kopieren:", text);
    return ok;
  }

  return { el, copyText };
})();
