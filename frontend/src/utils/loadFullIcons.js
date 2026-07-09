function getFullFontUrl() {
  const preload = document.querySelector(
    'link[rel="preload"][href*="material-symbols-core-filled"]',
  );
  if (preload) {
    return preload.href.replace(
      'material-symbols-core-filled.woff2',
      'material-symbols.woff2',
    );
  }
  return '/public/static/fonts/material-symbols.woff2';
}

export function loadFullIcons() {
  const root = document.documentElement;
  if (root.classList.contains('icons-full')) {
    return Promise.resolve();
  }

  function enableFullIcons() {
    root.classList.add('icons-full');
  }

  if (!document.fonts || !window.FontFace) {
    enableFullIcons();
    return Promise.resolve();
  }

  const fullFont = new FontFace(
    'Material Symbols Outlined',
    `url(${getFullFontUrl()}) format('woff2')`,
    { style: 'normal', weight: '400', display: 'swap' },
  );

  return fullFont.load()
    .then((loaded) => {
      document.fonts.add(loaded);
      enableFullIcons();
    })
    .catch(() => document.fonts.load("24px 'Material Symbols Outlined'")
      .then(enableFullIcons)
      .catch(() => {}));
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
