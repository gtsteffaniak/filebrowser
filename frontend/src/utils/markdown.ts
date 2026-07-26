import DOMPurify from "dompurify";
import { Marked } from "marked";
import {
  buildPreviewResourceUrl,
  isLocalResourceReference,
} from "@/utils/htmlPreview";
import { buildItemUrl, resolveRelativePath } from "@/utils/url";

export const DIRECTORY_README_MAX_BYTES = 256 * 1024;

const INLINE_MARKDOWN_SANITIZE_CONFIG = {
  ALLOWED_TAGS: [
    "a",
    "blockquote",
    "br",
    "code",
    "del",
    "details",
    "em",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "hr",
    "img",
    "li",
    "ol",
    "p",
    "pre",
    "strong",
    "summary",
    "table",
    "tbody",
    "td",
    "th",
    "thead",
    "tr",
    "ul",
  ],
  ALLOWED_ATTR: ["alt", "href", "loading", "referrerpolicy", "rel", "src", "title"],
  FORBID_TAGS: [
    "audio",
    "button",
    "embed",
    "form",
    "iframe",
    "input",
    "math",
    "object",
    "select",
    "source",
    "svg",
    "textarea",
    "video",
  ],
  FORBID_ATTR: ["autoplay", "class", "formaction", "id", "poster", "srcset", "style"],
};

function hardenInlineMarkdown(html: string): string {
  const template = document.createElement("template");
  template.innerHTML = html;

  for (const image of template.content.querySelectorAll("img")) {
    const src = image.getAttribute("src") || "";
    try {
      const resourceUrl = new URL(src, window.location.origin);
      if (resourceUrl.origin !== window.location.origin ||
          !resourceUrl.pathname.endsWith("/api/resources/download")) {
        image.remove();
        continue;
      }
    } catch {
      image.remove();
      continue;
    }
    image.setAttribute("loading", "lazy");
    image.setAttribute("referrerpolicy", "no-referrer");
  }

  for (const link of template.content.querySelectorAll("a")) {
    link.setAttribute("rel", "noopener noreferrer");
  }

  return template.innerHTML;
}

export function renderMarkdown(
  content: string,
  filePath: string,
  source: string,
  inline = false,
): string {
  const parser = new Marked({ gfm: true });
  if (inline) {
    parser.use({
      renderer: {
        html() {
          return "";
        },
      },
    });
  }
  parser.use({
    walkTokens(token) {
      if (token.type === "image" && token.href) {
        token.href = buildPreviewResourceUrl(token.href, filePath, source);
      }
      if (inline && token.type === "link" && token.href && isLocalResourceReference(token.href)) {
        const suffixIndex = token.href.search(/[?#]/);
        const pathPart = suffixIndex === -1 ? token.href : token.href.slice(0, suffixIndex);
        const suffix = suffixIndex === -1 ? "" : token.href.slice(suffixIndex);
        token.href = `${buildItemUrl(source, resolveRelativePath(filePath, pathPart), true)}${suffix}`;
      }
    },
  });

  const result = parser.parse(content);
  const sanitized = DOMPurify.sanitize(
    typeof result === "string" ? result : "Loading...",
    inline ? INLINE_MARKDOWN_SANITIZE_CONFIG : undefined,
  );
  return inline ? hardenInlineMarkdown(sanitized) : sanitized;
}

export async function readTextResponseUpTo(response: Response, maxBytes: number) {
  if (!response.body) {
    return null;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let total = 0;
  let content = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      return content + decoder.decode();
    }
    total += value.byteLength;
    if (total > maxBytes) {
      await reader.cancel();
      return null;
    }
    content += decoder.decode(value, { stream: true });
  }
}

export function findDirectoryReadme(listing: {
  type?: string;
  items?: Array<{ name?: string; type?: string }>;
}) {
  if (listing?.type !== "directory" || !Array.isArray(listing.items)) {
    return null;
  }

  return listing.items.find(
    (item) => item.type !== "directory" && item.name?.toLowerCase() === "readme.md",
  ) ?? null;
}
