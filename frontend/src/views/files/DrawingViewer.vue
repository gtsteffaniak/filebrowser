<template>
  <div class="drawing-viewer" ref="drawingContainer" @wheel.prevent="onWheel" @mousedown="onMouseDown">
    <!-- Loading Overlay -->
    <div v-if="loading" class="drawing-loading-overlay">
      <LoadingSpinner size="medium" />
      <p class="loading-status">{{ statusMessage }}</p>
    </div>

    <!-- Error Overlay -->
    <div v-else-if="error" class="drawing-error-overlay">
      <i class="material-symbols-outlined error-icon">error</i>
      <h3>{{ $t("cad.drawingLoadError") }}</h3>
      <p class="error-detail">{{ error }}</p>
      <button class="button button--flat" @click="loadDrawing">{{ $t("general.retry") }}</button>
    </div>

    <!-- Main Content -->
    <div v-else class="drawing-main">
      <!-- Toolbar -->
      <div class="drawing-toolbar">
        <!-- Sheet Selector -->
        <div class="drawing-sheets" v-if="sheets.length > 1">
          <button
            v-for="(sheet, index) in sheets"
            :key="sheet.nom || index"
            class="sheet-tab"
            :class="{ active: currentSheetIndex === index }"
            @click="selectSheet(index)"
          >
            <i class="material-symbols">description</i>
            <span>{{ sheet.nom || `Sheet ${index + 1}` }}</span>
          </button>
        </div>
        <div v-else class="single-sheet-title">
          <span>{{ currentSheet?.nom || fbdata.name }}</span>
        </div>

        <div class="toolbar-spacer"></div>

        <!-- Zoom Controls -->
        <div class="drawing-controls">
          <button class="icon-button" @click="zoomIn" :title="$t('cad.zoomIn')">
            <i class="material-symbols">zoom_in</i>
          </button>
          <button class="icon-button" @click="zoomOut" :title="$t('cad.zoomOut')">
            <i class="material-symbols">zoom_out</i>
          </button>
          <button class="icon-button" @click="resetView" :title="$t('cad.resetView')">
            <i class="material-symbols">fit_screen</i>
          </button>
          <span class="zoom-level">{{ Math.round(scale * 100) }}%</span>

          <!-- Export Actions -->
          <button class="action-btn" @click="exportCurrentSvg" :title="$t('cad.exportSvg')">
            <i class="material-symbols">image</i>
            <span>SVG</span>
          </button>
          <button v-if="hasDxf" class="action-btn" @click="exportDxf" :title="$t('cad.exportDxf')">
            <i class="material-symbols">architecture</i>
            <span>DXF</span>
          </button>
        </div>
      </div>

      <!-- Drawing Canvas -->
      <div class="drawing-canvas-wrapper" ref="canvasWrapper">
        <div
          class="drawing-viewport"
          :style="{
            transform: `translate(${panX}px, ${panY}px) scale(${scale})`,
            transformOrigin: '0 0'
          }"
          v-html="currentSheetSvg"
        ></div>
      </div>
    </div>
  </div>
</template>

<script>
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { loadCadDrawing } from "@/utils/cadService.ts";
import { resourcesApi } from "@/api";
import { getters, mutations, state } from "@/store";
import { ensureViewToken, requestViewIdentity } from "@/api/viewToken.js";

export default {
  name: "DrawingViewer",
  components: { LoadingSpinner },
  props: {
    fbdata: { type: Object, required: true },
  },
  data() {
    return {
      loading: true,
      error: null,
      statusMessage: "Loading SolidWorks drawing...",
      sheets: [],
      currentSheetIndex: 0,
      dxfBytes: null,
      scale: 1,
      panX: 40,
      panY: 40,
      isDragging: false,
      startX: 0,
      startY: 0,
    };
  },
  computed: {
    currentSheet() {
      return this.sheets[this.currentSheetIndex] || null;
    },
    currentSheetSvg() {
      return this.currentSheet?.svg || "";
    },
    hasDxf() {
      return this.dxfBytes && this.dxfBytes.length > 0;
    },
  },
  watch: {
    "fbdata.path"() {
      this.loadDrawing();
    },
  },
  mounted() {
    this.loadDrawing();
    window.addEventListener("mousemove", this.onMouseMove);
    window.addEventListener("mouseup", this.onMouseUp);
  },
  beforeUnmount() {
    window.removeEventListener("mousemove", this.onMouseMove);
    window.removeEventListener("mouseup", this.onMouseUp);
  },
  methods: {
    async loadDrawing() {
      this.loading = true;
      this.error = null;
      this.statusMessage = "Loading drawing in browser...";

      try {
        let viewToken = this.fbdata.viewToken;
        if (!viewToken && !getters.isShare()) {
          viewToken = await ensureViewToken(this.fbdata.source);
        }

        let url;
        if (getters.isShare()) {
          url = resourcesApi.getDownloadURLPublic(
            this.fbdata.path,
            state.shareInfo.hash,
            null,
            false,
            false,
            viewToken
          );
        } else {
          url = resourcesApi.getViewURL(this.fbdata.source, this.fbdata.path, viewToken, null, false);
        }

        const res = await fetch(url);
        if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
        const buffer = await res.arrayBuffer();

        const result = await loadCadDrawing(buffer, this.fbdata.name, (msg) => {
          this.statusMessage = msg;
        });

        this.sheets = result.plan?.feuilles || [];
        this.dxfBytes = result.dxfBytes || null;

        const activeIdx = this.sheets.findIndex((s) => s.active);
        this.currentSheetIndex = activeIdx >= 0 ? activeIdx : 0;
        this.resetView();
        this.loading = false;
      } catch (err) {
        this.error = err.message || "Failed to parse drawing";
        this.loading = false;
      }
    },

    selectSheet(index) {
      this.currentSheetIndex = index;
    },

    zoomIn() {
      this.scale = Math.min(this.scale * 1.25, 10);
    },

    zoomOut() {
      this.scale = Math.max(this.scale / 1.25, 0.1);
    },

    resetView() {
      this.scale = 1;
      this.panX = 40;
      this.panY = 40;
    },

    onWheel(e) {
      const zoomFactor = e.deltaY < 0 ? 1.15 : 0.87;
      const rect = this.$refs.canvasWrapper?.getBoundingClientRect();
      const mouseX = rect ? e.clientX - rect.left : 0;
      const mouseY = rect ? e.clientY - rect.top : 0;

      const newScale = Math.min(Math.max(this.scale * zoomFactor, 0.1), 10);
      this.panX = mouseX - ((mouseX - this.panX) * newScale) / this.scale;
      this.panY = mouseY - ((mouseY - this.panY) * newScale) / this.scale;
      this.scale = newScale;
    },

    onMouseDown(e) {
      if (e.button !== 0) return;
      this.isDragging = true;
      this.startX = e.clientX - this.panX;
      this.startY = e.clientY - this.panY;
    },

    onMouseMove(e) {
      if (!this.isDragging) return;
      this.panX = e.clientX - this.startX;
      this.panY = e.clientY - this.startY;
    },

    onMouseUp() {
      this.isDragging = false;
    },

    exportCurrentSvg() {
      if (!this.currentSheetSvg) return;
      const blob = new Blob([this.currentSheetSvg], { type: "image/svg+xml;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      const sheetName = this.currentSheet?.nom || `Sheet_${this.currentSheetIndex + 1}`;
      a.download = `${this.fbdata.name.replace(/\.slddrw$/i, "")}_${sheetName}.svg`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    },

    exportDxf() {
      if (!this.dxfBytes) return;
      const blob = new Blob([this.dxfBytes], { type: "application/dxf" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${this.fbdata.name.replace(/\.slddrw$/i, "")}.dxf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    },
  },
};
</script>

<style scoped>
.drawing-viewer {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: var(--surface-primary, #ffffff);
  color: var(--text-primary, #202124);
  overflow: hidden;
  position: relative;
  user-select: none;
}

.drawing-loading-overlay,
.drawing-error-overlay {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 2rem;
  text-align: center;
}

.loading-status {
  margin-top: 1.5rem;
  font-size: 0.95rem;
  color: var(--text-secondary, #5f6368);
}

.error-icon {
  font-size: 3.5rem;
  color: #ea4335;
  margin-bottom: 1rem;
}

.error-detail {
  margin: 0.5rem 0 1.5rem;
  color: var(--text-secondary, #5f6368);
  max-width: 500px;
}

.drawing-main {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

.drawing-toolbar {
  display: flex;
  align-items: center;
  padding: 0.5rem 1rem;
  background-color: var(--surface-secondary, #f1f3f4);
  border-bottom: 1px solid var(--border-color, #dadce0);
  gap: 0.75rem;
  z-index: 10;
  overflow-x: auto;
}

.drawing-sheets {
  display: flex;
  gap: 0.4rem;
}

.sheet-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  border: 1px solid transparent;
  background: transparent;
  cursor: pointer;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary, #5f6368);
  transition: all 0.2s ease;
}

.sheet-tab:hover {
  background-color: var(--surface-hover, rgba(0, 0, 0, 0.05));
}

.sheet-tab.active {
  background-color: var(--primary, #1a73e8);
  color: #ffffff;
}

.single-sheet-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.toolbar-spacer {
  flex: 1;
}

.drawing-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.icon-button {
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0.4rem;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-primary, #202124);
  transition: background-color 0.2s;
}

.icon-button:hover {
  background-color: var(--surface-hover, rgba(0, 0, 0, 0.08));
}

.zoom-level {
  font-size: 0.85rem;
  font-family: monospace;
  min-width: 45px;
  text-align: center;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.75rem;
  border-radius: 4px;
  border: 1px solid var(--border-color, #dadce0);
  background: var(--surface-primary, #ffffff);
  color: var(--text-primary, #202124);
  cursor: pointer;
  font-size: 0.8rem;
  font-weight: 500;
}

.action-btn:hover {
  background-color: var(--surface-hover, rgba(0, 0, 0, 0.05));
}

.drawing-canvas-wrapper {
  flex: 1;
  position: relative;
  overflow: hidden;
  background-color: #ffffff;
  cursor: grab;
}

.drawing-canvas-wrapper:active {
  cursor: grabbing;
}

.drawing-viewport {
  position: absolute;
  top: 0;
  left: 0;
  will-change: transform;
}

.drawing-viewport :deep(svg) {
  display: block;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  background-color: #ffffff;
}
</style>
