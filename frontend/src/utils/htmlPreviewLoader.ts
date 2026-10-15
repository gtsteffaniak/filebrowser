// Keep iframe document updates out of the browser's joint session history.
// With allow-same-origin (and no allow-scripts), blob documents are same-origin
// so subresources use the session cookie; only sanitized HTML is loaded here.
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
