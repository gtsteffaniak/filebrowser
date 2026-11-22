<template>
  <div :class="['cadgl-viewer', { 'cadgl-viewer--occt': displayMode === 'occt' }]" ref="container">
    <canvas ref="glcanvas" class="cadgl-canvas"></canvas>
    <canvas ref="triedre" class="cadgl-triedre"></canvas>
    <canvas ref="overlay" class="cadgl-overlay"></canvas>
    <div class="measure-layer">
      <div
        v-for="(label, i) in labelBoxes"
        :key="`ml${i}`"
        class="measure-label"
        :style="{ left: `${label.x}px`, top: `${label.y}px`, visibility: label.visible ? 'visible' : 'hidden' }"
      >{{ label.text }}</div>
    </div>

    <div v-if="loading" class="loading-overlay">
      <img v-if="previewBgUrl" :src="previewBgUrl" class="preview-bg" alt="" />
      <div class="loading-front">
        <LoadingSpinner size="medium" />
        <p v-if="cadStatusMessage" class="cad-status-text">{{ cadStatusMessage }}</p>
      </div>
    </div>
    <div v-if="error" class="error-overlay">
      <div class="error-content">
        <i class="material-symbols-outlined">error</i>
        <h3>{{ $t("threejs.failedToLoad") }}</h3>
        <p>{{ error }}</p>
      </div>
    </div>

    <p
      v-if="brepLoading || brepError"
      :class="['brep-feedback', { 'brep-feedback--error': brepError }]"
      :role="brepError ? 'alert' : 'status'"
      aria-live="polite"
    >{{ brepError || cadStatusMessage || $t("general.loading", { suffix: "..." }) }}</p>

    <template v-if="!loading && !error">
      <div class="mode-switcher" :title="$t('threejs.viewMode')">
        <button
          type="button"
          :class="['mode-btn', { active: displayMode === 'standard' }]"
          @click="setDisplayMode('standard')"
        >
          <i class="material-symbols-outlined">palette</i>
          <span class="mode-name">{{ $t("threejs.modeStandard") }}</span>
        </button>
        <button
          type="button"
          :class="['mode-btn', { active: displayMode === 'occt' }]"
          @click="setDisplayMode('occt')"
        >
          <i class="material-symbols-outlined">format_shapes</i>
          <span class="mode-name">{{ $t("threejs.modeOcct") }}</span>
        </button>
      </div>

      <div v-if="statusText" class="status-bar">{{ statusText }}</div>

      <!-- Keep actions away from the OpenSW triad (bottom-left) and view cube (bottom-right). -->
      <div v-if="measuring" class="measure-toolbar">
        <button
          type="button"
          :class="['measure-mode-btn', { active: measureMode === 'points' }]"
          @click="setMeasureMode('points')"
        >{{ $t("threejs.measurePoints") }}</button>
        <button
          type="button"
          :class="['measure-mode-btn', { active: measureMode === 'faces' }]"
          @click="setMeasureMode('faces')"
        >{{ $t("threejs.measureFaces") }}</button>
      </div>
      <div v-if="measuring && hint" class="measure-hint">{{ hint }}</div>

      <ViewCube :get-orientation="cameraOrientation" @select="setView" />
    </template>

    <aside v-if="showBom" class="cad-bom-panel">
      <header class="cad-bom-header">
        <h2>{{ $t("cadView.bom") }}</h2>
        <div class="cad-bom-actions">
          <button
            v-if="bomCsv"
            type="button"
            class="cad-bom-icon"
            :title="$t('cadView.exportBom')"
            :aria-label="$t('cadView.exportBom')"
            @click="exportBom"
          ><i class="material-symbols-outlined">download</i></button>
          <button
            type="button"
            class="cad-bom-icon"
            :title="$t('general.close')"
            :aria-label="$t('general.close')"
            @click="showBom = false"
          ><i class="material-symbols-outlined">close</i></button>
        </div>
      </header>
      <div v-if="bomRows.length" class="cad-bom-table-wrap">
        <table class="cad-bom-table">
          <thead>
            <tr>
              <th>{{ $t("cadView.part") }}</th>
              <th>{{ $t("cadView.configuration") }}</th>
              <th>{{ $t("cadView.quantity") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in bomRows" :key="`${row.file}:${row.configuration}`" :title="row.instances.join(', ')">
              <td>{{ row.file }}</td>
              <td>{{ row.configuration || "—" }}</td>
              <td class="cad-bom-quantity">{{ row.quantity }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="cad-bom-empty">{{ $t("cadView.bomEmpty") }}</p>
    </aside>
  </div>
</template>

<script>
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ViewCube from "@/components/files/ViewCube.vue";
import { state, mutations, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { resourcesApi } from "@/api";
import { fetchPreviewImage } from "@/utils/previewRequests.js";
import { ensureViewToken, getCachedViewToken, requestViewIdentity } from "@/api/viewToken";
import { loadCad3D, loadCadBRep, loadCadProperties } from "@/utils/cadService.ts";
import { CadWebGLRenderer } from "@/utils/cadWebGLRenderer.js";

export default {
  name: "cadViewer",
  components: { LoadingSpinner, ViewCube },
  props: {
    fbdata: { type: Object, required: true },
  },
  data() {
    return {
      loading: true,
      error: null,
      cadStatusMessage: "",
      brepLoading: false,
      brepLoaded: false,
      brepError: "",
      bomRows: [],
      bomCsv: "",
      showBom: false,
      previewBgUrl: null,
      previewAbort: null,
      displayMode: (() => {
        try {
          const saved = localStorage.getItem("threejsDisplayMode");
          return saved === "standard" ? "standard" : "occt";
        } catch {
          return "occt";
        }
      })(),
      // Mesh bookkeeping for measurement and the properties fallback.
      positions: null,
      cadFaces: [], // { start, count, nom, corps, area }
      rayon: 1,
      storedProperties: null,
      selectedFace: -1,
      // Measure state: picks are { point } or { range, point }.
      measuring: false,
      measureMode: "faces",
      measurePicks: [],
      measureLabels: [], // { point: [x,y,z], text }
      measureLines: [], // { a: [x,y,z], b: [x,y,z] }
      labelBoxes: [],
    };
  },
  computed: {
    hint() {
      if (!this.measuring) return "";
      if (this.measureMode === "points") {
        return this.measurePicks.length === 0
          ? this.$t("threejs.measureHintP1")
          : this.$t("threejs.measureHintP2");
      }
      return this.measurePicks.length === 0
        ? this.$t("threejs.measureHintF1")
        : this.$t("threejs.measureHintF2");
    },
    statusText() {
      if (this.selectedFace >= 0 && this.cadFaces[this.selectedFace]) {
        const f = this.cadFaces[this.selectedFace];
        return `${f.nom || "?"} · ${this.$t("cadView.faceInfo", { body: f.corps, tri: f.count })}`;
      }
      return "";
    },
  },
  async mounted() {
    eventBus.on("cadViewerAction", this.handleCadViewerAction);
    this.updateSelectedState();
    this.loadPreviewBackground();
    try {
      await this.loadModel();
    } catch (err) {
      this.handleError(err);
    }
  },
  beforeUnmount() {
    eventBus.off("cadViewerAction", this.handleCadViewerAction);
    this.clearPreviewBackground();
    this.renderer?.dispose();
    this.renderer = null;
  },
  methods: {
    updateSelectedState() {
      mutations.resetSelected();
      mutations.addSelected({
        name: this.fbdata.name,
        path: this.fbdata.path,
        size: this.fbdata.size,
        type: this.fbdata.type,
        source: this.fbdata.source,
        modified: this.fbdata.modified,
        created: this.fbdata.created,
        hasPreview: this.fbdata.hasPreview,
      });
    },

    handleError(err) {
      console.error("Failed to load CAD model", err);
      this.error = err.message || String(err);
      this.loading = false;
    },

    /** View URL for the raw file via viewToken (non-metered). */
    async modelUrl() {
      const typeHint = this.fbdata.name;
      if (getters.isShare()) {
        const identity = requestViewIdentity(state.req);
        const token = await ensureViewToken(this.fbdata.source);
        if (token && requestViewIdentity(state.req) === identity) {
          mutations.setRequestViewToken(token);
        }
        return resourcesApi.getViewURL(
          this.fbdata.source,
          this.fbdata.path,
          this.fbdata.viewToken || getCachedViewToken(this.fbdata.source) || "",
          { path: state.shareInfo.subPath, hash: state.shareInfo.hash },
          false,
          typeHint
        );
      }
      const identity = requestViewIdentity(state.req);
      let token = this.fbdata.viewToken || getCachedViewToken(this.fbdata.source);
      if (!token) {
        token = await ensureViewToken(this.fbdata.source);
        if (token && requestViewIdentity(state.req) === identity) {
          mutations.setRequestViewToken(token);
        }
      }
      return resourcesApi.getViewURL(this.fbdata.source, this.fbdata.path, token || "", null, false, typeHint);
    },

    /** The embedded preview image behind the spinner while the CAD pipeline
     * churns. Silent on failure. */
    async loadPreviewBackground() {
      if (!this.fbdata.hasPreview) return;
      try {
        const url = resourcesApi.getPreviewURL(this.fbdata.source, this.fbdata.path, this.fbdata.modified);
        this.previewAbort = new AbortController();
        this.previewBgUrl = await fetchPreviewImage(url, this.previewAbort.signal);
      } catch {
        // no background
      }
    },

    clearPreviewBackground() {
      this.previewAbort?.abort();
      this.previewAbort = null;
      if (this.previewBgUrl) {
        URL.revokeObjectURL(this.previewBgUrl);
        this.previewBgUrl = null;
      }
    },

    async loadModel() {
      this.loading = true;
      this.cadStatusMessage = this.$t("general.loading", { suffix: "..." });
      const url = await this.modelUrl();
      if (!url) throw new Error("No view token available for model");
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      const buffer = await response.arrayBuffer();

      const result = await loadCad3D(buffer, this.fbdata.name, (msg) => {
        this.cadStatusMessage = msg;
      });
      this.bomRows = result.bom || [];
      this.bomCsv = result.bomCsv || "";
      // SolidWorks mass properties are stored in the document; ask the Go
      // OpenSW reader for them (they are never recomputed from the mesh).
      try {
        const stored = await loadCadProperties(buffer, this.fbdata.name, (msg) => {
          this.cadStatusMessage = msg;
        });
        this.storedProperties = stored.properties || [];
      } catch {
        this.storedProperties = [];
      }

      this.applyCadMesh(result.mesh);
      this.brepLoaded = result.displayMesh !== true;
      this.clearPreviewBackground();
      this.loading = false;
    },

    async loadExactBrep() {
      if (this.brepLoading || this.brepLoaded) return;
      this.brepLoading = true;
      this.brepError = "";
      try {
        const url = await this.modelUrl();
        if (!url) throw new Error("No view token available for model");
        const response = await fetch(url);
        if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        const buffer = await response.arrayBuffer();
        const result = await loadCadBRep(buffer, this.fbdata.name, (msg) => {
          this.cadStatusMessage = msg;
        });
        this.error = null;
        this.applyCadMesh(result.mesh);
        this.brepLoaded = true;
      } catch (err) {
        this.brepError = err.message || String(err);
      } finally {
        this.brepLoading = false;
        this.cadStatusMessage = "";
      }
    },

    applyCadMesh(mesh) {
      this.positions = new Float32Array(mesh.sommets);
      this.rayon = mesh.rayon || 1;
      let tri = 0;
      this.cadFaces = (mesh.faces || []).map((f) => {
        const range = {
          start: tri,
          count: f.triangles || 0,
          nom: f.nom || "",
          corps: f.corps || 0,
          area: 0,
        };
        tri += range.count;
        return range;
      });
      this.computeFaceAreas();
      if (this.renderer?.ok) {
        this.renderer.setMesh(this.positions, mesh.faces || [], this.rayon);
        this.applyStyle();
      } else {
        this.initRenderer(mesh.faces || []);
      }
    },

    computeFaceAreas() {
      const p = this.positions;
      if (!p) return;
      for (const range of this.cadFaces) {
        let area = 0;
        for (let t = 0; t < range.count; t++) {
          const at = (range.start + t) * 9;
          if (at + 8 >= p.length) break;
          const ux = p[at + 3] - p[at], uy = p[at + 4] - p[at + 1], uz = p[at + 5] - p[at + 2];
          const wx = p[at + 6] - p[at], wy = p[at + 7] - p[at + 1], wz = p[at + 8] - p[at + 2];
          area += 0.5 * Math.hypot(uy * wz - uz * wy, uz * wx - ux * wz, ux * wy - uy * wx);
        }
        range.area = area;
      }
    },

    initRenderer(faces) {
      const canvas = this.$refs.glcanvas;
      this.renderer = new CadWebGLRenderer(canvas, this.$refs.triedre, {
        onPick: (faceIndex) => this.onFacePicked(faceIndex),
        onDraw: () => this.updateOverlay(),
      });
      if (!this.renderer.ok) {
        this.error = this.$t("cadView.noWebgl");
        return;
      }
      this.renderer.setMesh(this.positions, faces || [], this.rayon);
      this.applyStyle();
    },

    applyStyle() {
      if (!this.renderer) return;
      const occt = this.displayMode === "occt";
      this.renderer.setStyle({ teinte: occt ? 1 : 0, aretes: occt });
    },

    setDisplayMode(mode) {
      if (this.displayMode === mode) return;
      this.displayMode = mode;
      try {
        localStorage.setItem("threejsDisplayMode", mode);
      } catch {
        // storage unavailable: keep the choice for this session only
      }
      this.applyStyle();
    },

    // --- picking, selection, measurement --------------------------------

    onFacePicked(faceIndex) {
      if (faceIndex < 0) {
        this.selectedFace = -1;
        this.renderer?.select(-1);
        return;
      }
      if (this.measuring) return; // clicks are routed through the measure handler
      this.selectedFace = faceIndex;
      this.renderer?.select(faceIndex);
    },

    /** Canvas clicks while measuring: an exact pick through the face-index
     * buffer (faces) or the ortho ray (points). */
    onCanvasMeasurePick(faceIndex, event) {
      if (this.measureMode === "points") {
        this.pickPoint(event);
        this.updateOverlay();
        return;
      }
      this.pickFace(faceIndex, event);
      this.updateOverlay();
    },

    pickPoint(event) {
      const point = this.renderer.pointSous(event);
      if (!point) return;
      if (this.measurePicks.length >= 2) this.clearMeasure();
      this.measurePicks.push({ point });
      if (this.measurePicks.length === 2) {
        const [a, b] = this.measurePicks.map((x) => x.point);
        const d = Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);
        this.pushMeasure(a, b, `${this.$t("threejs.measureDist")}: ${d.toFixed(2)} mm`);
      }
    },

    pickFace(faceIndex, event) {
      const range = this.cadFaces[faceIndex];
      if (!range) return;
      if (this.measurePicks.length >= 2) this.clearMeasure();
      // Picking the same face again re-selects it instead of measuring 0 mm.
      if (this.measurePicks.length === 1 && this.measurePicks[0].range === range) return;
      const point = this.renderer.pointSous(event) || [0, 0, 0];
      this.measurePicks.push({ range, point });
      this.renderer.select(faceIndex);
      if (this.measurePicks.length === 1) {
        this.measureLabels.push({
          point,
          text: `${this.$t("threejs.measureArea")}: ${(range.area / 100).toFixed(2)} cm²`,
        });
      } else {
        const { dist, pa, pb } = this.minFaceDistance(this.measurePicks[0].range, this.measurePicks[1].range);
        this.pushMeasure(pa, pb, `${this.$t("threejs.measureDist")}: ${dist.toFixed(2)} mm`);
      }
    },

    pushMeasure(a, b, text) {
      this.measureLines.push({ a, b });
      this.measureLabels.push({
        point: [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2],
        text,
      });
    },

    /** Minimum distance between two BRep faces, approximated over their
     * triangle soups (vertices of one against triangles of the other, both
     * ways). Dense faces are sampled to keep it interactive. */
    minFaceDistance(ra, rb) {
      const p = this.positions;
      const triAt = (t) => {
        const at = t * 9;
        return [
          [p[at], p[at + 1], p[at + 2]],
          [p[at + 3], p[at + 4], p[at + 5]],
          [p[at + 6], p[at + 7], p[at + 8]],
        ];
      };
      const tris = (r) => {
        const stride = r.count > 600 ? Math.ceil(r.count / 600) : 1;
        const out = [];
        for (let t = 0; t < r.count; t += stride) out.push(triAt(r.start + t));
        return out;
      };
      const ta = tris(ra);
      const tb = tris(rb);
      let best = { dist: Infinity, pa: null, pb: null };
      const consider = (pt, tri) => {
        const d = this.pointTriangleDistance(pt, tri);
        if (d < best.dist) best = { dist: d, pa: pt, pb: this.triClosest };
      };
      for (const tri of ta) for (const v of tri) for (const other of tb) consider(v, other);
      for (const tri of tb) for (const v of tri) for (const other of ta) consider(v, other);
      if (!best.pa) {
        const pa = this.measurePicks[0].point;
        const pb = this.measurePicks[1].point;
        return { dist: Math.hypot(pa[0] - pb[0], pa[1] - pb[1], pa[2] - pb[2]), pa, pb };
      }
      return best;
    },

    /** Closest point on triangle tri to p; the point is left in
     * this.triClosest. */
    pointTriangleDistance(p, tri) {
      const [a, b, c] = tri;
      const sub = (u, v) => [u[0] - v[0], u[1] - v[1], u[2] - v[2]];
      const dot = (u, v) => u[0] * v[0] + u[1] * v[1] + u[2] * v[2];
      const ab = sub(b, a), ac = sub(c, a), ap = sub(p, a);
      const d1 = dot(ab, ap), d2 = dot(ac, ap);
      if (d1 <= 0 && d2 <= 0) { this.triClosest = a; return Math.hypot(...sub(p, a)); }
      const bp = sub(p, b);
      const d3 = dot(ab, bp), d4 = dot(ac, bp);
      if (d3 >= 0 && d4 <= d3) { this.triClosest = b; return Math.hypot(...sub(p, b)); }
      const vc = d1 * d4 - d3 * d2;
      if (vc <= 0 && d1 >= 0 && d3 <= 0) {
        const t = d1 / (d1 - d3);
        const q = [a[0] + t * ab[0], a[1] + t * ab[1], a[2] + t * ab[2]];
        this.triClosest = q;
        return Math.hypot(...sub(p, q));
      }
      const cp = sub(p, c);
      const d5 = dot(ab, cp), d6 = dot(ac, cp);
      if (d6 >= 0 && d5 <= d6) { this.triClosest = c; return Math.hypot(...sub(p, c)); }
      const vb = d5 * d2 - d1 * d6;
      if (vb <= 0 && d2 >= 0 && d6 <= 0) {
        const t = d2 / (d2 - d6);
        const q = [a[0] + t * ac[0], a[1] + t * ac[1], a[2] + t * ac[2]];
        this.triClosest = q;
        return Math.hypot(...sub(p, q));
      }
      const va = d3 * d6 - d5 * d4;
      if (va <= 0 && d4 - d3 >= 0 && d5 - d6 >= 0) {
        const t = (d4 - d3) / (d4 - d3 + (d5 - d6));
        const bc = sub(c, b);
        const q = [b[0] + t * bc[0], b[1] + t * bc[1], b[2] + t * bc[2]];
        this.triClosest = q;
        return Math.hypot(...sub(p, q));
      }
      const n = [ab[1] * ac[2] - ab[2] * ac[1], ab[2] * ac[0] - ab[0] * ac[2], ab[0] * ac[1] - ab[1] * ac[0]];
      const nn = dot(n, n) || 1;
      const t = dot(n, sub(p, a)) / Math.sqrt(nn);
      const u = n.map((x) => x / Math.sqrt(nn));
      this.triClosest = [p[0] - t * u[0], p[1] - t * u[1], p[2] - t * u[2]];
      return Math.abs(t);
    },

    toggleMeasure() {
      this.measuring = !this.measuring;
      if (this.measuring) {
        const pick = this.renderer ? (faceIndex, e) => this.onCanvasMeasurePick(faceIndex, e) : null;
        if (this.renderer && pick) this.renderer.opts.onPick = pick;
      } else {
        if (this.renderer) this.renderer.opts.onPick = (faceIndex) => this.onFacePicked(faceIndex);
        this.clearMeasure();
      }
    },

    setMeasureMode(mode) {
      this.measureMode = mode;
      this.clearMeasure();
    },

    clearMeasure() {
      this.measurePicks = [];
      this.measureLabels = [];
      this.measureLines = [];
      this.labelBoxes = [];
      this.renderer?.select(-1);
      this.updateOverlay();
    },

    /** Redraw the measure dimension lines and reproject the labels; called
     * from the renderer after each frame so they track the camera. */
    updateOverlay() {
      const canvas = this.$refs.overlay;
      if (!canvas) return;
      const r = window.devicePixelRatio || 1;
      const w = Math.round(canvas.clientWidth * r);
      const h = Math.round(canvas.clientHeight * r);
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      const g = canvas.getContext("2d");
      g.setTransform(r, 0, 0, r, 0, 0);
      g.clearRect(0, 0, canvas.clientWidth, canvas.clientHeight);
      if (!this.renderer?.ok) return;
      g.strokeStyle = "#ffd54f";
      g.lineWidth = 1.5;
      for (const line of this.measureLines) {
        const a = this.renderer.ecran(line.a);
        const b = this.renderer.ecran(line.b);
        g.beginPath();
        g.moveTo(a.x, a.y);
        g.lineTo(b.x, b.y);
        g.stroke();
        for (const pt of [a, b]) {
          g.fillStyle = "#ffd54f";
          g.beginPath();
          g.arc(pt.x, pt.y, 3, 0, Math.PI * 2);
          g.fill();
        }
      }
      this.labelBoxes = this.measureLabels.map((label) => {
        const s = this.renderer.ecran(label.point);
        return { x: s.x, y: s.y, text: label.text, visible: true };
      });
    },

    // --- view cube --------------------------------------------------------

    cameraOrientation() {
      return this.renderer?.ok
        ? this.renderer.orientation()
        : { right: [1, 0, 0], up: [0, 1, 0], back: [0, 0, 1] };
    },

    setView(view) {
      this.renderer?.orienter(view);
    },

    // --- prompts ------------------------------------------------------------

    handleCadViewerAction(payload) {
      if (!payload || payload.source !== this.fbdata.source || payload.path !== this.fbdata.path) return;
      if (payload.action === "properties") this.showCadProperties();
      else if (payload.action === "measure") this.toggleMeasure();
      else if (payload.action === "convert") this.showCadConvertPrompt();
      else if (payload.action === "brep") this.loadExactBrep();
      else if (payload.action === "bom") this.showBom = !this.showBom;
    },

    showCadConvertPrompt() {
      mutations.showPrompt({
        name: "CadConvert",
        props: { item: this.fbdata },
      });
    },

    exportBom() {
      if (!this.bomCsv) return;
      const blob = new Blob(["\uFEFF", this.bomCsv], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `${(this.fbdata.name || "assembly").replace(/\.sldasm$/i, "")}_BOM.csv`;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    },

    /** Mesh-derived properties as a fallback when SolidWorks stored none:
     * volume (divergence theorem), surface area, volume-weighted centre of
     * mass and the bounding box, all in mm. */
    computeCadProps() {
      const out = { volume: null, area: null, center: null, bbox: "—", triangles: 0, faces: this.cadFaces.length };
      const p = this.positions;
      if (!p) return out;
      let vol6 = 0;
      let area = 0;
      const com = [0, 0, 0];
      const tris = Math.floor(p.length / 9);
      out.triangles = tris;
      let lo = [Infinity, Infinity, Infinity];
      let hi = [-Infinity, -Infinity, -Infinity];
      for (let t = 0; t < tris; t++) {
        const at = t * 9;
        const ax = p[at], ay = p[at + 1], az = p[at + 2];
        const bx = p[at + 3], by = p[at + 4], bz = p[at + 5];
        const cx = p[at + 6], cy = p[at + 7], cz = p[at + 8];
        const v6 = ax * (by * cz - bz * cy) + ay * (bz * cx - bx * cz) + az * (bx * cy - by * cx);
        vol6 += v6;
        const ux = bx - ax, uy = by - ay, uz = bz - az;
        const wx = cx - ax, wy = cy - ay, wz = cz - az;
        area += 0.5 * Math.hypot(uy * wz - uz * wy, uz * wx - ux * wz, ux * wy - uy * wx);
        com[0] += (ax + bx + cx) * v6;
        com[1] += (ay + by + cy) * v6;
        com[2] += (az + bz + cz) * v6;
        for (const [x, y, z] of [[ax, ay, az], [bx, by, bz], [cx, cy, cz]]) {
          lo = [Math.min(lo[0], x), Math.min(lo[1], y), Math.min(lo[2], z)];
          hi = [Math.max(hi[0], x), Math.max(hi[1], y), Math.max(hi[2], z)];
        }
      }
      out.volume = Math.abs(vol6) / 6; // mm³
      out.area = area; // mm²
      if (Math.abs(vol6) > 1e-9) {
        // Tetra centroid = (a+b+c+origin)/4 and volume = v6/6, so the
        // moment Σ(a+b+c)·v6 divides by 4·Σv6 (not 3·Σv6).
        out.center = [com[0], com[1], com[2]].map((v) => v / (4 * vol6));
      } else {
        out.center = [(lo[0] + hi[0]) / 2, (lo[1] + hi[1]) / 2, (lo[2] + hi[2]) / 2];
      }
      out.bbox = `${(hi[0] - lo[0]).toFixed(1)} × ${(hi[1] - lo[1]).toFixed(1)} × ${(hi[2] - lo[2]).toFixed(1)} mm`;
      return out;
    },

    showCadProperties() {
      mutations.showPrompt({
        name: "CadProperties",
        props: {
          item: this.fbdata,
          propsData: this.computeCadProps(),
          storedProperties: this.storedProperties,
        },
      });
    },
  },
};
</script>

<style scoped>
.cadgl-viewer {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  background: var(--background);
}

/* OpenSW look: gradient background ("Top/Bottom Gradient Color"), the canvas
   itself stays transparent. */
.cadgl-viewer--occt {
  background: linear-gradient(#3f6fb5 0%, #b7cfec 100%);
}

.cadgl-canvas {
  display: block;
  width: 100%;
  height: 100%;
  position: absolute;
  inset: 0;
  touch-action: none;
}

.cadgl-triedre {
  position: absolute;
  left: 0.9rem;
  bottom: 0.9rem;
  width: 76px;
  height: 76px;
  pointer-events: none;
  z-index: 3;
}

.cadgl-overlay {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 4;
}

.measure-layer {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 5;
  overflow: hidden;
}

.measure-label {
  position: absolute;
  transform: translate(-50%, -130%);
  background: rgb(21 30 40 / 92%);
  color: #ffd54f;
  border: 1px solid rgb(255 213 79 / 55%);
  border-radius: 6px;
  padding: 2px 8px;
  font-size: 0.78rem;
  font-weight: 600;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.loading-overlay,
.error-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10;
}

.preview-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  opacity: 0.45;
}

.loading-front {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.cad-status-text {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: var(--textSecondary, #5f6368);
  font-weight: 500;
}

.error-content {
  text-align: center;
  padding: 2rem;
  max-width: 500px;
}

.error-content i {
  font-size: 64px;
  color: var(--red);
  margin-bottom: 1rem;
}

.error-content h3 {
  color: var(--textPrimary);
  margin-bottom: 0.5rem;
}

.error-content p {
  color: var(--textSecondary);
}

.mode-switcher {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: rgb(0 0 0 / 40%);
  backdrop-filter: blur(4px);
  z-index: 5;
}

.brep-feedback {
  position: absolute;
  top: 0.75rem;
  left: 0.75rem;
  z-index: 12;
  max-width: min(60vw, 32rem);
  margin: 0;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgb(255 255 255 / 20%);
  border-radius: 8px;
  color: #fff;
  background: rgb(15 23 32 / 82%);
  font-size: 0.78rem;
  backdrop-filter: blur(8px);
  box-shadow: 0 4px 16px rgb(0 0 0 / 18%);
}

.brep-feedback--error {
  color: #ffcdd2;
}

.mode-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  border: none;
  background: transparent;
  color: #fff;
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  opacity: 0.7;
  transition: background 0.15s ease, opacity 0.15s ease;
}

.mode-btn:hover {
  opacity: 0.95;
}

.mode-btn.active {
  background: rgb(255 255 255 / 25%);
  opacity: 1;
}

.mode-btn i {
  font-size: 18px;
}

@media (width <= 600px) {
  .mode-name {
    display: none;
  }
}

.status-bar {
  position: absolute;
  bottom: 0.75rem;
  left: 50%;
  transform: translateX(-50%);
  max-width: min(70%, 640px);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--textPrimary, #fff);
  background: rgb(0 0 0 / 40%);
  border-radius: 6px;
  padding: 3px 12px;
  font-size: 0.8rem;
  z-index: 5;
  pointer-events: none;
}

.measure-toolbar {
  position: absolute;
  top: 0.75rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: rgb(0 0 0 / 40%);
  backdrop-filter: blur(4px);
  z-index: 5;
}

.measure-mode-btn {
  border: none;
  background: transparent;
  color: #fff;
  padding: 4px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  opacity: 0.7;
}

.measure-mode-btn.active {
  background: rgb(255 255 255 / 25%);
  opacity: 1;
}

.measure-hint {
  position: absolute;
  top: 3.1rem;
  left: 50%;
  transform: translateX(-50%);
  color: #fff;
  background: rgb(0 0 0 / 45%);
  border-radius: 6px;
  padding: 3px 10px;
  font-size: 0.78rem;
  z-index: 5;
  pointer-events: none;
}

.cad-bom-panel {
  position: absolute;
  top: 3.6rem;
  right: 0.75rem;
  z-index: 8;
  display: flex;
  flex-direction: column;
  width: min(440px, calc(100% - 1.5rem));
  max-height: calc(100% - 6rem);
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--textPrimary) 18%, transparent);
  border-radius: 12px;
  color: var(--textPrimary);
  background: color-mix(in srgb, var(--background) 94%, transparent);
  backdrop-filter: blur(14px);
  box-shadow: 0 12px 36px rgb(0 0 0 / 24%);
}

.cad-bom-header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.65rem 0.8rem;
  border-bottom: 1px solid color-mix(in srgb, var(--textPrimary) 14%, transparent);
}

.cad-bom-header h2 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 650;
}

.cad-bom-actions {
  display: flex;
  gap: 0.25rem;
}

.cad-bom-icon {
  display: grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  border: 0;
  border-radius: 7px;
  color: inherit;
  background: transparent;
  cursor: pointer;
}

.cad-bom-icon:hover,
.cad-bom-icon:focus-visible {
  background: color-mix(in srgb, var(--textPrimary) 12%, transparent);
}

.cad-bom-table-wrap {
  overflow: auto;
}

.cad-bom-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}

.cad-bom-table th,
.cad-bom-table td {
  padding: 0.55rem 0.7rem;
  border-bottom: 1px solid color-mix(in srgb, var(--textPrimary) 10%, transparent);
  text-align: left;
}

.cad-bom-table th {
  position: sticky;
  top: 0;
  color: var(--textSecondary);
  background: var(--background);
  font-size: 0.7rem;
  font-weight: 650;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.cad-bom-table td:first-child {
  overflow-wrap: anywhere;
}

.cad-bom-quantity {
  width: 4.5rem;
  font-variant-numeric: tabular-nums;
  text-align: right !important;
}

.cad-bom-empty {
  margin: 0;
  padding: 1rem;
  color: var(--textSecondary);
  font-size: 0.85rem;
}

@media (width <= 600px) {
  .cad-bom-panel {
    right: 0.5rem;
    width: calc(100% - 1rem);
    max-height: calc(100% - 5rem);
  }
}
</style>
