<template>
  <div class="threejs-viewer" ref="container">
    <div v-if="loading" class="loading-overlay">
      <LoadingSpinner :size="isThumbnail ? 'small' : 'medium'" />
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
      v-if="!isThumbnail"
      icon="settings"
      position="bottom-left"
      :auto-hide="false"
      @press="showControlsPrompt"
      :label="$t('threejs.controls')"
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
  AnimationMixer,
  Timer,
  Mesh,
  Points,
  MeshStandardMaterial,
  PointsMaterial,
  DoubleSide,
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
import { state, mutations, getters } from "@/store/index.ts";
import { resourcesApi } from "@/api/index.js";
import { getCachedViewToken, ensureViewToken, requestViewIdentity } from "@/api/viewToken.js";
import { getObjectProperty } from '@/utils/object.js';
import { resolveAssetFilePath, hasSiblingFile } from "@/utils/threeAssetUrl.js";

// Texture slots on three.js materials that hold a Texture needing disposal.
const TEXTURE_SLOTS = [
  'map', 'normalMap', 'roughnessMap', 'metalnessMap', 'emissiveMap',
  'aoMap', 'alphaMap', 'bumpMap', 'specularMap', 'envMap', 'displacementMap', 'lightMap',
];

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
  components: { LoadingSpinner, FloatingActionButton },
  emits: { "load-error": () => true },
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
      // Source-scoped view token: one token authorizes the model and all of its
      // external assets (textures, .mtl) within the same source.
      activeViewToken: "",
      lastRender: 0,
    };
  },
  computed: {
    isMobile() { return getters.isMobile(); },
    hasAnimations() { return this.animations && this.animations.length > 0; },
    modelUrl() {
      return this.resourceViewUrl(this.fbdata.path);
    },
    fileExtension() {
      return this.fbdata.name ? this.fbdata.name.split('.').pop().toLowerCase() : '';
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
      this.updateBackgroundColor();
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
        if (['stl', 'ply', 'amf'].includes(this.fileExtension)) {
          this.model.material.color.setHex(0x4fc3f7);
        }
      }
    },

    updateCustomBackground() {
      if (this.scene) {
        this.scene.background = new Color(this.backgroundColor);
      }
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
        this.$emit('load-error', err);
      }
    },

    /** View URL for a model or sibling asset via viewToken (non-metered). */
    resourceViewUrl(filePath) {
      if (!filePath) {
        return "";
      }
      const viewToken = this.resolveViewToken();
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

    /**
     * Resolve the single source-scoped view token used for this model and all of
     * its external assets. Refreshes it if it is near expiry.
     */
    async ensureModelViewToken() {
      const viewIdentity = requestViewIdentity(state.req);
      let token = "";
      try {
        token = await ensureViewToken(this.fbdata.source);
        if (token && requestViewIdentity(state.req) === viewIdentity) {
          mutations.setRequestViewToken(token);
        }
      } catch (err) {
        console.warn("Failed to refresh view token for 3D preview:", err);
      }
      // The viewer is reused across source changes, so a refresh for a previous source must not
      // overwrite the token of the current one.
      if (requestViewIdentity(state.req) !== viewIdentity) return;
      this.activeViewToken = token || this.fbdata.viewToken || getCachedViewToken(this.fbdata.source) || "";
    },

    resolveViewToken() {
      return this.activeViewToken || this.fbdata.viewToken || getCachedViewToken(this.fbdata.source) || "";
    },

    /** Map a loader-emitted asset URL to a view URL; passthrough for blob:, data:, external. */
    resolveTextureUrl(url) {
      const filePath = resolveAssetFilePath(url, {
        modelPath: this.fbdata.path,
        parentDirItems: this.fbdata.parentDirItems,
      });
      if (!filePath) {
        return url;
      }
      return this.resourceViewUrl(filePath);
    },

    async loadModel() {
      await this.ensureModelViewToken();
      if (!this.modelUrl) {
        this.handleError(new Error("No view token available for model"), "Failed to load model");
        return;
      }
      this.loading = true;
      this.error = null;

      try {
        const extension = this.fileExtension;
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
        // Strip a trailing slash first, otherwise the buffer base is the model file itself.
        const modelPath = this.fbdata.path.replace(/\/+$/, '');
        const modelDir = modelPath.slice(0, modelPath.lastIndexOf('/') + 1);

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
      const mtlName = mtlPath.split("/").pop();
      // Skip the .mtl request when the directory listing proves it does not exist.
      // Note: this assumes mtllib names the .mtl by basename; a differing mtllib name will still fall back gracefully.
      const mtlUrl = hasSiblingFile(this.fbdata.parentDirItems, mtlName) ? this.resourceViewUrl(mtlPath) : "";
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

      // AMFLoader returns a Group with its own materials, so only geometry-only formats are wrapped.
      if (['stl', 'ply'].includes(ext)) {
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
      this.clearCurrentModel();

      if (this.model) {
        this.model.traverse((c) => {
          if (c.geometry) c.geometry.dispose();
          if (c.material) {
            for (const m of [].concat(c.material)) {
              for (const slot of TEXTURE_SLOTS) {
                m[slot]?.dispose?.();
              }
              m.dispose();
            }
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
      this.scene = null;
      this.camera = null;
    },

    reinit() {
      this.activeViewToken = "";
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
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10;
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

</style>
