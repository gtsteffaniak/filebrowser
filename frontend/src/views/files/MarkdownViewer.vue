<template>
  <div id="markedown-viewer">
    <div class="markdown-content-container" :class="{ 'dark-mode': darkMode }">
      <div ref="viewer" v-html="renderedContent" class="markdown-content"></div>
    </div>
    <div class="spacer" :style="{ height: spaceForStatusBar + 'em' }"></div>
  </div>
</template>

<script lang="ts">
import { marked } from "marked";
import DOMPurify from 'dompurify';
import { state, mutations, getters } from "@/store";
import hljs from 'highlight.js';
import { notify } from "@/notify";

export default {
  name: "markdownViewer",
  data() {
    return {
      content: "",
    };
  },
  methods: {
    // This theme switcher logic is correct and remains.
    setHighlightTheme(isDark: boolean) {
      const THEME_LINK_ID = 'highlight-theme-link';
      const lightTheme = 'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css';
      const darkTheme = 'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css';
      const themeUrl = isDark ? darkTheme : lightTheme;

      let link = document.getElementById(THEME_LINK_ID) as HTMLLinkElement;
      if (link) {
        link.href = themeUrl;
      } else {
        link = document.createElement('link');
        link.id = THEME_LINK_ID;
        link.rel = 'stylesheet';
        link.href = themeUrl;
        document.head.appendChild(link);
      }
    },
    // NEW METHOD: Finds and highlights all code blocks and adds line numbers
    applyHighlighting() {
      const viewer = this.$refs.viewer as HTMLElement;
      if (viewer) {
        // This tells highlight.js to find and style every code block.
        viewer.querySelectorAll('pre code').forEach((block) => {
          const codeBlock = block as HTMLElement;
          const langClass = codeBlock.className.split(/\s+/).find(c => c.startsWith('language-'));
          let lang = langClass ? langClass.split('-')[1] : null;

          if (lang && hljs.getLanguage(lang)) {
            hljs.highlightElement(codeBlock);
          } else {
            const text = codeBlock.textContent;
            const result = hljs.highlightAuto(text);
            codeBlock.innerHTML = result.value;
            codeBlock.classList.add('hljs');
          }
          // Add line numbers manually after highlighting
          this.addLineNumbers(codeBlock);
        });
      }
    },
    // Manual line numbers implementation
    addLineNumbers(codeBlock: HTMLElement) {
      const code = codeBlock.textContent || '';
      let lines = code.split('\n');

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
      copyButton.addEventListener('click', async (e) => {
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
        try {
          if (navigator.clipboard && navigator.clipboard.writeText) {
            // Using clipboard API
            navigator.clipboard.writeText(text).then(() => {
              showFeedback(true);
              notify.showSuccessToast(this.$t('buttons.copySuccess'));
            }).catch((err) => {
              console.error('Clipboard API error:', err);
              showFeedback(false);
              notify.showErrorToast(this.$t('tools.materialIconPicker.copyFailed'));
            });
          } else {
            // Fallback using execCommand.
            // This seems the only way to allow copy from http (insecure) connections (even if is marked as deprecated) 
            const textarea = document.createElement('textarea');
            textarea.value = text;
            document.body.appendChild(textarea);
            textarea.select();
            const success = document.execCommand('copy');
            document.body.removeChild(textarea);
            if (success) {
              showFeedback(true);
              notify.showSuccessToast(this.$t('buttons.copySuccess'));
            } else {
              showFeedback(false);
              notify.showErrorToast(this.$t('tools.materialIconPicker.copyFailed'));
            }
          }
        } catch (err) {
          console.error('Copy failed:', err);
          showFeedback(false);
          notify.showErrorToast(this.$t('tools.materialIconPicker.copyFailed'));
        }
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

      // Create code lines with preserved highlighting
      const codeLines = htmlLines.map((lineHTML, index) => {
        const lineElement = document.createElement('div');
        lineElement.className = 'code-line';
        lineElement.setAttribute('data-line', (index + 1).toString());
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
      let textLines = textContent.split('\n');

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
        const lineText = textLines[i];
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
    updateEditorStats() {
      const text = this.content.trim();
      const validWord = text.split(/\s+/).filter(t => /[a-zA-Z0-9]/.test(t));
      const words = validWord.length;
      const chars = text.length;
      mutations.setEditorStats({ lines: null, words, chars });
    },
    reinit() {
      mutations.resetSelected();
      mutations.addSelected({
        name: state.req.name,
        path: state.req.path,
        size: state.req.size,
        type: state.req.type,
        source: state.req.source,
      });
      this.setHighlightTheme(getters.isDarkMode());
      // Set initial content. The `watch` will trigger the first highlight.
      const fileContent = state.req.content == "empty-file-x6OlSil" ? "" : state.req.content || "";
      this.content = fileContent;
      this.updateEditorStats();
    },
  },
  watch: {
    // We now watch the `content` property.
    content() {
      // When the content changes, Vue updates the DOM. We use `nextTick`
      // to wait for that update to finish before applying highlighting.
      this.$nextTick(() => {
        this.applyHighlighting();
      });
      this.updateEditorStats();
    },
    // Watch for changes in state.req.content and update local content
    req() {
      this.reinit()
    },
    darkMode() {
      this.setHighlightTheme(getters.isDarkMode());
    }
  },
  computed: {
    req() {
      return state.req;
    },
    darkMode() {
      // This computed property returns the current dark mode state.
      return state.user.darkMode;
    },
    renderedContent() {
      // We now let marked run with its default, reliable settings.
      // It will correctly render tables and create basic code blocks.
      const markedResult = marked(this.content, { gfm: true });
      // Handle both string and Promise return types
      if (typeof markedResult === 'string') {
        return DOMPurify.sanitize(markedResult);
      } else {
        // If it's a Promise, we need to handle it differently
        return DOMPurify.sanitize('Loading...');
      }
    },
    spaceForStatusBar() {
      return state.isMobile ? 3.1 : 3.5;
    },
  },
  mounted() {
    this.reinit();
  },
  unmounted() {
    // Cleanup logic is correct and remains.
    const link = document.getElementById('highlight-theme-link');
    if (link) {
      document.head.removeChild(link);
    }
    mutations.setEditorStats({ lines: 0, words: 0, chars: 0 });
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
  background-color: var(--alt-background);
  border-radius: 1em;
  padding: 1em;
}

#markedown-viewer .markdown-content {
  width: 100%;
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
  background-color: #161b22;
}

#markedown-viewer .markdown-content-container .copy-code-button {
  position: absolute;
  top: 0.4em;
  right: 0.3em;
  border-radius: 0.45em;
  color: var(--primaryColor);
  background: var(--alt-background);
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
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  background-color: #f1f3f4;
  border-right: 1px solid #d0d7de;
  padding: 0.625em 0.5em 0.625em 0.75em;
  text-align: right;
  color: #656d76;
  min-width: 2.5em;
  flex-shrink: 0;
}

#markedown-viewer .markdown-content-container.dark-mode .line-numbers {
  background-color: #21262d;
  border-right-color: #30363d;
  color: #7d8590;
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
  background-color: #e1e4e8;
  color: #24292e;
}

#markedown-viewer .markdown-content-container.dark-mode .line-number:hover {
  background-color: #30363d;
  color: #f0f6fc;
}

#markedown-viewer .markdown-content-container .line-number.active {
  background-color: #0366d6;
  color: white;
}

#markedown-viewer .markdown-content-container.dark-mode .line-number.active {
  background-color: #1f6feb;
  color: white;
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
  background-color: #fff8c5;
}

#markedown-viewer .markdown-content-container.dark-mode .code-line.highlighted {
  background-color: #ffd33d20;
}

/* Code content styling */
#markedown-viewer .markdown-content-container .code-content {
  flex: 1;
  overflow-x: auto;
  max-width: 100%;
}

#markedown-viewer .markdown-content-container .code-content pre {
  margin: 0;
  background: transparent;
  border-radius: 0;
  padding: 0;
  line-height: 1.45;
  width: 100%;
}

#markedown-viewer .markdown-content-container .code-content code {
  background: transparent;
  padding: 0.5em;
  padding-top: 0.75em;
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
  color: #3737c9;
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

</style>