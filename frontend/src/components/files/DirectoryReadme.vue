<template>
  <section
    class="directory-readme"
    aria-label="Directory README"
    @click.stop
    @contextmenu.stop
    @dblclick.stop
    @keydown.stop
    @mousedown.stop="handleInteraction"
    @touchend.stop
    @touchstart.stop="handleInteraction"
  >
    <header class="directory-readme-header">
      <span class="directory-readme-title">
        <span class="material-symbols-outlined" aria-hidden="true">{{ titleIcon }}</span>
        <span>README.md</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </span>
    </header>
    <div
      ref="body"
      class="directory-readme-body"
      :class="{ 'is-collapsed': !expanded, 'has-overflow': isOverflowing }"
    >
      <div
        ref="content"
        class="directory-readme-content"
        :class="{ 'dark-mode': darkMode }"
        v-html="renderedContent"
      ></div>
    </div>
    <button
      v-if="isOverflowing"
      type="button"
      class="directory-readme-toggle"
      :aria-expanded="expanded"
      @click="toggleExpanded"
    >
      <span>{{ toggleLabel }}</span>
      <span class="material-symbols-outlined" aria-hidden="true">{{ toggleIcon }}</span>
    </button>
  </section>
</template>

<script lang="ts">
import { renderMarkdown } from "@/utils/markdown";

export default {
  name: "DirectoryReadme",
  data() {
    return {
      expanded: false,
      isOverflowing: false,
      resizeObserver: null as ResizeObserver | null,
    };
  },
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
    toggleIcon() {
      return this.expanded ? "expand_less" : "expand_more";
    },
    toggleLabel() {
      return this.expanded ? "Show less" : "Show full README";
    },
    renderedContent() {
      return renderMarkdown(this.content, this.filePath, this.source, true);
    },
  },
  watch: {
    renderedContent() {
      this.expanded = false;
      this.$nextTick(this.checkOverflow);
    },
  },
  mounted() {
    this.resizeObserver = new ResizeObserver(() => {
      if (!this.expanded) this.checkOverflow();
    });
    this.resizeObserver.observe(this.$refs.content as Element);
    this.$nextTick(this.checkOverflow);
  },
  beforeUnmount() {
    this.resizeObserver?.disconnect();
  },
  methods: {
    handleInteraction() {
      this.$emit("interact");
    },
    checkOverflow() {
      const body = this.$refs.body as HTMLElement | undefined;
      if (body) this.isOverflowing = body.scrollHeight > body.clientHeight + 1;
    },
    toggleExpanded() {
      this.expanded = !this.expanded;
      if (!this.expanded) this.$nextTick(this.checkOverflow);
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
  border-radius: 0.65em;
}

.directory-readme-header {
  display: flex;
  align-items: center;
  min-height: 2.6em;
  padding: 0.2em 0.8em;
  user-select: none;
  border-bottom: 1px solid color-mix(in srgb, var(--textPrimary) 10%, transparent);
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

.directory-readme-body {
  position: relative;
  min-height: 0;
}

.directory-readme-body.is-collapsed {
  max-height: 24rem;
  overflow: hidden;
}

.directory-readme-body.is-collapsed.has-overflow::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 5rem;
  pointer-events: none;
  content: "";
  background: linear-gradient(to bottom, transparent, var(--alt-background));
}

.directory-readme-content {
  box-sizing: border-box;
  width: 100%;
  padding: 1em 1.25em;
  overflow-wrap: break-word;
  word-break: break-word;
}

.directory-readme-toggle {
  display: flex;
  gap: 0.35em;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 2.75em;
  padding: 0.45em 1em;
  color: var(--primaryColor);
  font: inherit;
  font-size: 0.9em;
  font-weight: 600;
  cursor: pointer;
  background: var(--alt-background);
  border: 0;
  border-top: 1px solid color-mix(in srgb, var(--textPrimary) 10%, transparent);
}

.directory-readme-toggle:hover {
  background: color-mix(in srgb, var(--primaryColor) 7%, var(--alt-background));
}

.directory-readme-toggle:focus-visible {
  outline: 2px solid var(--primaryColor);
  outline-offset: -2px;
}

.directory-readme-toggle .material-symbols-outlined {
  font-size: 1.3em;
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

</style>
