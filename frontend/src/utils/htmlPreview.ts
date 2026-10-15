import DOMPurify from "dompurify";
import { getters, state } from "@/store/index.ts";
import { getCachedViewToken } from "@/api/viewToken.js";
import { getViewURL } from "@/api/resources.js";
import { buildItemUrl, getParentDir, resolveRelativePath } from "@/utils/url.js";

export const HTML_SANITIZE_CONFIG = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  WHOLE_DOCUMENT: true,
  ADD_TAGS: ["link"],
  FORBID_TAGS: ["iframe", "object", "embed", "form", "base", "meta", "script"],
  FORBID_ATTR: ["target", "formaction"],
};

/** Sandboxed HTML preview iframe flags (no allow-scripts — scripts must never share an origin with the parent). */
export const HTML_PREVIEW_SANDBOX =
  "allow-same-origin allow-popups allow-top-navigation-by-user-activation";

export const HTML_PREVIEW_CSP =
  "default-src 'none'; script-src 'none'; img-src 'self' data: blob:; media-src 'self' data: blob:; style-src 'self' 'unsafe-inline' data:; font-src 'self' data: blob:; connect-src 'none'; base-uri 'none'; form-action 'none'; frame-src 'none'";

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

function viewTokenForSibling(
  resolvedPath: string,
  baseFilePath: string,
): string | undefined {
  const items = state.req?.parentDirItems;
  if (!items?.length) {
    return undefined;
  }

  const exact = items.find((entry) => entry.path === resolvedPath);
  if (exact?.viewToken) {
    return exact.viewToken;
  }

  const resolvedDir = getParentDir(resolvedPath);
  const baseDir = getParentDir(baseFilePath);
  if (resolvedDir !== baseDir) {
    return undefined;
  }

  const name = resolvedPath.split("/").filter(Boolean).pop();
  if (!name) {
    return undefined;
  }

  const item = items.find((entry) => {
    if (entry.name !== name) {
      return false;
    }
    if (entry.path) {
      return getParentDir(entry.path) === resolvedDir;
    }
    return resolvedDir === baseDir;
  });
  return item?.viewToken;
}

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
  const viewToken =
    viewTokenForSibling(resolvedPath, baseFilePath) ??
    state.req?.viewToken ??
    getCachedViewToken(source);

  try {
    const viewUrl = getters.isShare()
      ? getViewURL(
          source,
          resolvedPath,
          viewToken,
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          false,
          resolvedPath,
        )
      : getViewURL(source, resolvedPath, viewToken, null, false, resolvedPath);
    return viewUrl ?? href;
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
    for (const element of doc.querySelectorAll(tag)) {
      if (!element.hasAttribute(attribute)) {
        continue;
      }
      if (tag === "link" && !shouldRewriteLinkHref(element)) {
        continue;
      }
      if (tag === "a") {
        const href = element.getAttribute("href");
        if (!href || !isLocalResourceReference(href)) {
          continue;
        }
      }
      rewriteAttribute(element, attribute, baseFilePath, source);
    }
  }

  for (const element of doc.querySelectorAll("img[srcset], source[srcset]")) {
    const srcset = element.getAttribute("srcset");
    if (srcset) {
      element.setAttribute("srcset", rewriteSrcset(srcset, baseFilePath, source));
    }
  }
}

export function rewriteDocumentStyles(
  doc: Document,
  baseFilePath: string,
  source: string,
): void {
  for (const styleEl of doc.querySelectorAll("style")) {
    const css = styleEl.textContent;
    if (css) {
      styleEl.textContent = rewriteCssContent(css, baseFilePath, source);
    }
  }

  for (const element of doc.querySelectorAll("[style]")) {
    const style = element.getAttribute("style");
    if (style) {
      element.setAttribute("style", rewriteCssContent(style, baseFilePath, source));
    }
  }
}

function buildHtmlViewerUrl(source: string, filePath: string): string {
  const pathOnly = filePath.split(/[?#]/)[0];
  return new URL(buildItemUrl(source, pathOnly, true), window.location.origin).href;
}

/** Same-origin FileBrowser viewer routes only (used to gate target="_top"). */
export function isAllowedTopNavigationUrl(href: string, source: string): boolean {
  try {
    const url = new URL(href, window.location.origin);
    if (url.origin !== window.location.origin || url.username || url.password) {
      return false;
    }
    const prefix = new URL(buildItemUrl(source, "/", true), window.location.origin);
    const prefixPath = prefix.pathname.endsWith("/")
      ? prefix.pathname
      : `${prefix.pathname}/`;
    if (!url.pathname.startsWith(prefixPath)) {
      return false;
    }
    const relative = decodeURIComponent(url.pathname.slice(prefixPath.length));
    // biome-ignore lint/suspicious/noControlCharactersInRegex: Reject control characters in navigation paths.
    if (!relative || !/\.html?$/i.test(relative) || /[\x00-\x1f\\]/.test(relative)) {
      return false;
    }
    return !relative.split("/").some((part) => part === "." || part === "..");
  } catch {
    return false;
  }
}

function applyPreviewLinkTargets(doc: ParentNode, source: string, pageViewerUrls: Set<string>): void {
  for (const anchor of doc.querySelectorAll("a[href]")) {
    const href = anchor.getAttribute("href") ?? "";
    if (pageViewerUrls.has(href) && isAllowedTopNavigationUrl(href, source)) {
      anchor.setAttribute("target", "_top");
      continue;
    }
    if (/^https?:/i.test(href)) {
      anchor.setAttribute("target", "_blank");
      anchor.setAttribute("rel", "noopener noreferrer");
    }
  }
}

function wrapPreviewDocument(htmlEl: HTMLElement): string {
  const doc = htmlEl.ownerDocument;
  const head = doc.head ?? htmlEl.querySelector("head");
  if (head) {
    const charset = doc.createElement("meta");
    charset.setAttribute("charset", "utf-8");
    head.insertBefore(charset, head.firstChild);

    const csp = doc.createElement("meta");
    csp.setAttribute("http-equiv", "Content-Security-Policy");
    csp.setAttribute("content", HTML_PREVIEW_CSP);
    head.insertBefore(csp, charset.nextSibling);
  }
  return `<!DOCTYPE html>\n${htmlEl.outerHTML}`;
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
  const pageLinks = new Map<string, string>();
  for (const anchor of doc.querySelectorAll("a[href]")) {
    const href = anchor.getAttribute("href") ?? "";
    if (!isLocalResourceReference(href)) continue;
    const resolved = resolveRelativePath(filePath, href);
    const pathOnly = resolved.split(/[?#]/)[0];
    if (!pathOnly || !/\.html?$/i.test(pathOnly)) continue;
    pageLinks.set(
      buildPreviewResourceUrl(href, filePath, source),
      buildHtmlViewerUrl(source, pathOnly),
    );
  }
  rewriteHtmlResources(doc, filePath, source);
  rewriteDocumentStyles(doc, filePath, source);

  const serialized = `<!DOCTYPE html>\n${doc.documentElement.outerHTML}`;

  DOMPurify.addHook("uponSanitizeAttribute", blockUnsafeUriAttributes);
  try {
    const sanitized = DOMPurify.sanitize(serialized, {
      ...HTML_SANITIZE_CONFIG,
      RETURN_DOM: true,
    }) as unknown as HTMLElement;

    for (const anchor of sanitized.querySelectorAll("a")) {
      anchor.removeAttribute("data-filebrowser-html-link");
    }

    const pageViewerUrls = new Set<string>();
    for (const anchor of sanitized.querySelectorAll("a[href]")) {
      const viewerURL = pageLinks.get(anchor.getAttribute("href") ?? "");
      if (!viewerURL || !isAllowedTopNavigationUrl(viewerURL, source)) {
        continue;
      }
      anchor.setAttribute("href", viewerURL);
      pageViewerUrls.add(viewerURL);
    }

    applyPreviewLinkTargets(sanitized, source, pageViewerUrls);

    return { srcdoc: wrapPreviewDocument(sanitized) };
  } finally {
    DOMPurify.removeHook("uponSanitizeAttribute", blockUnsafeUriAttributes);
  }
}
