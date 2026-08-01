<template>
  <div id="markedown-viewer" ref="scrollContainer">
    <iframe
      v-if="isHtml"
      ref="viewer"
      :key="req.path"
      class="html-content"
      :srcdoc="htmlPreview.srcdoc"
      sandbox="allow-scripts allow-popups allow-same-origin"
      referrerpolicy="no-referrer"
      title="HTML preview"
      @load="htmlPreviewHeight"
    ></iframe>
    <div v-else class="markdown-content-container" :class="{ 'dark-mode': darkMode }">
      <div ref="viewer" class="markdown-content">
        <div
          v-for="block in renderedContent"
          :key="block.key"
          class="md-block"
          :data-line="block.line"
          :data-code-line="block.codeLine"
          v-html="block.html"
        ></div>
      </div>
    </div>
    <div v-if="!splitMode && !isHtml" class="spacer" :style="{ height: `${spaceForStatusBar}em` }"></div>
  </div>
</template>

<script lang="ts">
import type { PropType } from "vue";
import type { HLJSApi } from 'highlight.js';
import { Marked, Token } from "marked";
import DOMPurify from 'dompurify';
import { state, mutations, getters } from "@/store";
import { createScrollSyncGuard } from "@/utils/markdownScrollSync";
import { copyToClipboard } from "@/utils/clipboard";
import { globalVars } from "@/utils/constants";
import { isHtmlMimeType } from "@/utils/mimetype";
import {
  buildHtmlPreview,
  buildPreviewResourceUrl,
  rewriteHtmlResources,
  rewriteDocumentStyles,
} from "@/utils/htmlPreview";

// Lazy load highlight.js -- it was making the viewer bloated, so now is on its own chunk grouped with its theme
let hljsPromise: Promise<HLJSApi> | null = null;
function loadHljs(): Promise<HLJSApi> {
  if (hljsPromise === null) {
    hljsPromise = import('highlight.js').then((mod) => mod.default).catch((err) => {
      hljsPromise = null; // allow a later render to retry
      throw err;
    });
  }
  return hljsPromise;
}

const highlightCssPromises = new Map<"light" | "dark", Promise<string>>();
function loadHighlightCss(variant: "light" | "dark"): Promise<string> {
  let promise = highlightCssPromises.get(variant);
  if (promise === undefined) {
    promise = (
      variant === "dark"
        ? import("highlight.js/styles/github-dark.min.css?raw")
        : import("highlight.js/styles/github.min.css?raw")
    ).then((mod) => mod.default);
    highlightCssPromises.set(variant, promise);
  }
  return promise;
}

const MD_SANITIZE_CONFIG = { USE_PROFILES: { html: true, mathMl: true }, ADD_TAGS: ["semantics", "annotation"] };

const marked = new Marked({ gfm: true });
marked.use({
  extensions: [{
    name: "blockKatexInterrupt",
    level: "block",
    start(src: string) {
      const m = src.match(/\n\${1,2}[ \t]*(?:\n|$)/);
      return m ? m.index : undefined;
    },
    tokenizer() { return undefined; },
  }],
});

// Lazy load katex + mhchem (math + chemistry) -- so md files that don't have math/chemistry notation simply will not load katex
// making the file load more faster, similar thing to the async components.
const MATH_PATTERN = /\$\$[\s\S]+?\$\$|\$[^\s$][^$\n]*\$/;
function contentHasMath(content: string): boolean {
  return MATH_PATTERN.test(content);
}
let katexLoaded = false;
let katexPromise: Promise<void> | null = null;
function loadKatex(): Promise<void> {
  if (katexLoaded) return Promise.resolve();
  if (katexPromise === null) {
    katexPromise = Promise.all([
      import("marked-katex-extension"),
      import("katex/contrib/mhchem"), // To render chemistry formulas
    ]).then(([{ default: markedKatex }]) => {
      marked.use(markedKatex({ throwOnError: false, output: "mathml" }));
      katexLoaded = true;
    }).catch((err) => {
      console.error("Failed to load katex:", err);
      katexPromise = null;
      throw err;
    });
  }
  return katexPromise;
}

// Void elements that not always need a closing tag
const VOID_ELEMENTS = new Set([
  "area", "base", "br", "col", "embed", "hr", "img", "input",
  "link", "meta", "param", "source", "track", "wbr",
]);

// open-tag count in raw HTML tokens, used to regroup tokens split across
function htmlTagBalance(raw: string): number {
  const tagPattern = /<\/?([a-zA-Z][a-zA-Z0-9-]*)\b[^>]*>/g;
  let balance = 0;
  let match: RegExpExecArray | null;
  while ((match = tagPattern.exec(raw))) {
    const [full, name] = match;
    if (VOID_ELEMENTS.has(name.toLowerCase()) || full.endsWith("/>")) {
      continue;
    }
    balance += full.startsWith("</") ? -1 : 1;
  }
  return balance;
}

// v-for block keys
function hashText(text: string): string {
  let hash = 5381;
  for (let i = 0; i < text.length; i++) {
    hash = ((hash << 5) + hash + text.charCodeAt(i)) | 0;
  }
  return hash.toString(36);
}

// Rewrites resource attributes inside a HTML block written in the markdown
function rewriteHtmlBlockForMd(html: string, filePath: string, source: string): string {
  const doc = new DOMParser().parseFromString(`<!doctype html><body>${html}</body>`, "text/html");
  rewriteHtmlResources(doc, filePath, source);
  rewriteDocumentStyles(doc, filePath, source);
  return doc.body.innerHTML;
}

export default {
  name: "markdownViewer",
  props: {
    splitMode: {
      type: Boolean,
      default: false,
    },
    liveContent: {
      type: String,
      default: null,
    },
    scrollTarget: {
      type: Object as PropType<HTMLElement | null>,
      default: null, // When null, falls back to the components own root (non-split view)
    },
  },
  data() {
    return {
      content: "",
      scrollGuard: createScrollSyncGuard(),
      boundScrollEl: null as HTMLElement | null,
      isLoadingNewContent: false,
      katexReady: false,
      htmlResizeObserver: null as ResizeObserver | null,
      htmlContentHeight: 0,
    };
  },
  methods: {
    htmlPreviewHeight() {
      if (!this.isHtml) return;
      const iframe = this.$refs.viewer as HTMLIFrameElement | undefined;
      if (!iframe) return;
      let contentHeight = 0;
      try {
        contentHeight = (iframe.contentWindow?.document?.documentElement?.scrollHeight || 0) + 25; // 25px of extra room, otherwise feels "stuck" on mobile
      } catch { /* ignore */ }
      this.htmlContentHeight = contentHeight;
      this.applyHtmlPreviewHeight();
    },
    applyHtmlPreviewHeight() {
      const iframe = this.$refs.viewer as HTMLIFrameElement | undefined;
      if (!iframe || !this.isHtml) return;
      const available = window.innerHeight - iframe.getBoundingClientRect().top;
      iframe.style.height = `${Math.max(available, this.htmlContentHeight)}px`;
    },
    observeHtmlResize() {
      if (this.htmlResizeObserver) return;
      window.addEventListener("resize", this.applyHtmlPreviewHeight);
      this.htmlResizeObserver = new ResizeObserver(() => this.applyHtmlPreviewHeight());
      this.htmlResizeObserver.observe(this.$el);
    },
    unobserveHtmlResize() {
      if (!this.htmlResizeObserver) return;
      window.removeEventListener("resize", this.applyHtmlPreviewHeight);
      this.htmlResizeObserver.disconnect();
      this.htmlResizeObserver = null;
    },
    async setHighlightTheme(isDark: boolean) {
      const THEME_STYLE_ID = "highlight-theme-style";
      const themeMode = await loadHighlightCss(isDark ? "dark" : "light");
      const nonce =
        typeof globalVars.cspNonce === "string" && globalVars.cspNonce !== ""
          ? globalVars.cspNonce
          : "";
      let style = document.getElementById(THEME_STYLE_ID) as HTMLStyleElement | null;
      if (!style) {
        style = document.createElement("style");
        style.id = THEME_STYLE_ID;
        if (nonce) {
          style.setAttribute("nonce", nonce);
        }
        document.head.appendChild(style);
      } else if (nonce) {
        style.setAttribute("nonce", nonce);
      }
      style.textContent = themeMode;
    },
    // Highlights code blocks and adds line numbers
    async applyHighlighting() {
      const viewer = this.$refs.viewer as HTMLElement;
      if (!viewer?.querySelector('pre code')) return;
      void this.setHighlightTheme(getters.isDarkMode());
      let hljs: HLJSApi | null = null;
      try {
        hljs = await loadHljs();
      } catch (err) {
        console.error("Failed to load highlight.js:", err);
      }
      // Re-query in case content changed while highlight.js was loading
      viewer.querySelectorAll('pre code').forEach((block) => {
        const codeBlock = block as HTMLElement;
        if (codeBlock.classList.contains("line-numbers-added")) return;
        const langClass = codeBlock.className.split(/\s+/).find(c => c.startsWith('language-'));
        const lang = langClass ? langClass.split('-')[1] : null;

        if (hljs && lang && hljs.getLanguage(lang)) {
          hljs.highlightElement(codeBlock);
        } else {
          codeBlock.classList.add('hljs');
        }
        this.addLineNumbers(codeBlock);
      });
    },
    // Manual line numbers implementation
    addLineNumbers(codeBlock: HTMLElement) {
      const code = codeBlock.textContent || '';
      const lines = code.split('\n');

      // Remove trailing empty lines
      if (lines[lines.length - 1] === '') {
        lines.pop();
      }

      // Don't add line numbers if already added
      if (codeBlock.classList.contains('line-numbers-added')) {
        return;
      }

      // Create a wrapper div
      const wrapper = document.createElement('div');
      wrapper.className = 'code-block-wrapper';

      // Create copy button
      const copyButton = document.createElement('button');
      copyButton.className = 'copy-code-button';
      copyButton.innerHTML = '<span class="material-symbols-outlined">content_copy</span>';
      copyButton.setAttribute('aria-label', 'Copy code to clipboard');
      copyButton.addEventListener('click', (e) => {
        e.stopPropagation();
        const text = codeBlock.textContent || '';
        const showFeedback = (success: boolean) => {
          copyButton.innerHTML = success
            ? '<span class="material-symbols-outlined">check</span>'
            : '<span class="material-symbols-outlined">error</span>';
          setTimeout(() => {
            copyButton.innerHTML = '<span class="material-symbols-outlined">content_copy</span>';
          }, 1500);
        };
        void copyToClipboard(text)
          .then((success) => {
            showFeedback(success);
          })
          .catch((err) => {
            console.error('Copy failed:', err);
            showFeedback(false);
          });
      });
      wrapper.appendChild(copyButton);

      // Create line numbers container
      const lineNumbers = document.createElement('div');
      lineNumbers.className = 'line-numbers';

      // Create code content container
      const codeContent = document.createElement('div');
      codeContent.className = 'code-content';

      // Get the highlighted HTML content and split it into lines
      const highlightedHTML = codeBlock.innerHTML;
      const htmlLines = this.splitHighlightedHTML(highlightedHTML, lines.length);

      // Absolute source line the code content starts on, gives each line its own anchor
      const blockEl = codeBlock.closest<HTMLElement>('.md-block');
      const parsedStart = Number(blockEl?.dataset.codeLine);
      const codeStartLine = Number.isFinite(parsedStart) ? parsedStart : null;

      // Create code lines with preserved highlighting
      const codeLines = htmlLines.map((lineHTML, index) => {
        const lineElement = document.createElement('div');
        lineElement.className = 'code-line';
        lineElement.setAttribute('data-line', (index + 1).toString());
        if (codeStartLine !== null) {
          lineElement.dataset.sourceLine = String(codeStartLine + index);
        }
        lineElement.innerHTML = lineHTML;
        return lineElement;
      });

      // Generate line numbers with click handlers
      for (let i = 1; i <= lines.length; i++) {
        const lineNumber = document.createElement('span');
        lineNumber.className = 'line-number';
        lineNumber.textContent = i.toString();
        lineNumber.setAttribute('data-line', i.toString());

        // Add click handler for line highlighting
        lineNumber.addEventListener('click', () => {
          // Check if this line is already active
          const isCurrentlyActive = lineNumber.classList.contains('active');

          // Remove previous highlights
          wrapper.querySelectorAll('.code-line.highlighted').forEach(el => {
            el.classList.remove('highlighted');
          });
          wrapper.querySelectorAll('.line-number.active').forEach(el => {
            el.classList.remove('active');
          });

          // If the line wasn't already active, highlight it
          if (!isCurrentlyActive) {
            const targetLine = wrapper.querySelector(`.code-line[data-line="${i}"]`);
            if (targetLine) {
              targetLine.classList.add('highlighted');
              lineNumber.classList.add('active');
            }
          }
          // If it was already active, we've already cleared it above
        });

        lineNumbers.appendChild(lineNumber);
      }

      // Create new code block with individual lines
      const newCodeBlock = document.createElement('code');
      newCodeBlock.className = codeBlock.className;
      newCodeBlock.classList.add('line-numbers-added');

      // Add all code lines to the new code block
      codeLines.forEach(line => {
        newCodeBlock.appendChild(line);
      });

      // Create new pre element
      const newPre = document.createElement('pre');
      newPre.appendChild(newCodeBlock);
      codeContent.appendChild(newPre);

      // Insert wrapper before the original code block
      codeBlock.parentNode?.insertBefore(wrapper, codeBlock);

      // Add line numbers and code content to wrapper
      wrapper.appendChild(lineNumbers);
      wrapper.appendChild(codeContent);

      // Remove the original code block
      codeBlock.remove();
    },

    // Helper method to split highlighted HTML while preserving syntax highlighting
    splitHighlightedHTML(html: string, expectedLines: number): string[] {
      const temp = document.createElement('div');
      temp.innerHTML = html;
      const textContent = temp.textContent || '';
      const textLines = textContent.split('\n');

      // Remove trailing empty line from textLines if present
      if (textLines[textLines.length - 1] === '') {
        textLines.pop();
      }

      if (textLines.length !== expectedLines) {
        return textLines.map(line => this.escapeHtml(line));
      }

      const htmlLines = [];
      let currentHTML = html;

      for (let i = 0; i < textLines.length; i++) {
        const lineText = textLines.at(i);
        if (i === textLines.length - 1) {
          htmlLines.push(currentHTML);
        } else {
          const lineBreakIndex = currentHTML.indexOf('\n');
          if (lineBreakIndex !== -1) {
            htmlLines.push(currentHTML.substring(0, lineBreakIndex));
            currentHTML = currentHTML.substring(lineBreakIndex + 1);
          } else {
            htmlLines.push(this.escapeHtml(lineText));
          }
        }
      }

      return htmlLines;
    },

    // Helper method to escape HTML
    escapeHtml(text: string): string {
      const div = document.createElement('div');
      div.textContent = text;
      return div.innerHTML;
    },
    parseMarkdown(content: string, filePath: string, source: string): { key: string; line: number; html: string; codeLine?: number }[] {
      const parser = marked;
      if (!katexLoaded && contentHasMath(content)) {
        void loadKatex().then(() => { this.katexReady = true; }).catch(() => { /* logged inside loadKatex above */ });
      }
      // Tag each top level block with its source line for scroll-sync
      let tokens: Token[] | null;
      try {
        tokens = parser.lexer(content);
      } catch (err) {
        console.error("Failed to lex markdown:", err);
        tokens = null;
      }
      if (!tokens) {
        return [{ key: "loading", line: 0, html: DOMPurify.sanitize("Loading...", MD_SANITIZE_CONFIG) }];
      }
      void parser.walkTokens(tokens, (token) => {
        if (token.type === "image" && token.href) {
          token.href = buildPreviewResourceUrl(token.href, filePath, source);
        } else if (token.type === "html" && token.block === false && htmlTagBalance(token.raw) === 0 && /\s(?:src|href|style)=/i.test(token.raw)) {
          const rewritten = rewriteHtmlBlockForMd(token.raw, filePath, source);
          token.raw = rewritten;
          token.text = rewritten;
        }
      });
      // Blocks are keyed off a hash of their own source, so Vue can move it in the DOM
      // instead of rendering it again.
      const keyCounts = new Map<string, number>();
      const nextKey = (raw: string): string => {
        const base = hashText(raw);
        const occurrence = keyCounts.get(base) ?? 0;
        keyCounts.set(base, occurrence + 1);
        return occurrence === 0 ? base : `${base}-${occurrence}`;
      };
      let line = 0;
      const parts: { key: string; line: number; html: string; codeLine?: number }[] = [];
      let group: { needsRewrite: unknown; html: string; raw: string; line: number; depth: number } | null = null;
      for (const token of tokens) {
        let html: string;
        try {
          const single = [token as never] as Token[] & { links?: Record<string, unknown> };
          single.links = (tokens as Token[] & { links?: Record<string, unknown> }).links ?? {};
          html = parser.parser(single as never);
        } catch (_e) {
          html = "";
        }
        const needsRewrite = !!html && (token.type === "html" || /<(?:video|audio|source|track)\b/i.test(token.raw));
        const lineCount = (token.raw.match(/\n/g) || []).length;
        const depth = token.type === "html" ? htmlTagBalance(token.raw) : 0;
        // For code blocks, work out the exact line the code content starts, from the token own raw/text offset,so
        // the scroll anchors can be placed correctly
        let codeLine: number | null = null;
        if (token.type === "code" && typeof (token as { text?: unknown }).text === "string") {
          const codeText = (token as { text: string }).text;
          const offset = token.raw.indexOf(codeText);
          if (offset !== -1) {
            codeLine = line + (token.raw.slice(0, offset).match(/\n/g) || []).length;
          }
        }
        if (group) {
          group.html += html;
          group.raw += token.raw;
          group.depth += depth;
          group.needsRewrite = group.needsRewrite || needsRewrite;
          if (group.depth <= 0) {
            const finalHtml = group.needsRewrite ? rewriteHtmlBlockForMd(group.html, filePath, source) : group.html;
            parts.push({ key: nextKey(group.raw), line: group.line, html: DOMPurify.sanitize(finalHtml, MD_SANITIZE_CONFIG) });
            group = null;
          }
        } else if (depth > 0) {
          group = { html, raw: token.raw, line, depth, needsRewrite };
        } else {
          const finalHtml = needsRewrite ? rewriteHtmlBlockForMd(html, filePath, source) : html;
          parts.push({ key: nextKey(token.raw), line, html: DOMPurify.sanitize(finalHtml, MD_SANITIZE_CONFIG), codeLine: codeLine ?? undefined });
        }
        line += lineCount;
      }
      if (group) {
        // Reached the end with tags still unclosed (maybe malformed HTML), so flush them rather than dropping.
        const finalHtml = group.needsRewrite ? rewriteHtmlBlockForMd(group.html, filePath, source) : group.html;
        parts.push({ key: nextKey(group.raw), line: group.line, html: DOMPurify.sanitize(finalHtml, MD_SANITIZE_CONFIG) });
      }
      return parts;
    },
    updateEditorStats() {
      if (this.splitMode) return;
      const text = this.content.trim();
      const validWord = text.split(/\s+/).filter(t => /[a-zA-Z0-9]/.test(t));
      const words = validWord.length;
      const chars = text.length;
      mutations.setEditorStats({ lines: null, words, chars });
    },
    reinit() {
      mutations.resetEditorScrollRatio(state.req.path);
      mutations.resetSelected();
      mutations.addSelected({
        name: state.req.name,
        path: state.req.path,
        size: state.req.size,
        type: state.req.type,
        source: state.req.source,
        modified: state.req.modified,
        hasPreview: state.req.hasPreview,
      });
      // Set initial content. The `watch` will trigger the first highlight.
      // In split mode, prefer the editor live buffer over the file.
      const fileContent = state.req.content === "empty-file-x6OlSil" ? "" : state.req.content || "";
      const newContent = (this.splitMode && this.liveContent !== null) ? this.liveContent : fileContent;
      if (newContent === this.content) {
        this.scrollGuard.suppress();
        this.finalizeContentRender(state.editor.scrollRatio);
      } else {
        this.isLoadingNewContent = true;
        this.content = newContent;
      }
      this.updateEditorStats();
    },
    finalizeContentRender(target: number) {
      this.$nextTick(async () => {
        try {
          await this.applyHighlighting();
        } catch (err) {
          console.error("Failed to apply syntax highlighting:", err);
        }
        if (!this.isHtml) this.applyScrollRatio(target);
      });
    },
    attachScrollListener(el: HTMLElement | null) {
      if (this.boundScrollEl === el) return;
      this.boundScrollEl?.removeEventListener("scroll", this.handleScroll);
      this.boundScrollEl = el;
      el?.addEventListener("scroll", this.handleScroll, { passive: true });
    },
    getScrollContainer(): HTMLElement | null {
      return this.splitMode
        ? (this.scrollTarget as HTMLElement | null) || (this.$refs.scrollContainer as HTMLElement | null)
        : document.getElementById("main");
    },
    getLineAnchors(): { line: number; top: number }[] {
      const viewer = this.$refs.viewer as HTMLElement | null;
      const container = this.getScrollContainer();
      if (!viewer || !container) return [];
      const containerTop = container.getBoundingClientRect().top - container.scrollTop;
      const topOf = (el: HTMLElement) => el.getBoundingClientRect().top - containerTop;
      const anchors = Array.from(viewer.querySelectorAll<HTMLElement>(".md-block")).map((el) => ({
        line: Number(el.dataset.line),
        top: topOf(el),
      }));
      // per-line anchors to keep the interpolation
      viewer.querySelectorAll<HTMLElement>(".code-line[data-source-line]").forEach((el) => {
        anchors.push({ line: Number(el.dataset.sourceLine), top: topOf(el) });
      });
      anchors.sort((a, b) => a.line - b.line);
      return anchors;
    },
    totalLines(): number {
      return Math.max(0, this.content.split('\n').length - 1);
    },
    // Finds the pair of adjacent anchors 'value' along whatever axis 'getValue' reads off each anchor (top or line).
    bracketAnchors(
      anchors: { line: number; top: number }[],
      getValue: (anchor: { line: number; top: number }) => number,
      value: number,
    ): [{ line: number; top: number }, { line: number; top: number }] {
      for (let i = 0; i < anchors.length - 1; i++) {
        if (getValue(anchors.at(i)) <= value && getValue(anchors.at(i + 1)) > value) {
          return [anchors.at(i), anchors.at(i + 1)];
        }
      }
      return [anchors.at(0), anchors.at(-1)];
    },
    // The line currently at the top of the viewport by interpolating between the near block anchors.
    currentLine() {
      const el = this.getScrollContainer();
      if (!el) return 0;
      const anchors = this.getLineAnchors();
      if (!anchors.length) return 0;
      const scrollTop = el.scrollTop;
      const maxScrollTop = el.scrollHeight - el.clientHeight;
      if (maxScrollTop > 0 && scrollTop >= maxScrollTop - 1) {
        return this.totalLines();
      }
      const [a, b] = this.bracketAnchors(anchors, (anchor) => anchor.top, scrollTop);
      const topSpan = b.top - a.top;
      const frac = topSpan > 0 ? Math.min(1, Math.max(0, (scrollTop - a.top) / topSpan)) : 0;
      return a.line + frac * (b.line - a.line);
    },
    syncScrollRatio() {
      mutations.setEditorScrollRatio(this.currentLine(), "viewer");
    },
    handleScroll() {
      if (this.isHtml) return;
      this.scrollGuard.schedule(() => this.syncScrollRatio());
    },
    applyScrollRatio(line: number) {
      const el = this.getScrollContainer();
      if (!el) return;
      const anchors = this.getLineAnchors();
      if (!anchors.length) return;
      const first = anchors.at(0);
      let top;
      if (line <= first.line) {
        top = 0;
      } else if (line >= this.totalLines()) {
        top = el.scrollHeight - el.clientHeight;
      } else {
        const [a, b] = this.bracketAnchors(anchors, (anchor) => anchor.line, line);
        const lineSpan = b.line - a.line;
        top = lineSpan > 0
          ? a.top + ((line - a.line) / lineSpan) * (b.top - a.top)
          : a.top;
      }
      this.scrollGuard.applyRemote(() => { el.scrollTop = top; });
    },
  },
  watch: {
    // We now watch the `content` property.
    content() {
      const target = this.isLoadingNewContent ? state.editor.scrollRatio : this.currentLine();
      this.isLoadingNewContent = false;
      this.scrollGuard.suppress();
      this.finalizeContentRender(target);
      this.updateEditorStats();
    },
    // Watch for changes in state.req.content and update local content
    req() {
      this.reinit()
    },
    darkMode() {
      const viewer = this.$refs.viewer as HTMLElement | null;
      if (viewer?.querySelector('pre code')) {
        void this.setHighlightTheme(getters.isDarkMode());
      }
    },
    editorScrollRatio() {
      if (this.isHtml || state.editor.scrollSource === 'viewer') return;
      this.applyScrollRatio(state.editor.scrollRatio);
    },
    liveContent(newVal) {
      if (!this.splitMode || newVal === null) return;
      if (this.content === "") this.isLoadingNewContent = true;
      this.content = newVal;
    },
    scrollTarget(newEl) {
      this.attachScrollListener(newEl);
      const container = this.getScrollContainer();
      this.attachScrollListener(container);
      if (container && !this.isHtml) {
        this.applyScrollRatio(state.editor.scrollRatio);
      }
    },
    isHtml(newVal) {
      this.$nextTick(() => {
        if (newVal) this.observeHtmlResize();
        else this.unobserveHtmlResize();
      });
    },
  },
  computed: {
    req() {
      return state.req;
    },
    darkMode() {
      // This computed property returns the current dark mode state.
      return getters.isDarkMode();
    },
    isHtml() {
      return isHtmlMimeType(state.req.type);
    },
    htmlPreview() {
      if (!this.isHtml) {
        return { srcdoc: "" };
      }
      return buildHtmlPreview(this.content, state.req.path, state.req.source);
    },
    renderedContent() {
      void this.katexReady;
      return this.parseMarkdown(this.content, state.req.path, state.req.source);
    },
    spaceForStatusBar() {
      return state.isMobile ? 3.1 : 3.5;
    },
    editorScrollRatio() {
      return state.editor.scrollRatio;
    },
  },
  mounted() {
    this.reinit();
    this.$nextTick(() => {
      this.attachScrollListener(this.getScrollContainer());
    });
    if (this.isHtml) this.observeHtmlResize();
  },
  beforeUnmount() {
    if (this.scrollGuard.cancel()) {
      this.syncScrollRatio();
    }
  },
  unmounted() {
    this.attachScrollListener(null);
    this.unobserveHtmlResize();

    if (!this.splitMode) {
      mutations.setEditorStats({ lines: 0, words: 0, chars: 0 });
    }
  }
};
</script>

<style>
#markedown-viewer {
  margin: 1em;
  overflow-wrap: break-word;
  word-break: break-word;
}

#markedown-viewer .markdown-content-container {
  background-color: var(--surfacePrimary);
  border-radius: 1em;
  padding: 1em;
}

#markedown-viewer .markdown-content,
#markedown-viewer .html-content {
  width: 100%;
}

#markedown-viewer .html-content {
  box-sizing: border-box;
  border: none;
  min-height: 24em;
  background: #fff;
  color-scheme: light dark;
}

#markedown-viewer .html-content img {
  max-width: 100%;
  height: auto;
}

#markedown-viewer .spacer {
  width: 100%;
  pointer-events: none;
}

/* Code block wrapper with line numbers */
#markedown-viewer .markdown-content-container .code-block-wrapper {
  display: flex;
  background-color: #f6f8fa;
  border-radius: 0.5em;
  overflow: hidden;
  margin: 1em 0;
  font-family: 'SFMono-Regular', 'Monaco', 'Inconsolata', 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.85em;
  line-height: 1.45;
  max-width: 100%;
  position: relative;
}

#markedown-viewer .markdown-content code:not(pre code) {
  background-color: #f6f8fa;
  padding: 0.25em 0.4em;
  border-radius: 0.5em;
  font-family: 'SFMono-Regular', 'Monaco', 'Inconsolata', 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.85em;
}

#markedown-viewer .markdown-content-container.dark-mode code:not(pre code),
#markedown-viewer .markdown-content-container.dark-mode .code-block-wrapper {
  background-color: #0d1117;
}

/* keybinds like <kbd>Ctrl</kbd> */
#markedown-viewer .markdown-content kbd {
  background-color: var(--background);
  border: 1px solid var(--divider);
  border-radius: 0.35em;
  padding: 0.1em 0.5em;
  font-family: 'SFMono-Regular', 'Monaco', 'Inconsolata', 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.85em;
  box-shadow: inset 0 -2px 0 var(--divider);
}

#markedown-viewer .markdown-content-container .copy-code-button {
  position: absolute;
  top: 0.4em;
  right: 0.3em;
  border-radius: 0.45em;
  color: var(--primaryColor);
  background: var(--background);
  font-size: 0.8em;
  padding: 0.2em 0.4em;
  transition: background 0.2s, color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Line numbers styling */
#markedown-viewer .markdown-content-container .line-numbers {
  -webkit-touch-callout: none;
  -webkit-user-select: none;
  user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  background-color: var(--background);
  border-right: 1px solid var(--divider);
  padding: 0.625em 0.5em 0.625em 0.75em;
  text-align: right;
  color: #7d8590;
  min-width: 2em;
  flex-shrink: 0;
}

#markedown-viewer .markdown-content-container .line-number {
  display: block;
  white-space: nowrap;
  height: 1.45em;
  line-height: 1.45;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

#markedown-viewer .markdown-content-container .line-number:hover {
  background-color: transparent;
  color: var(--primaryColor);
}

#markedown-viewer .markdown-content-container .line-number.active {
  background-color: color-mix(in srgb, var(--primaryColor) 30%, transparent);
  color: var(--primaryColor);
  border-radius: 0.25em;
}

/* Individual code lines */
#markedown-viewer .markdown-content-container .code-line {
  display: block;
  white-space: pre;
  line-height: 1.45;
  min-height: 1.45em;
  transition: background-color 0.2s ease;
}

#markedown-viewer .markdown-content-container .code-line.highlighted {
  background-color: color-mix(in srgb, var(--primaryColor) 20%, transparent);
}

/* Code content styling */
#markedown-viewer .markdown-content-container .code-content {
  flex: 1;
  overflow-x: auto;
  max-width: 100%;
}

#markedown-viewer .markdown-content-container .code-content pre {
  margin: 0;
  border-radius: 0;
  padding: 0;
  line-height: 1.45;
  width: 100%;
}

#markedown-viewer .markdown-content-container .code-content code {
  padding: 0.5em;
  padding-top: 0.65em;
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
  display: block;
  white-space: pre;
}

/* Fix for code content line height to match line numbers exactly */
#markedown-viewer .markdown-content-container .code-content pre code {
  line-height: 1.45;
}

/* Ensure each line in the code has the same height as line numbers */
#markedown-viewer .markdown-content-container .code-content pre code br {
  line-height: 1.45;
}

#markedown-viewer .markdown-content-container .code-content a {
  color: var(--primaryColor);
  font-weight: 500;
}

#markedown-viewer .markdown-content-container .code-content a:hover {
  text-decoration: underline;
}

#markedown-viewer .markdown-content ul,
#markedown-viewer .markdown-content ol {
  padding-left: 2em; /* base indent for first lvl */
  margin: 0.1em 0;
}

#markedown-viewer .markdown-content ul ul,
#markedown-viewer .markdown-content ul ol,
#markedown-viewer .markdown-content ol ul,
#markedown-viewer .markdown-content ol ol {
  padding-left: 2em; /* indent for nested lvls */
}

/* line height for list items and any paragraphs inside the nested lvls */
#markedown-viewer .markdown-content li,
#markedown-viewer .markdown-content li p {
  line-height: 1.65;
  margin-top: 0;
  margin-bottom: 0;
}

#markedown-viewer .markdown-content img {
  max-width: 100%;
  height: auto;
}

/* Task list checkboxes */
#markedown-viewer .markdown-content li:has(input[type="checkbox"]) {
  list-style: none;
  margin-left: -1.2em;
}

#markedown-viewer .markdown-content input[type="checkbox"] {
  appearance: none;
  width: 1em;
  height: 1em;
  margin-right: 0.5em;
  border: 1.5px solid var(--divider);
  border-radius: 0.25em;
  vertical-align: middle;
  position: relative;
}

#markedown-viewer .markdown-content input[type="checkbox"]:checked {
  background-color: var(--primaryColor);
  border-color: var(--primaryColor);
}

#markedown-viewer .markdown-content input[type="checkbox"]:checked::after {
  content: "";
  position: absolute;
  left: 0.28em;
  top: 0.04em;
  width: 0.28em;
  height: 0.5em;
  border: solid white;
  border-width: 0 0.15em 0.15em 0;
  transform: rotate(45deg);
}

#markedown-viewer .markdown-content li:has(input[type="checkbox"]:checked) {
  color: color-mix(in srgb, var(--textPrimary) 55%, transparent);
  text-decoration: line-through;
}

/* Links */
#markedown-viewer .markdown-content a {
  color: var(--primaryColor);
}

#markedown-viewer .markdown-content a:hover {
  text-decoration: underline;
}

/* Tables */
#markedown-viewer .markdown-content table {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
  overflow-x: auto;
  display: block;
}

#markedown-viewer .markdown-content th,
#markedown-viewer .markdown-content td {
  border: 2px solid var(--background);
  padding: 0.4em 0.8em;
}

#markedown-viewer .markdown-content th {
  font-weight: 600;
  background-color: var(--background);
}

#markedown-viewer .markdown-content tbody tr:nth-child(even) {
  background-color: color-mix(in srgb, var(--background) 40%, transparent);
}

/* Blockquotes */
#markedown-viewer .markdown-content blockquote {
  margin: 1em 0;
  padding: 0.1em 1em;
  border-left: 0.25em solid var(--primaryColor);
  background-color: color-mix(in srgb, var(--primaryColor) 8%, transparent);
  color: color-mix(in srgb, var(--textPrimary) 75%, transparent);
  border-radius: 0 0.5em 0.5em 0;
}

#markedown-viewer .markdown-content blockquote blockquote {
  border-left-color: color-mix(in srgb, var(--primaryColor) 50%, transparent);
  background-color: transparent;
}

#markedown-viewer .markdown-content blockquote p {
  margin: 0.5em 0;
}

#markedown-viewer .markdown-content blockquote > :first-child {
  margin-top: 0;
}

#markedown-viewer .markdown-content blockquote > :last-child {
  margin-bottom: 0;
}

/* mark (highlight) tags */
#markedown-viewer .markdown-content mark {
  background-color: var(--primaryColor);
  border-radius: 2px;
  padding: 0 0.2em;
}

</style>
