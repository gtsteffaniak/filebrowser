import DOMPurify from "dompurify";
import { getDownloadURL, getDownloadURLPublic } from "@/api/resources";
import { getters, state } from "@/store";
import { resolveRelativePath } from "@/utils/url";

export const HTML_SANITIZE_CONFIG = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  WHOLE_DOCUMENT: true,
  ADD_TAGS: ["link", "script"],
  FORBID_TAGS: ["iframe", "object", "embed", "form", "base", "meta"],
  FORBID_ATTR: ["target", "formaction"],
};

const CSS_URL_PATTERN = /url\(\s*(['"]?)([^'")]+?)\1\s*\)/gi;
const CSS_IMPORT_URL_PATTERN = /@import\s+url\(\s*['"]?([^'")\s]+)['"]?\s*\)/gi;
const CSS_IMPORT_STRING_PATTERN = /@import\s+['"]([^'"]+)['"]/gi;

const REWRITABLE_LINK_RELS = new Set([
  "stylesheet",
  "style",
  "icon",
  "shortcut icon",
  "apple-touch-icon",
  "preload",
  "prefetch",
  "modulepreload",
  "manifest",
  "font",
]);

const RESOURCE_ATTRIBUTES: Array<[string, string]> = [
  ["script", "src"],
  ["link", "href"],
  ["img", "src"],
  ["video", "src"],
  ["video", "poster"],
  ["audio", "src"],
  ["source", "src"],
  ["track", "src"],
  ["image", "href"],
  ["image", "xlink:href"],
  ["use", "href"],
  ["use", "xlink:href"],
  ["a", "href"],
];

const ALLOWED_ABSOLUTE_URI_PATTERN = /^(https?:|data:|mailto:|tel:|#)/i;

export function isLocalResourceReference(href: string): boolean {
  if (!href || typeof href !== "string") {
    return false;
  }
  const trimmed = href.trim();
  if (trimmed.startsWith("#")) {
    return false;
  }
  return !/^[a-z][a-z0-9+.-]*:/i.test(trimmed) && !trimmed.startsWith("//");
}

export function buildPreviewResourceUrl(
  href: string,
  baseFilePath: string,
  source: string,
): string {
  if (!href || !isLocalResourceReference(href)) {
    return href;
  }

  const resolvedPath = resolveRelativePath(baseFilePath, href);

  try {
    if (getters.isShare()) {
      return getDownloadURLPublic(
        {
          path: state.shareInfo.subPath,
          hash: state.shareInfo.hash,
          token: state.shareInfo.token,
        },
        [resolvedPath],
        true,
      );
    }
    return getDownloadURL(source, resolvedPath, true);
  } catch {
    return href;
  }
}

function shouldRewriteLinkHref(element: Element): boolean {
  const rel = (element.getAttribute("rel") ?? "stylesheet").toLowerCase();
  return rel.split(/\s+/).some((token) => REWRITABLE_LINK_RELS.has(token));
}

function rewriteAttribute(
  element: Element,
  attribute: string,
  baseFilePath: string,
  source: string,
): void {
  const value = element.getAttribute(attribute);
  if (!value) {
    return;
  }
  element.setAttribute(attribute, buildPreviewResourceUrl(value, baseFilePath, source));
}

export function rewriteSrcset(
  srcset: string,
  baseFilePath: string,
  source: string,
): string {
  return srcset
    .split(",")
    .map((part) => {
      const trimmed = part.trim();
      if (!trimmed) {
        return "";
      }
      const spaceIndex = trimmed.search(/\s/);
      const url = spaceIndex === -1 ? trimmed : trimmed.slice(0, spaceIndex);
      const descriptor = spaceIndex === -1 ? "" : trimmed.slice(spaceIndex + 1).trim();
      const rewrittenUrl = buildPreviewResourceUrl(url, baseFilePath, source);
      return descriptor ? `${rewrittenUrl} ${descriptor}` : rewrittenUrl;
    })
    .filter(Boolean)
    .join(", ");
}

export function rewriteCssContent(
  css: string,
  baseFilePath: string,
  source: string,
): string {
  let result = css.replace(CSS_URL_PATTERN, (_match, quote: string, url: string) => {
    const rewritten = buildPreviewResourceUrl(url.trim(), baseFilePath, source);
    return `url(${quote}${rewritten}${quote})`;
  });
  result = result.replace(CSS_IMPORT_URL_PATTERN, (_match, url: string) => {
    const rewritten = buildPreviewResourceUrl(url.trim(), baseFilePath, source);
    return `@import url(${rewritten})`;
  });
  result = result.replace(CSS_IMPORT_STRING_PATTERN, (_match, url: string) => {
    const rewritten = buildPreviewResourceUrl(url.trim(), baseFilePath, source);
    return `@import "${rewritten}"`;
  });
  return result;
}

function blockUnsafeUriAttributes(
  _node: Element,
  data: { attrName: string; attrValue: string; keepAttr: boolean },
): void {
  if (data.attrName !== "href" && data.attrName !== "src" && data.attrName !== "xlink:href") {
    return;
  }
  const value = data.attrValue.trim();
  if (!/^[a-z][a-z0-9+.-]*:/i.test(value)) {
    return;
  }
  if (!ALLOWED_ABSOLUTE_URI_PATTERN.test(value)) {
    data.keepAttr = false;
  }
}

export function rewriteHtmlResources(
  doc: Document,
  baseFilePath: string,
  source: string,
): void {
  for (const [tag, attribute] of RESOURCE_ATTRIBUTES) {
    doc.querySelectorAll(tag).forEach((element) => {
      if (!element.hasAttribute(attribute)) {
        return;
      }
      if (tag === "link" && !shouldRewriteLinkHref(element)) {
        return;
      }
      if (tag === "a") {
        const href = element.getAttribute("href");
        if (!href || !isLocalResourceReference(href)) {
          return;
        }
      }
      rewriteAttribute(element, attribute, baseFilePath, source);
    });
  }

  doc.querySelectorAll("img[srcset], source[srcset]").forEach((element) => {
    const srcset = element.getAttribute("srcset");
    if (srcset) {
      element.setAttribute("srcset", rewriteSrcset(srcset, baseFilePath, source));
    }
  });
}

export function rewriteDocumentStyles(
  doc: Document,
  baseFilePath: string,
  source: string,
): void {
  doc.querySelectorAll("style").forEach((styleEl) => {
    const css = styleEl.textContent;
    if (css) {
      styleEl.textContent = rewriteCssContent(css, baseFilePath, source);
    }
  });

  doc.querySelectorAll("[style]").forEach((element) => {
    const style = element.getAttribute("style");
    if (style) {
      element.setAttribute("style", rewriteCssContent(style, baseFilePath, source));
    }
  });
}

export interface HtmlPreview {
  srcdoc: string;
}

export function buildHtmlPreview(
  content: string,
  filePath: string,
  source: string,
): HtmlPreview {
  const doc = new DOMParser().parseFromString(content, "text/html");
  rewriteHtmlResources(doc, filePath, source);
  rewriteDocumentStyles(doc, filePath, source);

  const serialized = `<!DOCTYPE html>\n${doc.documentElement.outerHTML}`;

  DOMPurify.addHook("uponSanitizeAttribute", blockUnsafeUriAttributes);
  try {
    const srcdoc = String(DOMPurify.sanitize(serialized, HTML_SANITIZE_CONFIG));
    return { srcdoc };
  } finally {
    DOMPurify.removeHook("uponSanitizeAttribute", blockUnsafeUriAttributes);
  }
}
