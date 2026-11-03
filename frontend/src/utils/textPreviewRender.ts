import DOMPurify from "dompurify";
import { Marked } from "marked";
import { IMAGE_FORBID_ATTR, IMAGE_FORBID_TAGS, withImagePolicy } from "./imagePolicy.ts";

/** Renderer kinds returned by GET /api/resources/preview/text. */
export type TextPreviewKind = "plain" | "csv" | "markdown" | "html" | "css" | "code" | "rtf";

/** JSON payload returned by GET /api/resources/preview/text. Content is NOT sanitized by the server. */
export interface TextPreviewPayload {
  content: string;
  kind: TextPreviewKind;
  format: "text" | "html";
  truncated: boolean;
  size: number;
}

// Thumbnail tiles must not run scripts, apply the document's own styles, or load remote
// resources. Forbidding <style> and style attributes keeps the file from restyling the tile.
const TILE_SANITIZE_CONFIG = {
  USE_PROFILES: { html: true, mathMl: true },
  FORBID_TAGS: [
    "style", "link", "meta", "base", "iframe", "object", "embed",
    "form", "button", "textarea", "select", "script",
    ...IMAGE_FORBID_TAGS,
    // Thumbnails omit video. The full viewer keeps it.
    "video",
  ],
  FORBID_ATTR: ["style", ...IMAGE_FORBID_ATTR],
};

// Attributes that make the browser fetch or navigate. Removed only while sanitizing tiles,
// so the shared DOMPurify instance behaves the same for MarkdownViewer.vue.
// "background" is a legacy attribute on table, td, and th that browsers fetch as an image.
const URL_ATTRIBUTES = ["src", "srcset", "href", "xlink:href", "poster", "data", "action", "formaction", "background"];

let tileMode = false;
DOMPurify.addHook("afterSanitizeAttributes", (node: Element) => {
  if (!tileMode || typeof node.removeAttribute !== "function") {
    return;
  }
  for (const attr of URL_ATTRIBUTES) {
    node.removeAttribute(attr);
  }
  // Task-list checkboxes from markdown are kept, but always as disabled checkboxes.
  if (node.tagName === "INPUT") {
    node.setAttribute("type", "checkbox");
    node.setAttribute("disabled", "");
  }
});

/** Sanitizes untrusted HTML for a thumbnail tile. Returns an HTML string safe for v-html. */
export function sanitizeTileHtml(html: string): string {
  tileMode = true;
  try {
    return withImagePolicy(() => DOMPurify.sanitize(html, TILE_SANITIZE_CONFIG) as string);
  } finally {
    tileMode = false;
  }
}

/**
 * Visible text of an HTML fragment. Pass sanitized HTML only; the result is plain text
 * meant for textContent, never for v-html.
 */
export function plainTextFromHtml(html: string): string {
  return new DOMParser().parseFromString(html, "text/html").body.textContent ?? "";
}

const markdown = new Marked({ gfm: true });

/** Renders markdown the same way the viewer does (GFM), then sanitizes for a thumbnail tile. */
export function renderMarkdownTile(source: string): string {
  const html = markdown.parse(source, { async: false }) as string;
  return sanitizeTileHtml(html);
}
