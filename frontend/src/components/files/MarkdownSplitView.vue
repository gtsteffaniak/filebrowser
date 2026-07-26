<template>
  <template v-if="active">
    <div
      class="split-divider"
      :class="{ resizing: isResizing }"
      role="separator"
      aria-orientation="vertical"
      :aria-valuenow="Math.round(previewPercent)"
      aria-valuemin="20"
      aria-valuemax="80"
      tabindex="0"
      @mousedown="startResize"
      @keydown="handleKeydown"
    ></div>
    <div class="split-preview-pane" :style="{ flexBasis: `${previewPercent}%` }">
      <Scrollbar ref="previewScrollWrapper" force-enabled class="split-preview-scrollbar">
        <MarkdownViewer
          :split-mode="true"
          :live-content="liveMarkdownContent"
          :scroll-target="previewScrollEl"
        />
      </Scrollbar>
    </div>
  </template>
</template>

<script lang="ts">
import type { PropType } from "vue";
import type { Ace } from "ace-builds";
import { mutations } from "@/store";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import Scrollbar from "@/components/files/Scrollbar.vue";
import { createScrollSyncGuard } from "@/utils/markdownScrollSync";

export default {
  name: "markdownSplitView",
  components: {
    Scrollbar,
    MarkdownViewer: createAsyncComponent(() => import("@/views/files/MarkdownViewer.vue")),
  },
  props: {
    editor: {
      type: Object as PropType<Ace.Editor | null>,
      default: null,
    },
    active: {
      type: Boolean,
      default: false,
    },
  },
  data: () => ({
    liveMarkdownContent: "", // editor current buffer
    liveContentTimer: null as ReturnType<typeof setTimeout> | null, // debounce for updating liveMarkdownContent
    previewScrollEl: null as HTMLElement | null, // DOM node MarkdownViewer should scroll in split mode
    scrollGuard: createScrollSyncGuard(),
    lastEditAt: 0, // used to ignore incoming scroll for a bit while typing
    previewPercent: 50, // width of this pane, dragged via the divider; resets each time split view mounts
    isResizing: false,
    stopResize: null as (() => void) | null,
  }),
  watch: {
    active(isActive: boolean) {
      if (isActive && this.editor) {
        this.liveMarkdownContent = this.editor.getValue();
      }
      this.$nextTick(() => {
        this.previewScrollEl = isActive ? (this.previewScrollWrapperEl() || null) : null;
      });
    },
  },
  mounted() {
    if (this.active) {
      this.$nextTick(() => {
        this.previewScrollEl = this.previewScrollWrapperEl() || null;
      });
    }
  },
  beforeUnmount() {
    if (this.scrollGuard.cancel()) {
      this.syncScrollRatio();
    }
    if (this.liveContentTimer) {
      clearTimeout(this.liveContentTimer);
      this.liveContentTimer = null;
    }
    this.stopResize?.();
  },
  methods: {
    previewScrollWrapperEl(): HTMLElement | undefined {
      return (this.$refs.previewScrollWrapper as InstanceType<typeof Scrollbar> | undefined)?.$el;
    },
    startResize(e: MouseEvent) {
      e.preventDefault();
      this.isResizing = true;
      const container = this.$el.parentElement;
      const prevUserSelect = document.body.style.userSelect;
      document.body.style.userSelect = "none";

      const onMove = (moveEvent: MouseEvent) => {
        if (!container) return;
        const rect = container.getBoundingClientRect();
        const previewPercent = ((rect.right - moveEvent.clientX) / rect.width) * 100;
        this.previewPercent = Math.min(80, Math.max(20, previewPercent));
        this.$emit("resize", 100 - this.previewPercent);
      };
      const onUp = () => {
        this.isResizing = false;
        document.body.style.userSelect = prevUserSelect;
        this.stopResize = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      this.stopResize = onUp;
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    handleKeydown(e: KeyboardEvent) {
      const step = e.shiftKey ? 10 : 2;
      if (e.key === "ArrowLeft") {
        this.previewPercent = Math.min(80, this.previewPercent + step);
      } else if (e.key === "ArrowRight") {
        this.previewPercent = Math.max(20, this.previewPercent - step);
      } else {
        return;
      }
      e.preventDefault();
      this.$emit("resize", 100 - this.previewPercent);
    },
    setLiveContent(value: string) {
      this.liveMarkdownContent = value;
    },
    handleEditorChange() {
      this.lastEditAt = Date.now();
      if (this.active) {
        this.scheduleLiveContentUpdate();
      }
    },
    // Debounces preview updates so fast typing doesn't re-parse markdown on every keystroke.
    // The viewer liveContent watcher adds a small extra debounce on top of this one.
    scheduleLiveContentUpdate() {
      if (this.liveContentTimer) clearTimeout(this.liveContentTimer);
      this.liveContentTimer = setTimeout(() => {
        this.liveContentTimer = null;
        if (this.editor) {
          this.liveMarkdownContent = this.editor.getValue();
        }
      }, 100);
    },
    syncScrollRatio() {
      if (!this.editor) return;
      const lineHeight = this.editor.renderer.lineHeight || 16;
      const screenRow = this.editor.session.getScrollTop() / lineHeight;
      const { row } = this.editor.session.screenToDocumentPosition(Math.floor(screenRow), 0);
      mutations.setEditorScrollRatio(row, 'editor');
    },
    // Called by the editor on every 'changeScrollTop' event
    handleEditorScroll() {
      this.scrollGuard.schedule(() => this.syncScrollRatio());
    },
    // Called by the editor when a remote scroll ratio needs to be applied. Ignored for a bit after
    // typing, unless force is set (used for the one-time initial restore on mount).
    applyScrollRatio(row: number, force = false) {
      if (!this.editor) return;
      if (!force && Date.now() - this.lastEditAt < 500) return;
      const lineHeight = this.editor.renderer.lineHeight || 16;
      const maxRow = Math.max(0, this.editor.session.getLength() - 1);
      // Ace clamps setScrollTop to its real max (including scrollPastEnd) on render,
      // so an oversized target reliably lands at the true bottom.
      const target = row >= maxRow ? Number.MAX_SAFE_INTEGER : row * lineHeight;
      this.scrollGuard.applyRemote(() => this.editor.session.setScrollTop(target));
    },
  },
};
</script>

<style scoped>
.split-divider {
  flex: 0 0 5px;
  position: relative;
  cursor: col-resize;
  background-color: transparent;
}

.split-divider::before {
  content: "";
  position: absolute;
  left: 2px;
  top: 0;
  bottom: 0;
  width: 1px;
  background-color: var(--alt-background);
}

.split-divider:hover::before,
.split-divider.resizing::before {
  background-color: var(--primaryColor);
}

.split-preview-pane {
  flex: 0 0 auto;
  min-width: 0;
  height: 100%;
  position: relative;
  overflow: hidden;
}

.split-preview-scrollbar {
  height: 100%;
  width: 100%;
  overflow-y: auto;
  scrollbar-width: none;
}

.split-preview-scrollbar::-webkit-scrollbar {
  display: none;
  width: 0;
}
</style>
