export function loadFullIcons() {
  const root = document.documentElement;
  if (root.classList.contains('icons-full')) {
    return Promise.resolve();
  }

  if (!document.fonts?.load) {
    return Promise.resolve();
  }

  // Use the @font-face rule in fonts.css — more reliable than the FontFace API in Firefox.
  return document.fonts.load("24px 'Material Symbols Outlined'")
    .then(() => {
      root.classList.add('icons-full');
    })
    .catch(() => {
      // Keep the core filled subset on failure instead of switching to an unloaded family.
    });
}

export function scheduleFullIcons() {
  const run = () => {
    void loadFullIcons();
  };

  if (window.requestIdleCallback) {
    window.requestIdleCallback(run, { timeout: 5000 });
  } else {
    setTimeout(run, 0);
  }
}
