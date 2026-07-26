<template>
  <section
    class="directory-readme"
    aria-label="Directory README"
    tabindex="0"
    @click.stop
    @contextmenu.stop
    @dblclick.stop
    @keydown.stop
    @mousedown.stop="handleInteraction"
    @touchend.stop
    @touchstart.stop="handleInteraction"
  >
    <div
      class="directory-readme-content"
      :class="{ 'dark-mode': darkMode }"
      v-html="renderedContent"
    ></div>
  </section>
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
    renderedContent() {
      return renderMarkdown(this.content, this.filePath, this.source, true);
    },
  },
  methods: {
    handleInteraction(event: Event) {
      if (event.currentTarget instanceof HTMLElement) {
        event.currentTarget.focus({ preventScroll: true });
      }
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
  user-select: text;
}

.directory-readme-content {
  box-sizing: border-box;
  width: 100%;
  padding: 1em 1.25em;
  overflow-wrap: break-word;
  word-break: break-word;
  background: var(--alt-background);
  border: 1px solid color-mix(in srgb, var(--textPrimary) 12%, transparent);
  border-radius: 1em;
}

.directory-readme-content :deep(> :first-child) {
  margin-top: 0;
}

.directory-readme-content :deep(> :last-child) {
  margin-bottom: 0;
}

.directory-readme-content :deep(img) {
  max-width: 100%;
  height: auto;
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
}
</style>
