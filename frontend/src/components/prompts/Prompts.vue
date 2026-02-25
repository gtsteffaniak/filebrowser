<template>
  <Teleport to="body" v-for="prompt in prompts" :key="'prompt-' + prompt.id">
    <div
      ref="promptWindow"
      class="card floating floating-window"
      :class="{
        'dark-mode': isDarkMode,
        'is-dragging': isDragging(prompt.id),
        'is-resizing': resizingId === prompt.id,
        'prompt-behind': !isTopmost(prompt.id),
        'blocked': isBlocked(prompt)
      }"
      @mousedown="makeTopPrompt(prompt.id)"
      :style="{
        transform: `translate(calc(-50% + ${(dragOffsets[prompt.id]?.x || 0)}px), calc(-50% + ${(dragOffsets[prompt.id]?.y || 0)}px))`,
        width: sizes[prompt.id]?.width ? sizes[prompt.id].width + 'px' : null,
        height: sizes[prompt.id]?.height ? sizes[prompt.id].height + 'px' : null,
        maxWidth: sizes[prompt.id]?.width ? 'none' : null,
        maxHeight: sizes[prompt.id]?.height ? 'none' : null,
        zIndex: 5 + prompts.indexOf(prompt),
      }"
      :aria-label="prompt.name + '-prompt'"
    >
      <header
        class="prompt-taskbar"
        :class="{ 'is-dragging': isDragging(prompt.id) }"
        @mousedown="onPointerDown($event, prompt.id, 'mouse')"
        @touchstart.passive="onPointerDown($event, prompt.id, 'touch')"
      >
        <button
          type="button"
          class="prompt-close"
          :aria-label="$t('general.close')"
          :title="$t('general.close')"
          :disabled="isBlocked(prompt)"
          @click.stop="closePrompt(prompt.id)"
          @mousedown.stop
          @touchstart.stop
        >
          <i class="material-icons">close</i>
        </button>
        <div class="prompt-taskbar-drag">
          <span class="prompt-title">{{ prompt?.props?.title || getDisplayTitle(prompt?.name) }}</span>
        </div>
      </header>
      <!-- Resize for the prompt on top -->
      <div class="resize-handles">
        <div class="resize-handle resize-handle-top" @mousedown.stop="startResize($event, prompt.id, 'top')" @touchstart.stop.passive="startResize($event, prompt.id, 'top')"></div>
        <div class="resize-handle resize-handle-bottom" @mousedown.stop="startResize($event, prompt.id, 'bottom')" @touchstart.stop.passive="startResize($event, prompt.id, 'bottom')"></div>
        <div class="resize-handle resize-handle-left" @mousedown.stop="startResize($event, prompt.id, 'left')" @touchstart.stop.passive="startResize($event, prompt.id, 'left')"></div>
        <div class="resize-handle resize-handle-right" @mousedown.stop="startResize($event, prompt.id, 'right')" @touchstart.stop.passive="startResize($event, prompt.id, 'right')"></div>
        <div class="resize-handle resize-handle-top-left" @mousedown.stop="startResize($event, prompt.id, 'top-left')" @touchstart.stop.passive="startResize($event, prompt.id, 'top-left')"></div>
        <div class="resize-handle resize-handle-top-right" @mousedown.stop="startResize($event, prompt.id, 'top-right')" @touchstart.stop.passive="startResize($event, prompt.id, 'top-right')"></div>
        <div class="resize-handle resize-handle-bottom-left" @mousedown.stop="startResize($event, prompt.id, 'bottom-left')" @touchstart.stop.passive="startResize($event, prompt.id, 'bottom-left')"></div>
        <div class="resize-handle resize-handle-bottom-right" @mousedown.stop="startResize($event, prompt.id, 'bottom-right')" @touchstart.stop.passive="startResize($event, prompt.id, 'bottom-right')"></div>
      </div>
      <component
        :is="prompt.name"
        v-bind="getPromptProps(prompt)"
      />
    </div>
  </Teleport>
</template>

<script>
import Help from "./Help.vue";
import Info from "./Info.vue";
import Delete from "./Delete.vue";
import Rename from "./Rename.vue";
import Download from "./Download.vue";
import MoveCopy from "./MoveCopy.vue";
import NewFile from "./NewFile.vue";
import NewDir from "./NewDir.vue";
import Replace from "./Replace.vue";
import ReplaceRename from "./ReplaceRename.vue";
import Share from "./Share.vue";
import Upload from "./Upload.vue";
import CreateApi from "./CreateApi.vue";
import ActionApi from "./ActionApi.vue";
import SidebarLinks from "./SidebarLinks.vue";
import IconPicker from "./IconPicker.vue";
import Sidebar from "../sidebar/Sidebar.vue";
import UserEdit from "./UserEdit.vue";
import Totp from "./Totp.vue";
import Access from "./Access.vue";
import Password from "./Password.vue";
import PlaybackQueue from "./PlaybackQueue.vue";
import PathPicker from "./PathPicker.vue";
import SaveBeforeExit from "./SaveBeforeExit.vue";
import CopyPasteConfirm from "./CopyPasteConfirm.vue";
import CloseWithActiveUploads from "./CloseWithActiveUploads.vue";
import Generic from "./Generic.vue";
import ShareInfo from "./ShareInfo.vue";
import FileList from "./FileListing.vue";
import Archive from "./Archive.vue";
import Unarchive from "./Unarchive.vue";
import { state, getters, mutations } from "@/store";

export default {
  name: "Prompts",
  components: {
    UserEdit,
    Info,
    Delete,
    Rename,
    Download,
    Move: MoveCopy,
    Copy: MoveCopy,
    Share,
    NewFile,
    NewDir,
    Help,
    Replace,
    ReplaceRename,
    Totp,
    Upload,
    Sidebar,
    CreateApi,
    ActionApi,
    SidebarLinks,
    IconPicker,
    Access,
    Password,
    PlaybackQueue,
    PathPicker,
    SaveBeforeExit,
    CopyPasteConfirm,
    CloseWithActiveUploads,
    Generic,
    ShareInfo,
    FileList,
    Archive,
    Unarchive,
  },
  data() {
    return {
      dragOffsets: {},
      dragStarts: {},
      draggingIds: new Set(),
      touchIds: {},
      sizes: {},
      resizingId: null,
      resizingEdge: null,
      resizeStart: {
        width: 0,
        height: 0,
        offsetX: 0,
        offsetY: 0,
        clientX: 0,
        clientY: 0,
      },
    };
  },
  computed: {
    prompts() {
      // Filter out ContextMenu - it's rendered separately in Layout.vue
      const p = (state.prompts || []).filter(prompt => prompt.name !== "ContextMenu" && prompt.name !== "OverflowMenu");
      return p;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    pinnedPromptExists() {
      return this.prompts.some(p => p.pinnedHover);
    },
  },
  methods: {
    isTopmost(id) {
      const allPrompts = this.prompts;
      return allPrompts[allPrompts.length - 1]?.id === id;
    },
    isDragging(id) {
      return this.draggingIds.has(id);
    },
    isBlocked(prompt) {
      // If there is a pinned prompt and this prompt isn't it, it's blocked
      if (this.pinnedPromptExists && !prompt.pinnedHover) return true;
      // If this prompt has any open child, also it's blocked
      if (this.prompts.some(p => p.parentId === prompt.id)) return true;
      return false;
    },
    getPromptProps(prompt) {
      const baseProps = {
        ...prompt.props,
        promptId: prompt.id,
      };
      if (prompt.name === "move" || prompt.name === "copy") {
        return {
          ...baseProps,
          operation: prompt.name,
        };
      }
      return baseProps;
    },
    makeTopPrompt(id) {
      if (getters.isMobile()) return; // Don't allow in mobile since we can lose the prompt easily.
      const prompt = this.prompts.find(p => p.id === id);
      if (!prompt) return;
      if (this.pinnedPromptExists && !prompt.pinnedHover) return;
      const index = state.prompts.findIndex(p => p.id === id);
      if (index === -1) return;
      const pinnedCount = state.prompts.filter(p => p.pinnedHover).length;
      const targetIndex = state.prompts.length - pinnedCount;
      if (index >= targetIndex) return;
      const [movedPrompt] = state.prompts.splice(index, 1);
      state.prompts.splice(targetIndex, 0, movedPrompt);
    },
    getDisplayTitle(promptName) {
      // convert to lowercase
      // Explicit switch statement for compile-time safety with ESLint i18n validation
      switch (promptName.toLowerCase()) {
        case "delete":
          return this.$t("general.delete");
        case "access":
          return this.$t("access.rules");
        case "download":
          return this.$t("prompts.download");
        case "move":
          return this.$t("general.move");
        case "copy":
          return this.$t("general.copy");
        case "rename":
          return this.$t("general.rename");
        case "share":
          return this.$t("general.share");
        case "replace":
          return this.$t("general.replace");
        case "info":
          return this.$t("prompts.fileInfo");
        case "help":
          return this.$t("general.help");
        case "upload":
          return this.$t("general.upload");
        case "createapi":
          return this.$t("api.createTitle");
        case "actionapi":
          return this.$t("api.title");
        case "sidebarlinks":
          return this.$t("sidebar.customizeLinks");
        case "password":
          return this.$t("general.password");
        case "playbackqueue":
          return this.$t("player.QueuePlayback");
        case "pathpicker":
          return this.$t("prompts.selectPath");
        case "savebeforeexit":
          return this.$t("prompts.saveBeforeExit");
        case "copypasteconfirm":
          return this.$t("prompts.copyPasteConfirm");
        case "closewithactiveuploads":
          return this.$t("prompts.closeWithActiveUploads");
        case "shareinfo":
          return this.$t("share.shareInfo");
        case "totp":
          return this.$t("otp.name");
        case "useredit":
          return this.$t("settings.modifyOtherUser");
        case "deleteuser":
          return this.$t("prompts.deleteUserMessage");
        case "iconpicker":
          return this.$t("sidebar.pickIcon");
        case "newfile":
          return this.$t("prompts.newFile");
        case "newdir":
          return this.$t("prompts.newDir");
        case "replace-rename":
          return this.$t("general.replace");
        case "archive":
          return this.$t("prompts.archive");
        case "unarchive":
          return this.$t("prompts.unarchive");
        default:
          console.warn("[Prompts.vue] unknown prompt name", promptName);
          // Fallback for unknown prompt types
          return promptName;
      }
    },
    closePrompt(id) {
      // Find the prompt we're trying to close
      const promptToClose = state.prompts.find(p => p.id === id);
      
      // Check if it's the upload prompt with active uploads
      if (promptToClose?.name === "upload") {
        const hasActiveUploads = state.upload.isUploading;
        const hasWarningPrompt = state.prompts.some(p => p.name === "CloseWithActiveUploads");
        
        if (hasActiveUploads && !hasWarningPrompt) {
          // Show warning prompt instead of closing
          mutations.showHover({
            name: "CloseWithActiveUploads",
            pinnedHover: true,
            confirm: () => {
              // User confirmed to close anyway - close the upload prompt
              mutations.closePromptById(id);
              // Clean up state for this prompt
              delete this.dragOffsets[id];
              delete this.dragStarts[id];
              delete this.touchIds[id];
              this.draggingIds.delete(id);
              delete this.sizes[id];
            },
            cancel: () => {
              // User cancelled - just close the warning prompt
              mutations.closeTopHover();
            },
          });
          return;
        }
      }
      // Normal close behavior
      mutations.closePromptById(id);
      // Clean up state for this prompt
      delete this.dragOffsets[id];
      delete this.dragStarts[id];
      delete this.touchIds[id];
      this.draggingIds.delete(id);
      delete this.sizes[id];
    },
    getPointerPos(e, type) {
      if (type === "touch") {
        const t = e.touches && e.touches[0];
        return t ? { x: t.clientX, y: t.clientY } : null;
      }
      return { x: e.clientX, y: e.clientY };
    },
    clampDragOffset(id, el) {
      if (!el || typeof el.getBoundingClientRect !== "function") return;

      const viewportW = window.innerWidth;
      const viewportH = window.innerHeight;
      const headerEl = el.querySelector && el.querySelector(".prompt-taskbar");
      const headerHeight = headerEl ? headerEl.getBoundingClientRect().height : 40;
      const rect = el.getBoundingClientRect();
      const windowHeight = rect.height;
      const offset = this.dragOffsets[id] || { x: 0, y: 0 };
      const centerX = viewportW / 2 + offset.x;
      const centerY = viewportH / 2 + offset.y;

      const minCenterX = 0;
      const maxCenterX = viewportW;
      const minCenterY = windowHeight / 2;
      const maxCenterY = viewportH - headerHeight / 2;
      const clampedX = Math.max(minCenterX, Math.min(maxCenterX, centerX));
      const clampedY = Math.max(minCenterY, Math.min(maxCenterY, centerY));

      this.dragOffsets[id] = {
        x: clampedX - viewportW / 2,
        y: clampedY - viewportH / 2,
      };
    },
    onPointerDown(e, id, type) {
      this.makeTopPrompt(id);
      if (type === "mouse" && e.button !== 0) return;
      if (type === "touch") {
        this.touchIds[id] = e.changedTouches && e.changedTouches[0] && e.changedTouches[0].identifier;
      }

      const pos = this.getPointerPos(e, type);
      if (!pos) return;

      if (!this.dragOffsets[id]) this.dragOffsets[id] = { x: 0, y: 0 };
      this.dragStarts[id] = { x: pos.x - this.dragOffsets[id].x, y: pos.y - this.dragOffsets[id].y };
      this.draggingIds.add(id);

      const move = (e) => this.onPointerMove(e, id, type);
      const end = (e) => this.onPointerEnd(e, id, type, move, end);

      if (type === "mouse") {
        window.addEventListener("mousemove", move);
        window.addEventListener("mouseup", end);
      } else {
        window.addEventListener("touchmove", move, { passive: false });
        window.addEventListener("touchend", end, { passive: true });
        window.addEventListener("touchcancel", end, { passive: true });
      }
    },
    onPointerMove(e, id, type) {
      if (!this.dragStarts[id]) return;
      let pos;
      if (type === "touch") {
        if (!e.touches) return;
        const t = Array.from(e.touches).find((touch) => touch.identifier === this.touchIds[id]);
        if (!t) return;
        e.preventDefault();
        pos = { x: t.clientX, y: t.clientY };
      } else {
        pos = { x: e.clientX, y: e.clientY };
      }

      this.dragOffsets[id] = {
        x: pos.x - this.dragStarts[id].x,
        y: pos.y - this.dragStarts[id].y,
      };

      // Find the element for this prompt
      const windows = this.$refs.promptWindow;
      const el = Array.isArray(windows) ? windows.find(w => w?.getAttribute('aria-label') === `${this.prompts.find(p => p.id === id)?.name}-prompt`) : windows;
      if (el) this.clampDragOffset(id, el);
    },
    onPointerEnd(_e, id, type, moveHandler, endHandler) {
      if (type === "touch") {
        delete this.touchIds[id];
        window.removeEventListener("touchmove", moveHandler);
        window.removeEventListener("touchend", endHandler);
        window.removeEventListener("touchcancel", endHandler);
      } else {
        window.removeEventListener("mousemove", moveHandler);
        window.removeEventListener("mouseup", endHandler);
      }
      delete this.dragStarts[id];
      this.draggingIds.delete(id);
    },
    startResize(e, id, edge) {
      const type = e.type === 'touchstart' ? 'touch' : 'mouse';
      const pos = this.getPointerPos(e, type);
      if (!pos) return;
      // Find the window
      const windows = this.$refs.promptWindow;
      const el = Array.isArray(windows)
        ? windows.find(w => w?.getAttribute('aria-label') === `${this.prompts.find(p => p.id === id)?.name}-prompt`)
        : windows;
      if (!el) return;
      // Capture current size (if not already fixed, set it)
      if (!this.sizes[id]) {
        this.sizes[id] = {
          width: el.offsetWidth,
          height: el.offsetHeight,
        };
      }
      // Initial values
      this.resizingId = id;
      this.resizingEdge = edge;
      this.resizeStart = {
        width: this.sizes[id].width,
        height: this.sizes[id].height,
        offsetX: this.dragOffsets[id]?.x || 0,
        offsetY: this.dragOffsets[id]?.y || 0,
        clientX: pos.x,
        clientY: pos.y,
      };

      const moveHandler = (e) => this.onResizeMove(e, type);
      const endHandler = (e) => this.onResizeEnd(e, type, moveHandler, endHandler);

      if (type === 'mouse') {
        window.addEventListener('mousemove', moveHandler);
        window.addEventListener('mouseup', endHandler);
      } else {
        window.addEventListener('touchmove', moveHandler, { passive: false });
        window.addEventListener('touchend', endHandler, { passive: true });
        window.addEventListener('touchcancel', endHandler, { passive: true });
      }
      e.preventDefault();
      e.stopPropagation();
    },

    onResizeMove(e, type) {
      if (!this.resizingId) return;
      const pos = this.getPointerPos(e, type);
      if (!pos) return;
      if (type === 'touch') {
        e.preventDefault(); // prevent scrolling while resizing
      }

      const id = this.resizingId;
      const edge = this.resizingEdge;
      const start = this.resizeStart;

      const dx = pos.x - start.clientX;
      const dy = pos.y - start.clientY;

      let newWidth = start.width;
      let newHeight = start.height;
      let deltaOffsetX = 0;
      let deltaOffsetY = 0;

      const MIN_WIDTH = 200;
      const MIN_HEIGHT = 150;

      // Calculate new dimensions and the required offset change to keep opposite edges fixed
      // If not we'll have some weird behaviors when resizing.
      if (edge.includes('left')) {
        newWidth = Math.max(MIN_WIDTH, start.width - dx);
        deltaOffsetX = -(newWidth - start.width) / 2;   // right
      } else if (edge.includes('right')) {
        newWidth = Math.max(MIN_WIDTH, start.width + dx);
        deltaOffsetX = +(newWidth - start.width) / 2;   // left
      }
      if (edge.includes('top') && !edge.includes('left') && !edge.includes('right')) {
        // pure top edge, not part of a corner
        newHeight = Math.max(MIN_HEIGHT, start.height - dy);
        deltaOffsetY = -(newHeight - start.height) / 2; // bottom
      } else if (edge.includes('bottom') && !edge.includes('left') && !edge.includes('right')) {
        // pure bottom edge
        newHeight = Math.max(MIN_HEIGHT, start.height + dy);
        deltaOffsetY = +(newHeight - start.height) / 2; // top
      } else if (edge.includes('top-left')) {
        newWidth = Math.max(MIN_WIDTH, start.width - dx);
        newHeight = Math.max(MIN_HEIGHT, start.height - dy);
        deltaOffsetX = -(newWidth - start.width) / 2;
        deltaOffsetY = -(newHeight - start.height) / 2;
      } else if (edge.includes('top-right')) {
        newWidth = Math.max(MIN_WIDTH, start.width + dx);
        newHeight = Math.max(MIN_HEIGHT, start.height - dy);
        deltaOffsetX = +(newWidth - start.width) / 2;
        deltaOffsetY = -(newHeight - start.height) / 2;
      } else if (edge.includes('bottom-left')) {
        newWidth = Math.max(MIN_WIDTH, start.width - dx);
        newHeight = Math.max(MIN_HEIGHT, start.height + dy);
        deltaOffsetX = -(newWidth - start.width) / 2;
        deltaOffsetY = +(newHeight - start.height) / 2;
      } else if (edge.includes('bottom-right')) {
        newWidth = Math.max(MIN_WIDTH, start.width + dx);
        newHeight = Math.max(MIN_HEIGHT, start.height + dy);
        deltaOffsetX = +(newWidth - start.width) / 2;
        deltaOffsetY = +(newHeight - start.height) / 2;
      }
      // Update sizes
      this.sizes[id] = { width: newWidth, height: newHeight };
      // Update drag
      this.dragOffsets[id] = {
        x: start.offsetX + deltaOffsetX,
        y: start.offsetY + deltaOffsetY,
      };
      // Clamp to viewport
      const windows = this.$refs.promptWindow;
      const el = Array.isArray(windows)
        ? windows.find(w => w?.getAttribute('aria-label') === `${this.prompts.find(p => p.id === id)?.name}-prompt`)
        : windows;
      if (el) this.clampDragOffset(id, el);
    },
    onResizeEnd(_e, type, moveHandler, endHandler) {
      if (type === 'mouse') {
        window.removeEventListener('mousemove', moveHandler);
        window.removeEventListener('mouseup', endHandler);
      } else {
        window.removeEventListener('touchmove', moveHandler);
        window.removeEventListener('touchend', endHandler);
        window.removeEventListener('touchcancel', endHandler);
      }
      this.resizingId = null;
      this.resizingEdge = null;
      this.resizeStart = { width:0, height:0, offsetX:0, offsetY:0, clientX:0, clientY:0 };
    },
  },
};
</script>

<style scoped>

.floating-window > :deep(.card-content) {
  padding-top: 3.5em !important;
  padding-bottom: 3.5em !important;
  margin-top: 1px; /* 1px to avoid edge flickering */
  margin-bottom: 1px;  /* 1px to avoid edge flickering */
}

.floating-window > :deep(.card-actions) {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
}

/* Backdrop-filter support */
@supports (backdrop-filter: none) {
  .floating-window :deep(.prompt-taskbar) {
    backdrop-filter: blur(12px) invert(0.2);
    background-color: color-mix(in srgb, var(--background) 50%, transparent);
  }
  .floating-window :deep(.card-actions) {
    backdrop-filter: blur(12px);
    background-color: transparent;
  }
}

.floating-window.is-dragging {
  border-color: var(--primaryColor);
  user-select: none;
}

.floating-window.is-dragging > :deep(.card-content),
.floating-window.is-dragging > :deep(.card-actions) {
  pointer-events: none;
}

.floating-window.is-resizing {
  border-color: var(--primaryColor);
}

/* Block all interactions but allow move and resize */
.floating-window.blocked > :not(.prompt-taskbar):not(.resize-handles) {
  pointer-events: none;
}

.floating-window.blocked {
  cursor: not-allowed;
  user-select: none;
  opacity: 0.7;
  transition: opacity 0.5s;
}

.prompt-close:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.prompt-behind {
  filter: brightness(0.85);
  transition: filter 0.5s;
}

.prompt-taskbar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  cursor: grab;
  user-select: none;
  touch-action: none;
  transition: background 0.15s;
  background: var(--surfaceSecondary);
  height: 3em;
  z-index: 10;
}

.prompt-taskbar:hover {
  background: color-mix(in srgb, var(--primaryColor) 12%, var(--surfaceSecondary, #f5f5f5));
}

.prompt-taskbar.is-dragging {
  cursor: grabbing;
  background: color-mix(in srgb, var(--primaryColor) 18%, var(--surfaceSecondary, #f5f5f5));
}

.dark-mode .prompt-taskbar:hover {
  background: color-mix(in srgb, var(--primaryColor) 18%, var(--surfaceSecondary));
}

.dark-mode .prompt-taskbar.is-dragging {
  background: color-mix(in srgb, var(--primaryColor) 22%, var(--surfaceSecondary));
}

.prompt-close {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2em;
  height: 2em;
  padding: 0;
  border: none;
  border-radius: 1em;
  background: #c62828;
  color: #fff;
  cursor: pointer;
  transition: background 0.15s, filter 0.15s;
}

.prompt-close:hover {
  background: #b71c1c;
  filter: brightness(1.1);
}

.prompt-close .material-icons {
  font-size: 1em;
}

.prompt-taskbar-drag {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.prompt-title {
  font-weight: 500;
  font-size: 1rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 80%;
}

.resize-handles {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
}
.resize-handle {
  position: absolute;
  pointer-events: auto;
  background: transparent;
  z-index: 20;
}
.resize-handle-top {
  top: -5px;
  left: 5px;
  right: 5px;
  height: 10px;
  cursor: n-resize;
}
.resize-handle-bottom {
  bottom: -5px;
  left: 5px;
  right: 5px;
  height: 10px;
  cursor: s-resize;
}
.resize-handle-left {
  left: -5px;
  top: 5px;
  bottom: 5px;
  width: 10px;
  cursor: w-resize;
}
.resize-handle-right {
  right: -5px;
  top: 5px;
  bottom: 5px;
  width: 10px;
  cursor: e-resize;
}
.resize-handle-top-left {
  top: -5px;
  left: -5px;
  width: 15px;
  height: 15px;
  cursor: nw-resize;
}
.resize-handle-top-right {
  top: -5px;
  right: -5px;
  width: 15px;
  height: 15px;
  cursor: ne-resize;
}
.resize-handle-bottom-left {
  bottom: -5px;
  left: -5px;
  width: 15px;
  height: 15px;
  cursor: sw-resize;
}
.resize-handle-bottom-right {
  bottom: -5px;
  right: -5px;
  width: 15px;
  height: 15px;
  cursor: se-resize;
}

</style>
