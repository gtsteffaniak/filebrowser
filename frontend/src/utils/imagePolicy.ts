import DOMPurify from "dompurify";

// Thumbnails (tiles) for HTML and markdown files never render images, whether the reference is
// relative, absolute, or data:. The full viewer is not affected: it keeps images and videos.

/** Elements that display an image. Their content is dropped with them. */
export const IMAGE_FORBID_TAGS = ["img", "image", "picture"];

/** Attributes that make the browser fetch an image. */
export const IMAGE_FORBID_ATTR = ["srcset", "background"];

// SVG elements that fetch an external file when their href points outside the document.
const EXTERNAL_HREF_ELEMENTS = new Set(["use", "feimage", "image"]);

type AttributeHookData = { attrName: string; attrValue: string; keepAttr: boolean };

function imagePolicyHook(node: Element, data: AttributeHookData): void {
  const tag = node.nodeName.toLowerCase();
  const name = data.attrName;
  const value = data.attrValue.trim();

  if ((name === "href" || name === "xlink:href") && EXTERNAL_HREF_ELEMENTS.has(tag)) {
    // In-document references like "#icon" are inline SVG and stay. Anything else is a fetch.
    if (!value.startsWith("#")) data.keepAttr = false;
    return;
  }
  // CSS url() can pull in a background image.
  if (name === "style" && /url\s*\(/i.test(value)) {
    data.keepAttr = false;
    return;
  }
  // <input type="image" src="..."> is an image button.
  if (name === "src" && tag === "input") {
    data.keepAttr = false;
  }
}

/**
 * Runs a DOMPurify sanitize call with the image policy hook active. Only the calls made inside
 * `sanitize` see the hook, so other callers of the shared DOMPurify instance are unchanged.
 */
export function withImagePolicy<T>(sanitize: () => T): T {
  DOMPurify.addHook("uponSanitizeAttribute", imagePolicyHook);
  try {
    return sanitize();
  } finally {
    DOMPurify.removeHook("uponSanitizeAttribute", imagePolicyHook);
  }
}
