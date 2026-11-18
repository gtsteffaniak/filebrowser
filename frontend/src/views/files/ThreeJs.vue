<template>
  <div class="threejs-viewer" ref="container">
    <div v-if="loading" class="loading-overlay">
      <img v-if="previewBgUrl" :src="previewBgUrl" class="preview-bg" alt="" />
      <div class="loading-front">
        <LoadingSpinner :size="isThumbnail ? 'small' : 'medium'" />
        <p v-if="!isThumbnail && cadStatusMessage" class="cad-status-text">{{ cadStatusMessage }}</p>
      </div>
    </div>
    <div v-if="error" class="error-overlay">
      <div v-if="isThumbnail" class="error-icon">
        <i class="material-symbols">view_in_ar</i>
      </div>
      <div v-else class="error-content">
        <i class="material-symbols-outlined">error</i>
        <h3>{{ $t("threejs.failedToLoad") }}</h3>
        <p>{{ error }}</p>
      </div>
    </div>
    <FloatingActionButton
      v-if="!isThumbnail && isCad"
      icon="change_circle"
      position="bottom-left"
      :style="{ bottom: '5.2rem' }"
      :auto-hide="false"
      @click="showCadConvertPrompt"
      :label="$t('cad.convert')"
    />
    <FloatingActionButton
      v-if="!isThumbnail"
      icon="settings"
      position="bottom-left"
      :auto-hide="false"
      @click="showControlsPrompt"
      :label="$t('threejs.controls')"
    />
    <div v-if="!isThumbnail && !error && !loading" class="mode-switcher" :title="$t('threejs.viewMode')">
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
    <FloatingActionButton
      v-if="!isThumbnail && !error && isCad"
      icon="analytics"
      position="bottom-left"
      :style="{ bottom: '10.4rem' }"
      :auto-hide="false"
      @click="showCadProperties"
      :label="$t('threejs.properties')"
    />
    <FloatingActionButton
      v-if="!isThumbnail && !error && isCad"
      :icon="measuring ? 'close' : 'architecture'"
      position="bottom-left"
      :style="{ bottom: '15.6rem' }"
      :auto-hide="false"
      @click="toggleMeasure"
      :label="measuring ? $t('threejs.measureExit') : $t('threejs.measure')"
    />
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
    <div class="measure-layer">
      <div
        v-for="(label, i) in measureLabels"
        :key="`ml${i}`"
        class="measure-label"
        :style="label.style"
      >{{ label.text }}</div>
    </div>
    <ViewCube
      v-if="!isThumbnail && !error && !loading && model"
      :get-direction="cameraDirection"
      @select="setView"
    />
  </div>
</template>

<script>
import { markRaw } from 'vue';
import {
  Scene,
  PerspectiveCamera,
  WebGLRenderer,
  AmbientLight,
  DirectionalLight,
  Color,
  LoadingManager,
  Box3,
  Vector3,
  Vector2,
  Raycaster,
  AnimationMixer,
  Timer,
  Mesh,
  Points,
  MeshStandardMaterial,
  PointsMaterial,
  DoubleSide,
  BufferGeometry,
  BufferAttribute,
  EdgesGeometry,
  LineSegments,
  LineBasicMaterial,
  Line,
  CanvasTexture,
  SRGBColorSpace,
} from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';
import { MTLLoader } from 'three/addons/loaders/MTLLoader.js';
import { FBXLoader } from 'three/addons/loaders/FBXLoader.js';
import { STLLoader } from 'three/addons/loaders/STLLoader.js';
import { PLYLoader } from 'three/addons/loaders/PLYLoader.js';
import { ColladaLoader } from 'three/addons/loaders/ColladaLoader.js';
import { ThreeMFLoader } from 'three/addons/loaders/3MFLoader.js';
import { TDSLoader } from 'three/addons/loaders/TDSLoader.js';
import { USDZLoader } from 'three/addons/loaders/USDZLoader.js';
import { USDLoader } from 'three/addons/loaders/USDLoader.js';
import { AMFLoader } from 'three/addons/loaders/AMFLoader.js';
import { VRMLLoader } from 'three/addons/loaders/VRMLLoader.js';
import { VTKLoader } from 'three/addons/loaders/VTKLoader.js';
import { PCDLoader } from 'three/addons/loaders/PCDLoader.js';
import { XYZLoader } from 'three/addons/loaders/XYZLoader.js';
import { VOXLoader } from 'three/addons/loaders/VOXLoader.js';
import { KMZLoader } from 'three/addons/loaders/KMZLoader.js';
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import ViewCube from "@/components/files/ViewCube.vue";
import { state, mutations, getters } from "@/store";
import { resourcesApi } from "@/api";
import { fetchPreviewImage } from "@/utils/previewRequests.js";
import { getCachedViewToken, ensureViewToken, requestViewIdentity } from "@/api/viewToken";
import { removeLastDir, resolveRelativePath } from "@/utils/url";
import { getObjectProperty } from '@/utils/object.js';
import { isCad3DFile, loadCad3D, loadCadProperties } from "@/utils/cadService.ts";

const LOADERS = {
  gltf: GLTFLoader,
  glb: GLTFLoader,
  obj: OBJLoader,
  fbx: FBXLoader,
  stl: STLLoader,
  ply: PLYLoader,
  dae: ColladaLoader,
  '3mf': ThreeMFLoader,
  '3ds': TDSLoader,
  usdz: USDZLoader,
  usd: USDLoader,
  usda: USDLoader,
  usdc: USDLoader,
  amf: AMFLoader,
  vrml: VRMLLoader,
  wrl: VRMLLoader,
  vtk: VTKLoader,
  vtp: VTKLoader,
  pcd: PCDLoader,
  xyz: XYZLoader,
  vox: VOXLoader,
  kmz: KMZLoader,
};

export default {
  name: "threeJsViewer",
  components: { LoadingSpinner, FloatingActionButton, ViewCube },
  props: {
    fbdata: { type: Object, required: true },
    isThumbnail: { type: Boolean, default: false },
    /** When true (e.g. listing icons), add extra delay (1750ms) so total is 2s. Avoids loading when user skips past. */
    addLoadDelay: { type: Boolean, default: false },
  },
  data() {
    return {
      scene: null,
      camera: null,
      renderer: null,
      controls: null,
      model: null,
      loading: true,
      error: null,
      animationFrameId: null,
      keyboardHandler: null,
      initialCameraPosition: null,
      initialControlsTarget: null,
      backgroundColor: '#000000',
      resizeObserver: null,
      animationMixer: null,
      animations: [],
      isAnimationPlaying: false,
      isAutoRotating: false,
      timer: null,
      observer: null,
      loadTimer: null,
      hasInitialized: false,
      isInView: false,
      viewTokenByPath: {},
      fetchedDirs: new Set(),
      lastRender: 0,
      cadStatusMessage: '',
      previewBgUrl: null,
      previewAbort: null,
      // CAD measure state: picks, labels, highlight and connector line.
      cadFaces: [],
      cadStoredProperties: null,
      measuring: false,
      measureMode: 'points',
      measurePicks: [],
      measureLabels: [],
      measureLine: null,
      highlightMesh: null,
      raycaster: null,
      pointerDownAt: null,
      displayMode: (() => {
        try {
          const saved = localStorage.getItem('threejsDisplayMode');
          // 'opensw' was renamed to the OpenCASCADE mode ('occt').
          return saved === 'opensw' || saved === 'occt' ? 'occt' : 'standard';
        } catch {
          return 'standard';
        }
      })(),
    };
  },
  computed: {
    isMobile() { return getters.isMobile(); },
    hasAnimations() { return this.animations && this.animations.length > 0; },
    isCad() {
      return isCad3DFile(this.fbdata.name);
    },
    modelUrl() {
      return this.resourceViewUrl(this.fbdata.path);
    },
    fileExtension() {
      return this.fbdata.name ? this.fbdata.name.split('.').pop().toLowerCase() : '';
    },
    hint() {
      if (!this.measuring) return '';
      if (this.measureMode === 'points') {
        return this.measurePicks.length === 0
          ? this.$t('threejs.measureHintP1')
          : this.$t('threejs.measureHintP2');
      }
      return this.measurePicks.length === 0
        ? this.$t('threejs.measureHintF1')
        : this.$t('threejs.measureHintF2');
    }
  },
  watch: {
    'fbdata.path'() { this.reinit(); },
  },
  mounted() {
    this.backgroundColor = '#000000';

    if (this.isThumbnail) {
      this.initIntersectionObserver();
    } else {
      // Full viewer: always 250ms base; add 1750ms when addLoadDelay (total 2s) to avoid loading when skipping
      const delay = 250 + (this.addLoadDelay ? 1750 : 0);
      this.loading = true;
      this.loadTimer = setTimeout(() => {
        this.initScene();
        this.loadModel();
        this.setupKeyboardShortcuts();
        this.updateSelectedState();
        this.loadTimer = null;
      }, delay);
    }

    window.addEventListener('resize', this.onWindowResize);
    if (this.$refs.container) {
      this.resizeObserver = new ResizeObserver(() => this.onWindowResize());
      this.resizeObserver.observe(this.$refs.container);
    }
  },
  beforeUnmount() {
    this.cleanup();
    this.clearPreviewBackground();
    window.removeEventListener('resize', this.onWindowResize);
    if (this.keyboardHandler) window.removeEventListener('keydown', this.keyboardHandler);
    if (this.resizeObserver) this.resizeObserver.disconnect();
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
    if (this.loadTimer) clearTimeout(this.loadTimer);
  },
  methods: {
    showControlsPrompt() {
      mutations.showPrompt({
        name: 'ThreeJSControls',
        props: {
          backgroundColor: this.backgroundColor,
          isMobile: this.isMobile,
          hasAnimations: this.hasAnimations,
          'onUpdate:backgroundColor': (newColor) => {
            this.backgroundColor = newColor;
            this.updateCustomBackground();
          },
        },
      });
    },
    showCadConvertPrompt() {
      mutations.showPrompt({
        name: 'CadConvert',
        props: { item: this.fbdata },
      });
    },
    getSpaceText() {
      return this.hasAnimations ? this.$t("general.playPause") : this.$t("threejs.autoRotate");
    },
    async initIntersectionObserver() {
      // Use a single global observer if possible, but for now localize config
      this.observer = new IntersectionObserver(this.handleIntersect, {
        root: null,
        rootMargin: "50px", // Reduced margin to avoid eager loading too many
        threshold: 0,
      });
      await this.$nextTick();
      if (this.observer && this.$el instanceof Element) this.observer.observe(this.$el);
    },

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

    handleIntersect(entries) {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          this.isInView = true;
          if (!this.hasInitialized && !this.loadTimer) {
            // Always 250ms base; add 1750ms when addLoadDelay (e.g. listing icons) for 2s total
            const delay = 250 + (this.addLoadDelay ? 1750 : 0);
            this.loadTimer = setTimeout(() => {
              if (this.isInView && !this.hasInitialized) {
                this.hasInitialized = true;
                this.initScene();
                void this.loadModel();
              }
              this.loadTimer = null;
            }, delay);
          }
        } else {
          this.isInView = false;
          if (this.loadTimer) {
            clearTimeout(this.loadTimer);
            this.loadTimer = null;
          }
          if (this.hasInitialized) {
            this.cleanup();
            this.hasInitialized = false;
            this.loading = true;
            this.error = null;
          }
        }
      }
    },

    initScene() {
      // Create scene
      this.scene = markRaw(new Scene());
      this.applyDisplayMode();
      this.timer = markRaw(new Timer());

      const container = this.$refs.container;
      const width = container.clientWidth;
      const height = container.clientHeight;

      this.camera = markRaw(new PerspectiveCamera(75, width / height, 0.1, 1000));
      this.camera.position.set(0, 0, 5);

      // OPTIMIZATION: Check if we can reuse a context or limit features
      // For thumbnails, we can use a simpler renderer configuration
      const rendererConfig = {
        antialias: !this.isThumbnail, // Disable antialiasing for thumbnails
        powerPreference: "high-performance",
        alpha: false, // We use a solid background
        depth: true,
        stencil: false,
      };

      this.renderer = markRaw(new WebGLRenderer(rendererConfig));
      const pixelRatioCap = this.isThumbnail ? 1 : 2;
      const pixelRatio = Math.min(window.devicePixelRatio, pixelRatioCap);
      this.renderer.setSize(Math.floor(width * pixelRatio), Math.floor(height * pixelRatio), false);
      this.renderer.domElement.style.width = "100%";
      this.renderer.domElement.style.height = "100%";
      container.appendChild(this.renderer.domElement);
      this.renderer.domElement.addEventListener('pointerdown', this.onPointerDown);
      this.renderer.domElement.addEventListener('click', this.onCanvasClick);

      // Lights - Simplify lighting for thumbnails
      this.scene.add(markRaw(new AmbientLight(0xffffff, 0.6)));
      const dirLight1 = markRaw(new DirectionalLight(0xffffff, 0.8));
      dirLight1.position.set(1, 2, 3);
      this.scene.add(dirLight1);

      if (!this.isThumbnail) {
        // Only add secondary lights for full view
        const dirLight2 = markRaw(new DirectionalLight(0xffffff, 0.4));
        dirLight2.position.set(-1, -2, -3);
        this.scene.add(dirLight2);
      }

      // Controls
      this.controls = markRaw(new OrbitControls(this.camera, this.renderer.domElement));
      this.controls.enableDamping = true;
      this.controls.dampingFactor = 0.05;

      if (this.isThumbnail) {
        this.controls.autoRotate = true;
        this.controls.autoRotateSpeed = 2.0;
        this.controls.enableZoom = false;
        this.controls.enablePan = false;
        this.controls.enableRotate = false;
        this.controls.enabled = false;
      } else {
        this.controls.autoRotate = false;
        this.controls.autoRotateSpeed = 2.0;
      }

      this.animate();
    },

    updateBackgroundColor() {
      if (this.scene) {
        this.scene.background = new Color(this.backgroundColor);
        this.updateMaterialColors();
      }
    },

    updateMaterialColors() {
      if (this.model?.isMesh && this.model.material) {
        if (['stl', 'ply', 'amf'].includes(this.fileExtension) && this.displayMode !== 'occt') {
          this.model.material.color.setHex(0x4fc3f7);
        }
      }
    },

    updateCustomBackground() {
      if (this.scene) {
        this.scene.background = new Color(this.backgroundColor);
      }
    },

    setDisplayMode(mode) {
      if (this.displayMode === mode) return;
      this.displayMode = mode;
      try {
        localStorage.setItem('threejsDisplayMode', mode);
      } catch {
        // Storage may be unavailable (private mode); selection stays per-session.
      }
      this.applyDisplayMode();
    },

    /** Apply the current display mode to background, part colours and edges.
     * In the OpenCASCADE mode the BRep face colours reported by OpenCASCADE
     * drive the render (vertex colours); edges get thin black outlines. */
    applyDisplayMode() {
      if (!this.scene) return;
      const occt = this.displayMode === 'occt' && !this.isThumbnail;
      this.scene.background = occt ? this.createOpenCASCADEBackground() : new Color(this.backgroundColor);
      if (!this.model) return;
      this.model.traverse((child) => {
        if (!child.isMesh || child.userData.isEdgeOverlay || child.userData.isMeasureOverlay) return;
        const material = child.material && !Array.isArray(child.material) ? child.material : null;
        if (occt) {
          if (material) {
            if (!('origColorHex' in child.userData)) {
              child.userData.origColorHex = material.color.getHex();
            }
            // Remember the model's own vertex-colour setting so Standard mode
            // and thumbnails keep it (e.g. GLTF vertex colours).
            if (!('origVertexColors' in child.userData)) {
              child.userData.origVertexColors = material.vertexColors === true;
            }
            if (child.userData.isCadMesh && child.geometry?.attributes?.color) {
              // OpenCASCADE view: each face keeps its own colour.
              material.vertexColors = true;
              material.color.setHex(0xffffff);
            } else {
              material.vertexColors = false;
              material.color.setHex(0xc9ccd1);
            }
            material.metalness = 0.08;
            material.roughness = 0.5;
            // Push faces back slightly so edge lines render crisply on top.
            material.polygonOffset = true;
            material.polygonOffsetFactor = 1;
            material.polygonOffsetUnits = 1;
            material.needsUpdate = true;
          }
          this.addEdgeOverlay(child);
        } else {
          if (material) {
            if ('origVertexColors' in child.userData) {
              material.vertexColors = child.userData.origVertexColors;
            }
            material.needsUpdate = true;
            if ('origColorHex' in child.userData) {
              material.color.setHex(child.userData.origColorHex);
            }
            material.polygonOffset = false;
          }
          this.removeEdgeOverlay(child);
        }
      });
    },

    /** OpenCASCADE-style vertical blue gradient. */
    createOpenCASCADEBackground() {
      if (!this._occtBgTexture) {
        const canvas = document.createElement('canvas');
        canvas.width = 2;
        canvas.height = 256;
        const ctx = canvas.getContext('2d');
        const gradient = ctx.createLinearGradient(0, 0, 0, 256);
        gradient.addColorStop(0, '#3f6fb5');
        gradient.addColorStop(1, '#b7cfec');
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, 2, 256);
        const texture = new CanvasTexture(canvas);
        texture.colorSpace = SRGBColorSpace;
        this._occtBgTexture = markRaw(texture);
      }
      return this._occtBgTexture;
    },

    /** Black feature edges over each face, like the OpenSW viewer. */
    addEdgeOverlay(mesh) {
      if (mesh.userData.edgeOverlay || !mesh.geometry?.attributes?.position) return;
      const edges = new LineSegments(
        new EdgesGeometry(mesh.geometry, 20),
        new LineBasicMaterial({ color: 0x0d0d0d }),
      );
      edges.userData.isEdgeOverlay = true;
      mesh.userData.edgeOverlay = edges;
      mesh.add(edges);
    },

    removeEdgeOverlay(mesh) {
      const edges = mesh.userData.edgeOverlay;
      if (!edges) return;
      edges.geometry.dispose();
      edges.material.dispose();
      mesh.remove(edges);
      mesh.userData.edgeOverlay = null;
    },

    handleError(err, prefix = "Failed to load model") {
      console.error(prefix, err);
      const msg = err.message || 'Unknown error';
      if (msg.includes('JSON') || msg.includes('Unexpected token')) {
        this.error = 'Invalid file format or corrupted file.';
      } else if (msg.includes('404') || msg.includes('not found')) {
        this.error = 'File not found.';
      } else {
        this.error = `${prefix}: ${msg}`;
      }
      this.loading = false;
      if (this.isThumbnail) {
        this.$emit('error', err);
      }
    },

    /** Show the file's embedded preview image behind the spinner while the
     * CAD pipeline (fetch + Pyodide + OpenCASCADE) churns. Silent on failure:
     * files without an embedded preview (STEP, Parasolid) just keep the plain
     * spinner. */
    async loadPreviewBackground() {
      if (this.isThumbnail || this.previewBgUrl || !this.fbdata.hasPreview) {
        return;
      }
      try {
        const url = resourcesApi.getPreviewURL(this.fbdata.source, this.fbdata.path, this.fbdata.modified);
        this.previewAbort = new AbortController();
        this.previewBgUrl = await fetchPreviewImage(url, this.previewAbort.signal);
      } catch {
        // Aborted, no preview available, or preview request failed: no background.
      }
    },

    clearPreviewBackground() {
      if (this.previewAbort) {
        this.previewAbort.abort();
        this.previewAbort = null;
      }
      if (this.previewBgUrl) {
        URL.revokeObjectURL(this.previewBgUrl);
        this.previewBgUrl = null;
      }
    },

    // --- view cube -----------------------------------------------------------

    cameraDirection() {
      if (!this.camera) return { x: 0, y: 0, z: -1 };
      return this.camera.getWorldDirection(new Vector3());
    },

    /** Fly the camera to a named orthographic-ish view. */
    setView(view) {
      if (!this.camera || !this.controls) return;
      const dirs = {
        top: new Vector3(0.0001, 1, 0.0001),
        bottom: new Vector3(0.0001, -1, 0.0001),
        right: new Vector3(1, 0, 0),
        left: new Vector3(-1, 0, 0),
        front: new Vector3(0, 0, 1),
        back: new Vector3(0, 0, -1),
        iso: new Vector3(1, 1, 1),
      };
      const dir = dirs[view];
      if (!dir) return;
      dir.normalize();
      const target = this.controls.target.clone();
      const dist = this.camera.position.distanceTo(target);
      const from = this.camera.position.clone();
      const to = target.clone().add(dir.multiplyScalar(dist));
      const t0 = performance.now();
      const dur = 350;
      const step = () => {
        if (!this.camera) return;
        const k = Math.min(1, (performance.now() - t0) / dur);
        const e = k < 0.5 ? 2 * k * k : 1 - (-2 * k + 2) ** 2 / 2;
        const pos = from.clone().lerp(to, e).sub(target).normalize().multiplyScalar(dist).add(target);
        this.camera.position.copy(pos);
        this.controls.update();
        if (k < 1) requestAnimationFrame(step);
      };
      requestAnimationFrame(step);
    },

    // --- measure tool ----------------------------------------------------------

    toggleMeasure() {
      this.measuring = !this.measuring;
      if (this.measuring) {
        this.raycaster = markRaw(new Raycaster());
        this.clearMeasureVisuals();
      } else {
        this.clearMeasureVisuals();
      }
    },

    setMeasureMode(mode) {
      this.measureMode = mode;
      this.clearMeasureVisuals();
    },

    clearMeasureVisuals() {
      this.measurePicks = [];
      this.measureLabels = [];
      if (this.measureLine && this.scene) {
        this.scene.remove(this.measureLine);
        this.measureLine.geometry.dispose();
        this.measureLine.material.dispose();
        this.measureLine = null;
      }
      if (this.highlightMesh?.parent) {
        this.highlightMesh.parent.remove(this.highlightMesh);
        this.highlightMesh.material.dispose();
        this.highlightMesh = null;
      }
    },

    onPointerDown(e) {
      this.pointerDownAt = { x: e.clientX, y: e.clientY };
    },

    /** A click on the canvas while measuring: pick a point or a face. */
    onCanvasClick(e) {
      if (!this.measuring || !this.camera || !this.model || !this.renderer) return;
      if (this.pointerDownAt &&
        Math.hypot(e.clientX - this.pointerDownAt.x, e.clientY - this.pointerDownAt.y) > 6) {
        return; // it was an orbit drag, not a pick
      }
      const rect = this.renderer.domElement.getBoundingClientRect();
      const ndc = new Vector2(
        ((e.clientX - rect.left) / rect.width) * 2 - 1,
        -((e.clientY - rect.top) / rect.height) * 2 + 1,
      );
      this.raycaster.setFromCamera(ndc, this.camera);
      const targets = [];
      this.model.traverse((c) => {
        if (c.isMesh && !c.userData.isEdgeOverlay && !c.userData.isMeasureOverlay) targets.push(c);
      });
      const hits = this.raycaster.intersectObjects(targets, false);
      if (!hits.length) return;
      if (this.measureMode === 'points') this.pickPoint(hits[0]);
      else this.pickFace(hits[0]);
    },

    pickPoint(hit) {
      if (this.measurePicks.length >= 2) this.clearMeasureVisuals();
      this.measurePicks.push({ point: hit.point.clone() });
      if (this.measurePicks.length === 2) {
        const [a, b] = this.measurePicks.map((p) => p.point);
        this.drawMeasureLine(a, b);
        this.pushMeasureLabel(
          a.clone().add(b).multiplyScalar(0.5),
          `${a.distanceTo(b).toFixed(2)} mm`,
        );
      }
    },

    pickFace(hit) {
      if (this.measurePicks.length >= 2) this.clearMeasureVisuals();
      const range = this.faceFromTriangle(hit.faceIndex);
      if (!range) return;
      // Picking the same face again re-selects it instead of measuring 0 mm.
      if (this.measurePicks.length === 1 && this.measurePicks[0].range === range) return;
      this.measurePicks.push({ range, point: hit.point.clone() });
      this.highlightFace(range);
      if (this.measurePicks.length === 1) {
        this.pushMeasureLabel(
          hit.point.clone(),
          `${this.$t('threejs.measureArea')}: ${(range.area / 100).toFixed(2)} cm²`,
        );
      } else {
        const [a, b] = this.measurePicks;
        const { dist, pa, pb } = this.minFaceDistance(a.range, b.range);
        this.drawMeasureLine(pa, pb);
        this.pushMeasureLabel(
          pa.clone().add(pb).multiplyScalar(0.5),
          `${this.$t('threejs.measureDist')}: ${dist.toFixed(2)} mm`,
        );
      }
    },

    cadSourceMesh() {
      let source = null;
      this.model?.traverse((c) => {
        if (c.isMesh && c.userData.isCadMesh && c.geometry?.attributes?.position) source = c;
      });
      return source;
    },

    faceFromTriangle(triangleIndex) {
      if (triangleIndex == null) return null;
      for (const range of this.cadFaces) {
        if (triangleIndex >= range.start && triangleIndex < range.start + range.count) {
          return range;
        }
      }
      return null;
    },

    highlightFace(range) {
      const source = this.cadSourceMesh();
      if (!source) return;
      if (this.highlightMesh?.parent) {
        this.highlightMesh.parent.remove(this.highlightMesh);
        this.highlightMesh.material.dispose();
      }
      const geometry = new BufferGeometry();
      geometry.setAttribute('position', source.geometry.attributes.position);
      geometry.setDrawRange(range.start * 3, range.count * 3);
      const material = new MeshStandardMaterial({
        color: 0xff9800,
        transparent: true,
        opacity: 0.45,
        side: DoubleSide,
      });
      this.highlightMesh = markRaw(new Mesh(geometry, material));
      this.highlightMesh.userData.isMeasureOverlay = true;
      this.highlightMesh.renderOrder = 2;
      source.add(this.highlightMesh); // inherits the model transform
    },

    drawMeasureLine(a, b) {
      const geometry = new BufferGeometry().setFromPoints([a, b]);
      this.measureLine = markRaw(new Line(
        geometry,
        new LineBasicMaterial({ color: 0xffd54f, depthTest: false }),
      ));
      this.measureLine.renderOrder = 3;
      this.scene.add(this.measureLine);
    },

    pushMeasureLabel(worldPoint, text) {
      this.measureLabels.push({ point: worldPoint, text, style: {} });
    },

    /** Reproject measure labels onto the screen; called from animate(). */
    updateMeasureLabels() {
      if (!this.camera || !this.$refs.container) return;
      const w = this.$refs.container.clientWidth;
      const h = this.$refs.container.clientHeight;
      for (const label of this.measureLabels) {
        const v = label.point.clone().project(this.camera);
        label.style.left = `${(v.x * 0.5 + 0.5) * w}px`;
        label.style.top = `${(-v.y * 0.5 + 0.5) * h}px`;
        label.style.visibility = v.z < 1 ? 'visible' : 'hidden';
      }
    },

    /** Minimum distance between two BRep faces, approximated over their
     * triangle soups (vertices of one against triangles of the other, both
     * ways). Dense faces are sampled to keep it interactive. */
    minFaceDistance(a, b) {
      const source = this.cadSourceMesh();
      const fallback = {
        dist: this.measurePicks[0].point.distanceTo(this.measurePicks[1].point),
        pa: this.measurePicks[0].point,
        pb: this.measurePicks[1].point,
      };
      if (!source) return fallback;
      const pos = source.geometry.attributes.position.array;
      const triAt = (t) => {
        const at = t * 9;
        return [
          [pos[at], pos[at + 1], pos[at + 2]],
          [pos[at + 3], pos[at + 4], pos[at + 5]],
          [pos[at + 6], pos[at + 7], pos[at + 8]],
        ];
      };
      const tris = (r) => {
        const stride = r.count > 600 ? Math.ceil(r.count / 600) : 1;
        const out = [];
        for (let t = 0; t < r.count; t += stride) out.push(triAt(r.start + t));
        return out;
      };
      const ta = tris(a);
      const tb = tris(b);
      let best = { dist: Infinity, pa: null, pb: null };
      const consider = (p, tri) => {
        const d = this.pointTriangleDistance(p, tri);
        if (d < best.dist) {
          best = { dist: d, pa: this.localToWorldPoint(source, p), pb: this.localToWorldPoint(source, this.triClosest) };
        }
      };
      for (const tri of ta) {
        for (const v of tri) {
          for (const other of tb) consider(v, other);
        }
      }
      for (const tri of tb) {
        for (const v of tri) {
          for (const other of ta) consider(v, other);
        }
      }
      if (!best.pa) return fallback;
      return best;
    },

    localToWorldPoint(mesh, p) {
      const v = new Vector3(p[0], p[1], p[2]);
      mesh.localToWorld(v);
      return v;
    },

    /** Squared-distance housekeeping: closest point on triangle tri to p.
     * The closest point is left in this.triClosest (local coords). */
    pointTriangleDistance(p, tri) {
      const [a, b, c] = tri;
      const sub = (u, v) => [u[0] - v[0], u[1] - v[1], u[2] - v[2]];
      const dot = (u, v) => u[0] * v[0] + u[1] * v[1] + u[2] * v[2];
      const cross = (u, v) => [
        u[1] * v[2] - u[2] * v[1],
        u[2] * v[0] - u[0] * v[2],
        u[0] * v[1] - u[1] * v[0],
      ];
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
      const n = cross(ab, ac);
      const nn = dot(n, n) || 1;
      const t = dot(n, sub(p, a)) / nn;
      const q = [p[0] - t * n[0] / Math.sqrt(nn), p[1] - t * n[1] / Math.sqrt(nn), p[2] - t * n[2] / Math.sqrt(nn)];
      this.triClosest = q;
      return Math.abs(t) * Math.sqrt(nn);
    },

    // --- properties panel --------------------------------------------------------

    /** Mesh-derived properties: volume (divergence theorem), surface area,
     * volume-weighted centre of mass and the bounding box, all in mm. */
    computeCadProps() {
      const out = { volume: null, area: null, center: null, bbox: '—', triangles: 0, faces: this.cadFaces.length };
      const source = this.cadSourceMesh();
      if (!source || !this.model) return out;
      const p = source.geometry.attributes.position.array;
      let vol6 = 0;
      let area = 0;
      const com = [0, 0, 0];
      const tris = Math.floor(p.length / 9);
      out.triangles = tris;
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
      }
      out.volume = Math.abs(vol6) / 6; // mm³
      out.area = area; // mm²
      if (Math.abs(vol6) > 1e-9) {
        // Tetra centroid = (a+b+c+origin)/4 and volume = v6/6, so the
        // moment Σ(a+b+c)·v6 divides by 4·Σv6 (not 3·Σv6).
        out.center = [com[0], com[1], com[2]].map((v) => v / (4 * vol6));
      } else {
        // Open shell: the centre of mass is not defined; show the bbox centre.
        const box = new Box3().setFromObject(this.model);
        const c = box.getCenter(new Vector3());
        out.center = [c.x, c.y, c.z];
      }
      const box = new Box3().setFromObject(this.model);
      const size = box.getSize(new Vector3());
      out.bbox = `${size.x.toFixed(1)} × ${size.y.toFixed(1)} × ${size.z.toFixed(1)} mm`;
      return out;
    },

    showCadProperties() {
      const propsData = this.computeCadProps();
      mutations.showPrompt({
        name: 'CadProperties',
        props: { item: this.fbdata, propsData, storedProperties: this.cadStoredProperties },
      });
    },

    /** View URL for a model or sibling asset via viewToken (non-metered). */
    resourceViewUrl(filePath) {
      if (!filePath) {
        return "";
      }
      const viewToken = this.resolveViewTokenForPath(filePath);
      if (!viewToken) {
        return "";
      }
      const typeHint = filePath.split("/").pop() || filePath;
      let url;
      if (getters.isShare()) {
        url = resourcesApi.getViewURL(
          this.fbdata.source,
          filePath,
          viewToken,
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          false,
          typeHint,
        );
      } else {
        url = resourcesApi.getViewURL(this.fbdata.source, filePath, viewToken, null, false, typeHint);
      }
      return url || "";
    },

    normalizeAssetPath(filePath) {
      if (!filePath) {
        return filePath;
      }
      const trimmed = filePath.replace(/\/+$/, "");
      return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
    },

    indexViewTokens(items) {
      if (!items?.length) {
        return;
      }
      for (const item of items) {
        if (item.path && item.viewToken) {
          this.viewTokenByPath[this.normalizeAssetPath(item.path)] = item.viewToken;
        }
      }
    },

    directoryListingPath(dirPath) {
      const normalized = this.normalizeAssetPath(dirPath);
      return `${normalized}/`;
    },

    async fetchDirectoryTokens(dirPath) {
      const listingPath = this.directoryListingPath(dirPath);
      if (this.fetchedDirs.has(listingPath)) {
        return;
      }
      this.fetchedDirs.add(listingPath);
      try {
        let listing;
        if (getters.isShare()) {
          listing = await resourcesApi.fetchFilesPublic(
            listingPath,
            state.shareInfo.hash,
            "",
            false,
            false,
          );
        } else {
          listing = await resourcesApi.fetchFiles(
            this.fbdata.source,
            listingPath,
            false,
            false,
          );
        }
        if (listing?.type === "directory" && listing.items?.length) {
          this.indexViewTokens(listing.items);
        }
      } catch {
        // Missing or inaccessible directories are ignored.
      }
    },

    async prefetchAssetViewTokens() {
      const viewIdentity = requestViewIdentity(state.req);
      try {
        const token = await ensureViewToken(this.fbdata.source);
        if (token && requestViewIdentity(state.req) === viewIdentity) {
          mutations.setRequestViewToken(token);
          if (this.fbdata.path) {
            this.viewTokenByPath[this.normalizeAssetPath(this.fbdata.path)] = token;
          }
        }
      } catch (err) {
        console.warn("Failed to refresh view token for 3D preview:", err);
      }
      this.indexViewTokens(this.fbdata.parentDirItems);
      if (this.fbdata.path && this.fbdata.viewToken) {
        this.viewTokenByPath[this.normalizeAssetPath(this.fbdata.path)] = this.fbdata.viewToken;
      }

      const modelDir = removeLastDir(this.fbdata.path);
      await this.fetchDirectoryTokens(modelDir);
      if (this.fbdata.parentDirItems?.some((item) => item.name === "textures" && item.type === "directory")) {
        await this.fetchDirectoryTokens(`${modelDir}/textures`);
      }
    },

    resolveViewTokenForPath(filePath) {
      const normalizedPath = this.normalizeAssetPath(filePath);
      const cachedToken = getObjectProperty(this.viewTokenByPath, normalizedPath);
      if (cachedToken) {
        return cachedToken;
      }
      if (
        this.normalizeAssetPath(this.fbdata.path) === normalizedPath &&
        this.fbdata.viewToken
      ) {
        return this.fbdata.viewToken;
      }
      const items = this.fbdata.parentDirItems;
      if (!items?.length) {
        return getCachedViewToken(this.fbdata.source);
      }

      const exact = items.find(
        (item) => this.normalizeAssetPath(item.path) === normalizedPath,
      );
      if (exact?.viewToken) {
        return exact.viewToken;
      }

      const fileDir = removeLastDir(normalizedPath);
      const name = normalizedPath.split("/").filter(Boolean).pop();
      if (!name) {
        return getCachedViewToken(this.fbdata.source);
      }

      const sibling = items.find((item) => {
        if (item.name !== name) {
          return false;
        }
        if (item.path) {
          return removeLastDir(this.normalizeAssetPath(item.path)) === fileDir;
        }
        return fileDir === removeLastDir(this.fbdata.path);
      });
      return sibling?.viewToken ?? getCachedViewToken(this.fbdata.source);
    },

    resolveTextureUrl(url) {
      if (!url) {
        return url;
      }
      if (url.startsWith("blob:") || url.startsWith("data:")) {
        return url;
      }
      if (
        url.includes("/api/media/stream?") ||
        url.includes("/api/resources/view?") ||
        url.includes("/api/resources/download?")
      ) {
        try {
          const parsed = new URL(url, window.origin);
          const filePath = parsed.searchParams.get("file") || parsed.searchParams.getAll("file")[0];
          if (filePath) {
            return this.resourceViewUrl(filePath);
          }
        } catch {
          return url;
        }
        return url;
      }

      const cleanUrl = url.split(/[?#]/)[0];
      let relativePath;
      if (cleanUrl.includes("/api/resources/")) {
        relativePath = cleanUrl.split("/api/resources/").pop() || "";
      } else if (/^https?:\/\//i.test(cleanUrl)) {
        return url;
      } else {
        relativePath = cleanUrl;
      }

      relativePath = relativePath.replace(/^\/+/, "");
      const filename = relativePath.split("/").pop();
      if (!filename) {
        return url;
      }

      const modelDir = removeLastDir(this.fbdata.path);
      let texturePath;
      if (relativePath.includes("/")) {
        texturePath = resolveRelativePath(this.fbdata.path, relativePath);
      } else if (this.fbdata.parentDirItems?.some((item) => item.name === filename)) {
        texturePath = `${modelDir}/${filename}`;
      } else {
        texturePath = `${modelDir}/textures/${filename}`;
      }
      return this.resourceViewUrl(texturePath);
    },

    async loadModel() {
      await this.prefetchAssetViewTokens();
      if (!this.modelUrl) {
        this.handleError(new Error("No view token available for model"), "Failed to load model");
        return;
      }
      this.loading = true;
      this.error = null;
      this.cadFaces = []; // rebuilt by buildCadObject for CAD loads

      try {
        const extension = this.fileExtension;
        if (this.isCad) {
          await this.loadCadModel(extension);
          return;
        }

        const LoaderClass = getObjectProperty(LOADERS, extension);
        if (!LoaderClass) throw new Error(`Unsupported 3D format: .${extension}`);

        const loadingManager = markRaw(new LoadingManager());
        loadingManager.onError = (url) => console.warn(`Error loading asset: ${url}`);
        loadingManager.setURLModifier((url) => this.resolveTextureUrl(url));

        const loader = new LoaderClass(loadingManager);

        // Special handlers
        if (extension === 'glb') {
          await this.loadGLB(loader);
        } else if (extension === 'obj') {
          this.loadOBJ(loader, loadingManager);
        } else {
          if (extension === 'dae') {
            const parse = loader.parse.bind(loader);
            loader.parse = (...args) => {
              const warn = console.warn;
              console.warn = (msg, ...rest) => {
                if (!String(msg).includes('Z-UP coordinate system')) warn(msg, ...rest);
              };
              try {
                return parse(...args);
              } finally {
                console.warn = warn;
              }
            };
          }
          // Standard load for all other formats including FBX
          loader.load(
            this.modelUrl,
            (data) => this.onModelLoaded(data, extension),
            this.onProgress,
            (err) => this.handleError(err)
          );
        }
      } catch (err) {
        this.handleError(err, "Error initializing loader");
      }
    },

    onProgress(xhr) {
      if (xhr.lengthComputable) {
        // Progress tracking available if needed in the future
        // const percent = Math.round((xhr.loaded / xhr.total) * 100);
      }
    },

    async loadGLB(loader) {
      try {
        const response = await fetch(this.modelUrl);
        if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        const data = await response.arrayBuffer();

        // Verify Magic Number
        const magic = new TextDecoder().decode(new Uint8Array(data, 0, 4));
        const modelDir = this.fbdata.path.substring(0, this.fbdata.path.lastIndexOf('/') + 1);

        if (magic !== 'glTF') {
          // Try text parse
          try {
             const text = new TextDecoder().decode(data);
             const json = JSON.parse(text);
             loader.parse(json, modelDir, (gltf) => this.onModelLoaded(gltf, 'gltf'), (err) => this.handleError(err, "Invalid GLB"));
          } catch (_e) {
             this.handleError(new Error("Invalid GLB magic number and not valid JSON"), "Corrupted File");
          }
          return;
        }

        loader.parse(
            data,
            modelDir,
            (gltf) => this.onModelLoaded(gltf, 'glb'),
            (err) => this.handleError(err, "Failed to parse GLB")
        );
      } catch (err) {
        this.handleError(err, "Failed to load GLB");
      }
    },

    loadOBJ(loader, manager) {
      // Remove trailing slash if present before replacing extension
      const cleanPath = this.fbdata.path.replace(/\/$/, '');
      const mtlPath = cleanPath.replace(/\.obj$/i, '.mtl');
      const mtlUrl = this.resourceViewUrl(mtlPath);
      if (!mtlUrl) {
        this.doLoad(loader);
        return;
      }

      const mtlLoader = new MTLLoader(manager);
      mtlLoader.load(mtlUrl, (materials) => {
        materials.preload();
        loader.setMaterials(materials);
        this.doLoad(loader);
      }, undefined, () => {
        this.doLoad(loader); // Fail gracefully if MTL missing
      });
    },

    /** CAD mesh → Mesh with per-face colors (OpenCASCADE BRep colours) and
     * per-face triangle ranges, kept for area/distance measurements. */
    buildCadObject(cadMesh) {
      const sommetsArray = new Float32Array(cadMesh.sommets);
      const geometry = new BufferGeometry();
      geometry.setAttribute('position', new BufferAttribute(sommetsArray, 3));
      geometry.computeVertexNormals();

      // Triangle ranges per BRep face + the face colour when OpenCASCADE
      // reported one (STEP colours, kept through the SolidWorks → STEP trip).
      const faces = Array.isArray(cadMesh.faces) ? cadMesh.faces : [];
      const colors = new Float32Array(sommetsArray.length);
      const base = new Color().setRGB(0.565, 0.706, 0.827, SRGBColorSpace);
      for (let i = 0; i < colors.length; i += 3) {
        colors[i] = base.r;
        colors[i + 1] = base.g;
        colors[i + 2] = base.b;
      }
      let tri = 0;
      this.cadFaces = faces.map((f) => {
        const count = f.triangles || 0;
        const range = {
          start: tri,
          count,
          nom: f.nom || '',
          corps: f.corps || 0,
          area: 0,
          color: f.couleur ? f.couleur.slice(0, 3) : null,
        };
        if (range.color) {
          const c = new Color().setRGB(range.color[0], range.color[1], range.color[2], SRGBColorSpace);
          for (let t = 0; t < count && (tri + t) * 9 + 8 < colors.length; t++) {
            const at = (tri + t) * 9;
            for (let v = 0; v < 3; v++) {
              colors[at + v * 3] = c.r;
              colors[at + v * 3 + 1] = c.g;
              colors[at + v * 3 + 2] = c.b;
            }
          }
        }
        tri += count;
        return range;
      });
      // Pre-compute each face's area (mm²) from its triangles.
      const p = sommetsArray;
      for (const range of this.cadFaces) {
        let area = 0;
        for (let t = 0; t < range.count; t++) {
          const at = (range.start + t) * 9;
          if (at + 8 >= p.length) break;
          const ax = p[at + 3] - p[at], ay = p[at + 4] - p[at + 1], az = p[at + 5] - p[at + 2];
          const bx = p[at + 6] - p[at], by = p[at + 7] - p[at + 1], bz = p[at + 8] - p[at + 2];
          area += 0.5 * Math.hypot(
            ay * bz - az * by,
            az * bx - ax * bz,
            ax * by - ay * bx,
          );
        }
        range.area = area;
      }
      geometry.setAttribute('color', new BufferAttribute(colors, 3));

      const material = new MeshStandardMaterial({
        color: 0x90caf9,
        metalness: 0.2,
        roughness: 0.5,
        side: DoubleSide,
      });

      const mesh = new Mesh(geometry, material);
      mesh.userData.isCadMesh = true;
      return mesh;
    },

    async loadCadModel(extension) {
      try {
        this.cadStatusMessage = this.$t("general.loading", { suffix: "..." });
        this.loadPreviewBackground();
        const response = await fetch(this.modelUrl);
        if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        const buffer = await response.arrayBuffer();

        const result = await loadCad3D(buffer, this.fbdata.name, (msg) => {
          this.cadStatusMessage = msg;
        });
        // SolidWorks mass properties are stored in the document; ask the Go
        // OpenSW reader for them without replacing them with mesh estimates.
        try {
          const stored = await loadCadProperties(buffer, this.fbdata.name, (msg) => {
            this.cadStatusMessage = msg;
          });
          this.cadStoredProperties = stored.properties || null;
        } catch {
          this.cadStoredProperties = null;
        }

        const object = this.buildCadObject(result.mesh);
        this.onModelLoaded(object, extension);
      } catch (err) {
        this.handleError(err, "Failed to load CAD model");
      }
    },

    doLoad(loader) {
      loader.load(
        this.modelUrl,
        (data) => this.onModelLoaded(data, this.fileExtension),
        this.onProgress,
        (err) => this.handleError(err)
      );
    },

    onModelLoaded(loadedData, extension) {
      this.clearCurrentModel();

      let object;
      const ext = extension.toLowerCase();

      // Extract object based on format
      if (['gltf', 'glb', 'dae', 'usdz', 'usd', 'usda', 'usdc', 'vrml', 'wrl', 'kmz'].includes(ext)) {
        object = loadedData.scene;
      } else if (['pcd', 'xyz'].includes(ext)) {
        // Point cloud formats return Points objects directly
        object = loadedData;
      } else if (ext === 'vtk' || ext === 'vtp') {
        // VTK returns BufferGeometry, needs to be wrapped in Mesh
        const material = new MeshStandardMaterial({
          color: 0x4fc3f7,
          metalness: 0.3,
          roughness: 0.6,
        });
        object = new Mesh(loadedData, material);
      } else {
        object = loadedData; // 3mf, stl, ply, obj, amf, vox
      }

      // Format specifics
      if (ext === '3mf') object.rotation.set(-Math.PI / 2, 0, 0);

      if (['stl', 'ply', 'amf'].includes(ext)) {
        const material = new MeshStandardMaterial({
          color: 0x4fc3f7,
          flatShading: ext === 'stl',
          metalness: 0.3,
          roughness: 0.6,
        });
        object = new Mesh(loadedData, material);
      }

      // Point clouds need Points material
      if (['pcd', 'xyz'].includes(ext)) {
        const material = new PointsMaterial({
          size: 0.05,
          color: 0x4fc3f7,
          vertexColors: !!loadedData.geometry?.attributes?.color,
        });
        if (loadedData.material) {
          // Use existing material if provided
          object = loadedData;
        } else {
          object = new Points(loadedData.geometry || loadedData, material);
        }
      }

      // Animation Setup
      // For scene-based loaders (gltf, dae, etc.), animations are on loadedData.scene.animations
      // For direct loaders (fbx, etc.), animations are on loadedData.animations
      const animations = loadedData.scene?.animations || loadedData.animations || [];
      if (animations.length > 0) {
        this.setupAnimations(object, animations);
      }

      // Validation & Processing
      let hasGeometry = false;
      object.traverse((child) => {
        if (child.isMesh || child.isSkinnedMesh) {
          hasGeometry = true;
          child.castShadow = true;
          child.receiveShadow = true;
          if (child.material) {
             const mats = Array.isArray(child.material) ? child.material : [child.material];
             for (const m of mats) {
                m.side = DoubleSide;
                if (child.isSkinnedMesh) m.skinning = true;
             }
          }
        }
      });

      if (!hasGeometry) {
        this.handleError(new Error("Model contains no renderable geometry"));
        return;
      }

      this.model = markRaw(object);
      this.scene.add(this.model);
      this.centerAndScaleModel();
      this.applyDisplayMode();
      this.clearPreviewBackground();
      this.loading = false;
    },

    setupAnimations(root, animations) {
        this.animations = animations;
        this.animationMixer = markRaw(new AnimationMixer(root));
        for (const clip of this.animations) {
            this.animationMixer.clipAction(clip).play();
        }
        this.isAnimationPlaying = true;
    },

    clearCurrentModel() {
      this.clearMeasureVisuals();
      this.measuring = false;
      if (this.model && this.scene) this.scene.remove(this.model);
      if (this.animationMixer) {
        this.animationMixer.stopAllAction();
        this.animationMixer = null;
      }
      this.animations = [];
      this.isAnimationPlaying = false;
    },

    centerAndScaleModel() {
      if (!this.model) return;
      this.model.updateMatrixWorld(true);
      const box = new Box3().setFromObject(this.model);

      if (box.isEmpty()) {
        this.model.position.set(0,0,0);
        return;
      }

      const center = box.getCenter(new Vector3());
      const size = box.getSize(new Vector3());
      const maxDim = Math.max(size.x, size.y, size.z);

      if (!Number.isFinite(maxDim) || maxDim === 0) return;

      this.model.position.copy(center.negate());

      const fov = this.camera.fov * (Math.PI / 180);
      const dist = Math.abs(maxDim / 2 / Math.tan(fov / 2)) * 1.5;

      this.camera.position.set(dist, dist * 0.5, dist);
      this.camera.lookAt(0, 0, 0);
      this.controls.target.set(0, 0, 0);
      this.controls.update();

      this.camera.near = Math.max(dist / 1000, 0.01);
      this.camera.far = Math.max(dist * 100, 1000);
      this.camera.updateProjectionMatrix();

      this.initialCameraPosition = this.camera.position.clone();
      this.initialControlsTarget = this.controls.target.clone();
    },

    animate() {
      if (!this.isInView && this.isThumbnail) return; // Stop rendering if not in view
      this.animationFrameId = requestAnimationFrame(this.animate);

      // Throttle rendering for thumbnails to 30fps to save resources
      if (this.isThumbnail) {
          const now = Date.now();
          if (!this.lastRender || now - this.lastRender > 33) { // ~30fps
              this.lastRender = now;
          } else {
              return;
          }
      }

      this.timer.update();
      if (this.animationMixer && this.isAnimationPlaying) {
        this.animationMixer.update(this.timer.getDelta());
      }
      if (this.controls) this.controls.update();
      if (this.measuring) this.updateMeasureLabels();
      if (this.renderer && this.scene && this.camera) {
        this.renderer.render(this.scene, this.camera);
      }
    },

    onWindowResize() {
      if (!this.$refs.container) return;
      const { clientWidth: w, clientHeight: h } = this.$refs.container;

      if (this.camera) {
        this.camera.aspect = w / h;
        this.camera.updateProjectionMatrix();
      }
      if (this.renderer) {
        const pixelRatio = Math.min(window.devicePixelRatio, this.isThumbnail ? 1 : 2);
        this.renderer.setSize(Math.floor(w * pixelRatio), Math.floor(h * pixelRatio), false);
      }
    },

    cleanup() {
      if (this.animationFrameId) cancelAnimationFrame(this.animationFrameId);
      this.clearMeasureVisuals();
      this.measuring = false;
      this.cadFaces = [];
      if (this.renderer?.domElement) {
        this.renderer.domElement.removeEventListener('pointerdown', this.onPointerDown);
        this.renderer.domElement.removeEventListener('click', this.onCanvasClick);
      }
      this.clearCurrentModel();

      if (this.model) {
        this.model.traverse((c) => {
          if (c.geometry) c.geometry.dispose();
          if (c.material) {
            for (const m of [].concat(c.material)) { m.dispose(); }
          }
        });
        this.model = null;
      }

      if (this.renderer) {
        // Essential for releasing WebGL contexts
        this.renderer.dispose();
        this.renderer.domElement?.remove();
        this.renderer = null;
      }
      if (this.controls) this.controls.dispose();
      if (this._occtBgTexture) {
        this._occtBgTexture.dispose();
        this._occtBgTexture = null;
      }
      this.scene = null;
      this.camera = null;
    },

    reinit() {
      this.viewTokenByPath = {};
      this.fetchedDirs = new Set();
      this.cleanup();
      this.initScene();
      void this.loadModel();
      if (!this.isThumbnail) this.updateSelectedState();
    },

    setupKeyboardShortcuts() {
      this.keyboardHandler = (e) => {
        const k = e.key.toLowerCase();
        if (['+', '=', '-', '_', ' ', 'q', 'e', 'w', 's', 'r'].includes(k)) e.preventDefault();

        const ROT_SPEED = Math.PI / 2;
        const ZOOM = 0.3;

        if (!this.model) return;

        switch (k) {
          case ' ': if (this.hasAnimations) this.toggleAnimation(); else this.toggleAutoRotate(); break;
          case 'r': this.resetCamera(); break;
          case 'q': this.model.rotation.y -= ROT_SPEED; break;
          case 'e': this.model.rotation.y += ROT_SPEED; break;
          case 'w': this.model.rotation.x -= ROT_SPEED; break;
          case 's': this.model.rotation.x += ROT_SPEED; break;
          case '+': case '=': this.zoomCamera(-ZOOM); break;
          case '-': case '_': this.zoomCamera(ZOOM); break;
        }
      };
      window.addEventListener('keydown', this.keyboardHandler);
    },

    toggleAnimation() {
      if (!this.animationMixer) return;
      this.isAnimationPlaying = !this.isAnimationPlaying;
      this.animationMixer.timeScale = this.isAnimationPlaying ? 1 : 0;
    },

    toggleAutoRotate() {
      this.isAutoRotating = !this.isAutoRotating;
      if (this.controls) this.controls.autoRotate = this.isAutoRotating;
    },

    resetCamera() {
      if (this.initialCameraPosition) {
        this.camera.position.copy(this.initialCameraPosition);
        this.controls.target.copy(this.initialControlsTarget);
        this.controls.update();
        if (this.model) {
            this.model.rotation.set(0, 0, 0);
            if (this.fileExtension === '3mf') this.model.rotation.set(-Math.PI / 2, 0, 0);
        }
      } else {
        this.centerAndScaleModel();
      }
    },

    zoomCamera(delta) {
      const dir = new Vector3().subVectors(this.camera.position, this.controls.target).normalize();
      const dist = this.camera.position.distanceTo(this.controls.target) * (1 + delta);
      if (dist > this.camera.near * 2 && dist < this.camera.far / 2) {
        this.camera.position.copy(this.controls.target.clone().add(dir.multiplyScalar(dist)));
        this.controls.update();
      }
    },
  },
};
</script>

<style scoped>
.threejs-viewer {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
}


.threejs-viewer canvas {
  display: block;
  width: 100% !important;
  height: 100% !important;
  position: absolute;
  top: 0;
  left: 0;
}

.loading-overlay, .error-overlay {
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

.measure-layer {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 4;
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

</style>
