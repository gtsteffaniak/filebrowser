<template>
  <details
    class="directory-readme"
    aria-label="Directory README"
    open
    @click.stop
    @contextmenu.stop
    @dblclick.stop
    @keydown.stop
    @mousedown.stop="handleInteraction"
    @touchend.stop
    @touchstart.stop="handleInteraction"
  >
    <summary class="directory-readme-header">
      <div class="directory-readme-title">
        <span class="material-symbols-outlined">{{ titleIcon }}</span>
        <span>README.md</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </div>
      <span class="directory-readme-chevron material-symbols-outlined">{{ chevronIcon }}</span>
    </summary>
    <div class="directory-readme-body">
      <div
        class="directory-readme-content"
        :class="{ 'dark-mode': darkMode }"
        v-html="renderedContent"
      ></div>
    </div>
  </details>
</template>

<script lang="ts">
import { renderMarkdown } from "@/utils/markdown";

export default {
  name: "DirectoryReadme",
  props: {
    content: {
      type: String,
      required: true,
    },
    filePath: {
      type: String,
      required: true,
    },
    source: {
      type: String,
      required: true,
    },
    darkMode: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    titleIcon() {
      return "markdown";
    },
    chevronIcon() {
      return "expand_more";
    },
    renderedContent() {
      return renderMarkdown(this.content, this.filePath, this.source, true);
    },
  },
  methods: {
    handleInteraction() {
      this.$emit("interact");
    },
  },
};
</script>

<style scoped>
.directory-readme {
  box-sizing: border-box;
  width: calc(100% - 2em);
  margin: 0 1em 1.25em;
  overflow: hidden;
  user-select: text;
  background: var(--alt-background);
  border: 1px solid color-mix(in srgb, var(--textPrimary) 16%, transparent);
  border-left: 0.2em solid var(--primaryColor);
  border-radius: 0.65em;
}

.directory-readme-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 2.6em;
  padding: 0.2em 0.35em 0.2em 0.8em;
  cursor: pointer;
  list-style: none;
  user-select: none;
  transition: background-color 150ms ease;
}

.directory-readme-header::-webkit-details-marker {
  display: none;
}

.directory-readme[open] .directory-readme-header {
  border-bottom: 1px solid color-mix(in srgb, var(--textPrimary) 10%, transparent);
}

.directory-readme-header:hover {
  background: color-mix(in srgb, var(--primaryColor) 5%, transparent);
}

.directory-readme-header:focus-visible {
  outline: 2px solid var(--primaryColor);
  outline-offset: -2px;
}

.directory-readme-title {
  display: flex;
  gap: 0.5em;
  align-items: center;
  color: var(--textPrimary);
  font-family: 'SFMono-Regular', 'Monaco', 'Inconsolata', 'Liberation Mono', monospace;
  font-size: 0.82em;
  font-weight: 500;
}

.directory-readme-title .material-symbols-outlined {
  color: var(--primaryColor);
  font-size: 1.25em;
}

.directory-readme-chevron {
  margin-left: auto;
  color: var(--textPrimary);
  font-size: 1.4em;
  transition: color 150ms ease, transform 180ms ease;
}

.directory-readme[open] .directory-readme-chevron {
  color: var(--primaryColor);
  transform: rotate(180deg);
}

.directory-readme-body {
  min-height: 0;
  overflow: hidden;
}

.directory-readme-content {
  box-sizing: border-box;
  width: 100%;
  padding: 1em 1.25em;
  overflow-wrap: break-word;
  word-break: break-word;
}

.directory-readme-content :deep(> :first-child) {
  margin-top: 0;
}

.directory-readme-content :deep(> :last-child) {
  margin-bottom: 0;
}

.directory-readme-content :deep(img) {
  display: block;
  max-width: 100%;
  max-height: 32rem;
  height: auto;
  margin: 1em 0;
  object-fit: contain;
  border: 1px solid color-mix(in srgb, var(--textPrimary) 10%, transparent);
  border-radius: 0.5em;
}

.directory-readme-content :deep(a) {
  color: var(--primaryColor);
  text-underline-offset: 0.15em;
}

.directory-readme-content :deep(blockquote) {
  margin-left: 0;
  padding-left: 1em;
  color: color-mix(in srgb, var(--textPrimary) 72%, transparent);
  border-left: 0.2em solid color-mix(in srgb, var(--primaryColor) 55%, transparent);
}

.directory-readme-content :deep(pre) {
  max-width: 100%;
  padding: 0.8em;
  overflow-x: auto;
  background: color-mix(in srgb, var(--background) 85%, transparent);
  border-radius: 0.5em;
}

.directory-readme-content :deep(code:not(pre code)) {
  padding: 0.2em 0.35em;
  background: color-mix(in srgb, var(--background) 85%, transparent);
  border-radius: 0.35em;
}

@media (max-width: 768px) {
  .directory-readme {
    width: calc(100% - 1em);
    margin: 0 0.5em 1em;
  }

  .directory-readme-content {
    padding: 0.8em 1em;
  }

  .directory-readme-content :deep(img) {
    max-height: 22rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .directory-readme-chevron,
  .directory-readme-header {
    transition: none;
  }
}
</style>
