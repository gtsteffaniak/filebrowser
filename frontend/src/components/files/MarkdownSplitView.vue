<template>
  <template v-if="active">
    <div
      class="split-divider"
      :class="{ resizing: isResizing }"
      @mousedown="startResize"
      @touchstart="startResize"
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

const MD_SPLIT_PERCENT_KEY = "mdSplitPercent";

function loadPreviewPercent() {
  const stored = Number(sessionStorage.getItem(MD_SPLIT_PERCENT_KEY));
  return stored >= 20 && stored <= 80 ? stored : 50;
}

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
    resizeContainer: {
      type: Object as PropType<HTMLElement | null>,
      default: null,
    },
  },
  emits: ["resize"],
  data: () => ({
    liveMarkdownContent: "", // editor current buffer
    liveContentFrame: null as number | null, // rAF handle
    previewScrollEl: null as HTMLElement | null, // DOM node MarkdownViewer should scroll in split mode
    scrollGuard: createScrollSyncGuard(),
    lastEditAt: 0, // used to ignore incoming scroll for a bit while typing
    previewPercent: loadPreviewPercent(), // width of this pane, dragged via the divider; persisted across the session
    isResizing: false,
    stopResize: null as (() => void) | null,
  }),
  watch: {
    active(isActive: boolean) {
      if (isActive) {
        this.$emit("resize", 100 - this.previewPercent);
        if (this.editor) {
          this.liveMarkdownContent = this.editor.getValue();
        }
      }
      this.$nextTick(() => {
        this.previewScrollEl = isActive ? (this.previewScrollWrapperEl() || null) : null;
      });
    },
  },
  mounted() {
    if (this.active) {
      this.$emit("resize", 100 - this.previewPercent);
      this.$nextTick(() => {
        this.previewScrollEl = this.previewScrollWrapperEl() || null;
      });
    }
  },
  beforeUnmount() {
    if (this.scrollGuard.cancel()) {
      this.syncScrollRatio();
    }
    if (this.liveContentFrame) {
      cancelAnimationFrame(this.liveContentFrame);
      this.liveContentFrame = null;
    }
    this.stopResize?.();
  },
  methods: {
    previewScrollWrapperEl(): HTMLElement | undefined {
      return (this.$refs.previewScrollWrapper as InstanceType<typeof Scrollbar> | undefined)?.$el;
    },
    startResize(e: MouseEvent | TouchEvent) {
      e.preventDefault();
      this.isResizing = true;
      const container = this.resizeContainer;
      const prevUserSelect = document.body.style.userSelect;
      document.body.style.userSelect = "none";

      const clientXFrom = (event: MouseEvent | TouchEvent): number | null => {
        if ("touches" in event) {
          return event.touches[0]?.clientX ?? event.changedTouches[0]?.clientX ?? null;
        }
        return event.clientX;
      };

      const updatePercent = (clientX: number | null) => {
        if (!container || clientX === null) return;
        const rect = container.getBoundingClientRect();
        const previewPercent = ((rect.right - clientX) / rect.width) * 100;
        this.setPreviewPercent(Math.min(80, Math.max(20, previewPercent)));
      };
      const onMove = (moveEvent: MouseEvent) => updatePercent(clientXFrom(moveEvent));
      const onTouchMove = (moveEvent: TouchEvent) => {
        moveEvent.preventDefault();
        updatePercent(clientXFrom(moveEvent));
      };
      const onUp = () => {
        this.isResizing = false;
        document.body.style.userSelect = prevUserSelect;
        this.stopResize = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
        window.removeEventListener("touchmove", onTouchMove);
        window.removeEventListener("touchend", onUp);
        window.removeEventListener("touchcancel", onUp);
      };
      this.stopResize = onUp;
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
      window.addEventListener("touchmove", onTouchMove, { passive: false });
      window.addEventListener("touchend", onUp);
      window.addEventListener("touchcancel", onUp);
    },
    setPreviewPercent(percent: number) {
      this.previewPercent = percent;
      sessionStorage.setItem(MD_SPLIT_PERCENT_KEY, String(percent));
      this.$emit("resize", 100 - percent);
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
    // Batches preview updates without re-parsing markdown on every single keystroke.
    scheduleLiveContentUpdate() {
      if (this.liveContentFrame) return;
      this.liveContentFrame = requestAnimationFrame(() => {
        this.liveContentFrame = null;
        if (this.editor) {
          this.liveMarkdownContent = this.editor.getValue();
        }
      });
    },
    syncScrollRatio() {
      if (!this.editor) return;
      const session = this.editor.session;
      const lineHeight = this.editor.renderer.lineHeight || 16;
      const screenRow = session.getScrollTop() / lineHeight;
      const { row } = session.screenToDocumentPosition(Math.floor(screenRow), 0);
      const maxRow = Math.max(0, session.getLength() - 1);
      const lastScreenRow = this.editor.renderer.getLastFullyVisibleRow();
      const { row: lastDocumentRow } = session.screenToDocumentPosition(lastScreenRow, 0);
      const atBottom = lastDocumentRow >= maxRow;
      mutations.setEditorScrollRatio(atBottom ? maxRow : row, 'editor');
    },
    // Called by the editor on every 'changeScrollTop' event
    handleEditorScroll() {
      this.scrollGuard.schedule(() => this.syncScrollRatio());
    },
    // Called by the editor when a remote scroll ratio needs to be applied. Ignored for a bit after
    // typing unless force is set (used for the initial restore on mount)
    applyScrollRatio(row: number, force = false) {
      if (!this.editor) return;
      if (!force && Date.now() - this.lastEditAt < 500) return;
      const lineHeight = this.editor.renderer.lineHeight || 16;
      const maxRow = Math.max(0, this.editor.session.getLength() - 1);
      const session = this.editor.session;
      const screenRow = session.documentToScreenPosition(Math.floor(row), 0).row;
      const target = row >= maxRow ? Number.MAX_SAFE_INTEGER : screenRow * lineHeight;
      this.scrollGuard.applyRemote(() => session.setScrollTop(target));
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
