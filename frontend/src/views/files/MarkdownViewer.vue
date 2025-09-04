<template>
  <div id="markedown-viewer" ref="viewer" :class="{ 'dark-mode': darkMode }" v-html="renderedContent"></div>
</template>

<script lang="ts">
import { marked } from "marked";
import DOMPurify from 'dompurify';
import { state, mutations, getters } from "@/store";
import hljs from 'highlight.js';

// --- We have removed all `marked.use()` configuration ---
// This allows marked to function with its robust defaults.

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
          hljs.highlightElement(block as HTMLElement);

          // Add line numbers manually after highlighting
          this.addLineNumbers(block as HTMLElement);
        });
      }
    },
    // Manual line numbers implementation
    addLineNumbers(codeBlock: HTMLElement) {
      const code = codeBlock.textContent || '';
      const lines = code.split('\n');

      // Don't add line numbers if there's only one line or if already added
      if (lines.length <= 1 || codeBlock.classList.contains('line-numbers-added')) {
        return;
      }

      // Create a wrapper div
      const wrapper = document.createElement('div');
      wrapper.className = 'code-block-wrapper';

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
      // Create a temporary element to parse the HTML
      const temp = document.createElement('div');
      temp.innerHTML = html;

      // Get the text content to count actual line breaks
      const textContent = temp.textContent || '';
      const textLines = textContent.split('\n');

      // If there's a mismatch in line counts, fall back to simple split
      if (textLines.length !== expectedLines) {
        return textLines.map(line => this.escapeHtml(line));
      }

      // Try to split the HTML while preserving tags
      const htmlLines: string[] = [];
      let currentHTML = html;
      //let currentLineIndex = 0;

      // For each line, try to extract the corresponding HTML
      for (let i = 0; i < textLines.length; i++) {
        const lineText = textLines[i];

        if (i === textLines.length - 1) {
          // Last line - take remaining HTML
          htmlLines.push(currentHTML);
        } else {
          // Find the line break in the HTML and split there
          const lineBreakIndex = currentHTML.indexOf('\n');
          if (lineBreakIndex !== -1) {
            htmlLines.push(currentHTML.substring(0, lineBreakIndex));
            currentHTML = currentHTML.substring(lineBreakIndex + 1);
          } else {
            // Fallback: escape the plain text
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
    }
  },
  watch: {
    // We now watch the `content` property.
    content() {
      // When the content changes, Vue updates the DOM. We use `nextTick`
      // to wait for that update to finish before applying highlighting.
      this.$nextTick(() => {
        this.applyHighlighting();
      });
    },
    darkMode() {
      this.setHighlightTheme(getters.isDarkMode());
    }
  },
  computed: {
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
  },
  mounted() {
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
  },
  unmounted() {
    // Cleanup logic is correct and remains.
    const link = document.getElementById('highlight-theme-link');
    if (link) {
      document.head.removeChild(link);
    }
  }
};
</script>

<style>
/* This style block is now plain CSS, no "lang=scss" needed */
#markedown-viewer {
  margin: 1em;
  padding: 1em;
  background-color: var(--alt-background);
  border-radius: 1em;
}

#markedown-viewer pre {
  border-radius: 8px;
}

/* Code block wrapper with line numbers */
#markedown-viewer .code-block-wrapper {
  display: flex;
  background-color: #f6f8fa;
  border-radius: 8px;
  overflow: hidden;
  margin: 16px 0;
  font-family: 'SFMono-Regular', 'Monaco', 'Inconsolata', 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.85em;
  line-height: 1.45;
}

#markedown-viewer.dark-mode .code-block-wrapper {
  background-color: #161b22;
}

/* Line numbers styling */
#markedown-viewer .line-numbers {
  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  background-color: #f1f3f4;
  border-right: 1px solid #d0d7de;
  padding: 10px 8px 10px 12px;
  text-align: right;
  color: #656d76;
  min-width: 40px;
  flex-shrink: 0;
}

#markedown-viewer.dark-mode .line-numbers {
  background-color: #21262d;
  border-right-color: #30363d;
  color: #7d8590;
}

#markedown-viewer .line-number {
  display: block;
  white-space: nowrap;
  height: 1.45em;
  line-height: 1.45;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

#markedown-viewer .line-number:hover {
  background-color: #e1e4e8;
  color: #24292e;
}

#markedown-viewer.dark-mode .line-number:hover {
  background-color: #30363d;
  color: #f0f6fc;
}

#markedown-viewer .line-number.active {
  background-color: #0366d6;
  color: white;
}

#markedown-viewer.dark-mode .line-number.active {
  background-color: #1f6feb;
  color: white;
}

/* Individual code lines */
#markedown-viewer .code-line {
  display: block;
  white-space: pre;
  line-height: 1.45;
  min-height: 1.45em;
  transition: background-color 0.2s ease;
}

#markedown-viewer .code-line.highlighted {
  background-color: #fff8c5;
}

#markedown-viewer.dark-mode .code-line.highlighted {
  background-color: #ffd33d20;
}

/* Code content styling */
#markedown-viewer .code-content {
  flex: 1;
  overflow-x: auto;
}

#markedown-viewer .code-content pre {
  margin: 0;
  background: transparent !important;
  border-radius: 0;
  padding: 0;
  line-height: 1.45;
}

#markedown-viewer .code-content code {
  background: transparent !important;
  padding: 0.5em;
  padding-top: 0.75em;
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
  display: block;
  white-space: pre;
}

/* Fix for code content line height to match line numbers exactly */
#markedown-viewer .code-content pre code {
  line-height: 1.45;
}

/* Ensure each line in the code has the same height as line numbers */
#markedown-viewer .code-content pre code br {
  line-height: 1.45;
}
</style>