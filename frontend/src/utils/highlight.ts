import { getters } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.js";

type HLJSApi = typeof import("highlight.js").default;

// Same style element id as MarkdownViewer.vue, so both use one global highlight theme.
const THEME_STYLE_ID = "highlight-theme-style";

// highlight.js is large, so it loads on first use and stays in its own chunk.
let hljsPromise: Promise<HLJSApi> | null = null;

export function loadHljs(): Promise<HLJSApi> {
  if (hljsPromise === null) {
    hljsPromise = import("highlight.js")
      .then((mod) => mod.default)
      .catch((err) => {
        hljsPromise = null; // allow a later call to retry
        throw err;
      });
  }
  return hljsPromise;
}

const themeCssPromises = new Map<"light" | "dark", Promise<string>>();

function loadThemeCss(variant: "light" | "dark"): Promise<string> {
  let promise = themeCssPromises.get(variant);
  if (promise === undefined) {
    promise = (
      variant === "dark"
        ? import("highlight.js/styles/github-dark.min.css?raw")
        : import("highlight.js/styles/github.min.css?raw")
    ).then((mod) => mod.default);
    themeCssPromises.set(variant, promise);
  }
  return promise;
}

/** Injects the highlight.js theme for the current color mode, matching the file viewer. */
export async function applyHighlightTheme(isDark: boolean = getters.isDarkMode()): Promise<void> {
  const css = await loadThemeCss(isDark ? "dark" : "light");
  const nonce = typeof globalVars.cspNonce === "string" ? globalVars.cspNonce : "";
  let style = document.getElementById(THEME_STYLE_ID) as HTMLStyleElement | null;
  if (!style) {
    style = document.createElement("style");
    style.id = THEME_STYLE_ID;
    document.head.appendChild(style);
  }
  if (nonce) {
    style.setAttribute("nonce", nonce);
  }
  style.textContent = css;
}
