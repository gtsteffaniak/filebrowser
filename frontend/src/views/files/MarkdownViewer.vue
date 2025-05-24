<template>
  <div id="markedown-viewer" ref="viewer" v-html="renderedContent"></div>
</template>

<script lang="ts">
import { marked } from "marked";
import DOMPurify from 'dompurify';
import { state } from "@/store";
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
    // NEW METHOD: Finds and highlights all code blocks in the rendered content.
    applyHighlighting() {
      const viewer = this.$refs.viewer as HTMLElement;
      if (viewer) {
        // This tells highlight.js to find and style every code block.
        viewer.querySelectorAll('pre code').forEach((block) => {
          hljs.highlightElement(block as HTMLElement);
        });
      }
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
      this.setHighlightTheme(state.user.darkMode);
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
      return DOMPurify.sanitize(marked(this.content, { gfm: true }));
    },
  },
  mounted() {
    this.setHighlightTheme(state.user.darkMode);
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
</style>