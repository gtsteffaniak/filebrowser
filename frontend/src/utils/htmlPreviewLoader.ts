// Keep iframe document updates out of the browser's joint session history.
// The iframe stays sandboxed without allow-same-origin; only its sanitized
// document and the trusted navigation helper are placed in the blob.
export function createHtmlPreviewLoader(
  createURL: (html: string) => string = (html) =>
    URL.createObjectURL(new Blob([html], { type: "text/html;charset=utf-8" })),
  revokeURL: (url: string) => void = (url) => URL.revokeObjectURL(url),
) {
  let currentURL: string | null = null;
  let currentFrame: Window | null = null;
  let currentHTML: string | null = null;

  return {
    load(frame: Window, html: string) {
      if (frame === currentFrame && html === currentHTML) return;
      const nextURL = createURL(html);
      try {
        // replace(), rather than assigning src/srcdoc, avoids creating an
        // iframe history entry alongside each app-router navigation.
        frame.location.replace(nextURL);
      } catch (error) {
        revokeURL(nextURL);
        throw error;
      }
      const previousURL = currentURL;
      currentURL = nextURL;
      currentFrame = frame;
      currentHTML = html;
      if (previousURL !== null) revokeURL(previousURL);
    },
    dispose() {
      if (currentURL !== null) revokeURL(currentURL);
      currentURL = null;
      currentFrame = null;
      currentHTML = null;
    },
  };
}
