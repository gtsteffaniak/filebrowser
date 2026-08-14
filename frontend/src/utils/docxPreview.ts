import DOMPurify from "dompurify";

/** Sanitize mammoth DOCX HTML before v-html rendering. */
export function sanitizeDocxHtml(html: string): string {
  return DOMPurify.sanitize(html);
}
