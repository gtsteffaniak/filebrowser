<template>
  <div class="threejs-viewer" ref="container">
    <div v-if="loading" class="loading-overlay">
      <LoadingSpinner :size="isThumbnail ? 'small' : 'medium'" />
    </div>
    <div v-if="error" class="error-overlay">
      <div v-if="isThumbnail" class="error-icon">
        <i class="material-icons">view_in_ar</i>
      </div>
      <div v-else class="error-content">
        <i class="material-icons">error_outline</i>
        <h3>{{ $t("threejs.failedToLoad") }}</h3>
        <p>{{ error }}</p>
      </div>
    </div>
    
    <div v-if="!isThumbnail" class="controls-container">
      <!-- Settings icon toggle button -->
      <button 
        v-if="!showControls"
        @click="showControls = true"
        class="controls-toggle"
        title="Show controls"
        aria-label="Show controls"
      >
        <i class="material-icons">settings</i>
      </button>
      
      <!-- Expanded controls card -->
      <div v-if="showControls" class="controls-card card floating-window">
        <button 
          @click="showControls = false"
          class="controls-close"
          title="Hide controls"
          aria-label="Hide controls"
        >
          <i class="material-icons">close</i>
        </button>
        <div class="card-content">
          <div v-if="!isMobile" class="control-row">
            <span class="control-label">⌨️ {{ $t("threejs.keyboard") }}</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            <span class="control-desc">
              <kbd>{{ $t("general.space") }}</kbd> {{ getSpaceText() }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <kbd>R</kbd> {{ $t("general.reset") }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <kbd>Q</kbd>/<kbd>E</kbd> {{ $t("threejs.rotateY") }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <kbd>W</kbd>/<kbd>S</kbd> {{ $t("threejs.rotateX") }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <kbd>+</kbd>/<kbd>-</kbd> {{ $t("general.zoom") }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            </span>
          </div>
          <div class="control-row">
            <span class="control-label">🎨 {{ $t("general.background") }}</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            <span class="control-desc">
              <input 
                type="color" 
                v-model="backgroundColor" 
                @input="updateCustomBackground"
                class="color-picker-input"
                title="Change background color"
              />
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { markRaw } from 'vue';
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';
import { MTLLoader } from 'three/addons/loaders/MTLLoader.js';
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
import { state, mutations, getters } from "@/store";
import { resourcesApi } from "@/api";
import { removeLastDir } from "@/utils/url";

const LOADERS = {
  gltf: GLTFLoader,
  glb: GLTFLoader,
  obj: OBJLoader,
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
  components: { LoadingSpinner },
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
      showControls: false,
      animationMixer: null,
      animations: [],
      isAnimationPlaying: false,
      isAutoRotating: false,
      clock: null,
      observer: null,
      loadTimer: null,
      hasInitialized: false,
      isInView: false,
    };
  },
  computed: {
    isMobile() { return getters.isMobile(); },
    hasAnimations() { return this.animations && this.animations.length > 0; },
    modelUrl() {
      const useInline = this.fileExtension !== 'glb';
      if (getters.isShare()) {
        return resourcesApi.getDownloadURLPublic({
          path: state.shareInfo.subPath,
          hash: state.shareInfo.hash,
          token: state.shareInfo.token,
        }, [this.fbdata.path], useInline);
      }
      return resourcesApi.getDownloadURL(this.fbdata.source, this.fbdata.path, useInline);
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
    getSpaceText() {
      return this.hasAnimations ? this.$t("general.play") + "/" + this.$t("general.pause") : this.$t("threejs.autoRotate");
    },
    initIntersectionObserver() {
      // Use a single global observer if possible, but for now localize config
      this.observer = new IntersectionObserver(this.handleIntersect, {
        root: null,
        rootMargin: "50px", // Reduced margin to avoid eager loading too many
        threshold: 0,
      });
      this.$nextTick(() => {
        if (this.$el instanceof Element) this.observer.observe(this.$el);
      });
    },

    updateSelectedState() {
      mutations.resetSelected();
      mutations.addSelected({
        name: this.fbdata.name,
        path: this.fbdata.path,
        size: this.fbdata.size,
        type: this.fbdata.type,
        source: this.fbdata.source,
      });
    },

    handleIntersect(entries) {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          this.isInView = true;
          if (!this.hasInitialized && !this.loadTimer) {
            // Always 250ms base; add 1750ms when addLoadDelay (e.g. listing icons) for 2s total
            const delay = 250 + (this.addLoadDelay ? 1750 : 0);
            this.loadTimer = setTimeout(() => {
              if (this.isInView && !this.hasInitialized) {
                this.hasInitialized = true;
                this.initScene();
                this.loadModel();
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
      });
    },

    initScene() {
      // Create scene
      this.scene = markRaw(new THREE.Scene());
      this.updateBackgroundColor();
      this.clock = markRaw(new THREE.Clock());
      
      const container = this.$refs.container;
      const width = container.clientWidth;
      const height = container.clientHeight;
      
      this.camera = markRaw(new THREE.PerspectiveCamera(75, width / height, 0.1, 1000));
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
      
      this.renderer = markRaw(new THREE.WebGLRenderer(rendererConfig));
      this.renderer.setSize(width, height);
      this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2)); // Cap pixel ratio for performance
      container.appendChild(this.renderer.domElement);
      
      // Lights - Simplify lighting for thumbnails
      this.scene.add(markRaw(new THREE.AmbientLight(0xffffff, 0.6)));
      const dirLight1 = markRaw(new THREE.DirectionalLight(0xffffff, 0.8));
      dirLight1.position.set(1, 2, 3);
      this.scene.add(dirLight1);
      
      if (!this.isThumbnail) {
        // Only add secondary lights for full view
        const dirLight2 = markRaw(new THREE.DirectionalLight(0xffffff, 0.4));
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
        this.scene.background = new THREE.Color(this.backgroundColor);
        this.updateMaterialColors();
      }
    },

    updateMaterialColors() {
      if (this.model && this.model.isMesh && this.model.material) {
        if (['stl', 'ply', 'amf'].includes(this.fileExtension)) {
          this.model.material.color.setHex(0x4fc3f7);
        }
      }
    },
    
    updateCustomBackground() {
      if (this.scene) {
        this.scene.background = new THREE.Color(this.backgroundColor);
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
        this.$emit('error', err);
      }
    },

    resolveTextureUrl(url) {
      if (url.includes('/api/resources/raw?')) {
        return url;
      }
      const filename = url.split('/api/')[1];
      let texturePath = removeLastDir(this.fbdata.path) + "/textures/" + filename
      if (this.fbdata.parentDirItems) {
        for (const item of this.fbdata.parentDirItems) {
          if (item.name === filename) {
            texturePath = removeLastDir(this.fbdata.path) + "/" + filename;
          }
        }
      }
      if (getters.isShare()) {
        return resourcesApi.getDownloadURLPublic({
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          }, [texturePath], true);
      }
      return resourcesApi.getDownloadURL(this.fbdata.source, texturePath, true);
    },

    async loadModel() {
      this.loading = true;
      this.error = null;
      
      try {
        const extension = this.fileExtension;
        const LoaderClass = LOADERS[extension];
        if (!LoaderClass) throw new Error(`Unsupported 3D format: .${extension}`);

        const loadingManager = markRaw(new THREE.LoadingManager());
        loadingManager.onError = (url) => console.warn(`Error loading asset: ${url}`);
        loadingManager.setURLModifier((url) => this.resolveTextureUrl(url));
        
        const loader = new LoaderClass(loadingManager);

        // Special handlers
        if (extension === 'glb') {
          this.loadGLB(loader);
        } else if (extension === 'obj') {
          this.loadOBJ(loader, loadingManager);
        } else {
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
          } catch (e) {
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
      const mtlUrl = getters.isShare() ? 
          resourcesApi.getDownloadURLPublic({
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          }, [mtlPath], true) :
          resourcesApi.getDownloadURL(state.req.source, mtlPath, true);

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
        const material = new THREE.MeshStandardMaterial({
          color: 0x4fc3f7,
          metalness: 0.3,
          roughness: 0.6,
        });
        object = new THREE.Mesh(loadedData, material);
      } else {
        object = loadedData; // 3mf, stl, ply, obj, amf, vox
      }

      // Format specifics
      if (ext === '3mf') object.rotation.set(-Math.PI / 2, 0, 0);
      
      if (['stl', 'ply', 'amf'].includes(ext)) {
        const material = new THREE.MeshStandardMaterial({
          color: 0x4fc3f7,
          flatShading: ext === 'stl',
          metalness: 0.3,
          roughness: 0.6,
        });
        object = new THREE.Mesh(loadedData, material);
      }
      
      // Point clouds need Points material
      if (['pcd', 'xyz'].includes(ext)) {
        const material = new THREE.PointsMaterial({
          size: 0.05,
          color: 0x4fc3f7,
          vertexColors: loadedData.geometry?.attributes?.color ? true : false,
        });
        if (loadedData.material) {
          // Use existing material if provided
          object = loadedData;
        } else {
          object = new THREE.Points(loadedData.geometry || loadedData, material);
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
             mats.forEach(m => {
                m.side = THREE.DoubleSide;
                if (child.isSkinnedMesh) m.skinning = true;
             });
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
        this.animationMixer = markRaw(new THREE.AnimationMixer(root));
        this.animations.forEach(clip => {
            this.animationMixer.clipAction(clip).play();
        });
        this.isAnimationPlaying = true;
    },

    clearCurrentModel() {
      if (this.model) this.scene.remove(this.model);
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
      const box = new THREE.Box3().setFromObject(this.model);
      
      if (box.isEmpty()) {
        this.model.position.set(0,0,0);
        return;
      }

      const center = box.getCenter(new THREE.Vector3());
      const size = box.getSize(new THREE.Vector3());
      const maxDim = Math.max(size.x, size.y, size.z);
      
      if (!isFinite(maxDim) || maxDim === 0) return;
      
      this.model.position.copy(center.negate());
      
      const fov = this.camera.fov * (Math.PI / 180);
      let dist = Math.abs(maxDim / 2 / Math.tan(fov / 2)) * 1.5;
      
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

      if (this.animationMixer && this.isAnimationPlaying) {
        this.animationMixer.update(this.clock.getDelta());
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
        this.renderer.setSize(w, h);
      }
    },
    
    cleanup() {
      if (this.animationFrameId) cancelAnimationFrame(this.animationFrameId);
      this.clearCurrentModel();
      
      if (this.model) {
        this.model.traverse((c) => {
          if (c.geometry) c.geometry.dispose();
          if (c.material) {
            [].concat(c.material).forEach(m => m.dispose());
          }
        });
      }
      
      if (this.renderer) {
        // Essential for releasing WebGL contexts
        this.renderer.forceContextLoss();
        this.renderer.dispose();
        this.renderer.domElement?.remove();
        this.renderer = null;
      }
      if (this.controls) this.controls.dispose();
      this.scene = null;
      this.camera = null;
    },
    
    reinit() {
      this.cleanup();
      this.initScene();
      this.loadModel();
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
          case ' ': this.hasAnimations ? this.toggleAnimation() : this.toggleAutoRotate(); break;
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
      const dir = new THREE.Vector3().subVectors(this.camera.position, this.controls.target).normalize();
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
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
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

.controls-container {
  position: absolute;
  bottom: 1.5rem;
  left: 1.5rem;
  z-index: 10;
}

.controls-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: var(--surfacePrimary);
  color: var(--textPrimary);
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  transition: all 0.2s ease;
  backdrop-filter: blur(12px);
  border: 2px solid var(--surfaceSecondary);
}

.controls-toggle:hover {
  background: var(--surfaceSecondary);
  transform: scale(1.05);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.controls-toggle i {
  font-size: 1.5rem;
}

.controls-card {
  position: relative;
  width: clamp(20em, 30em, 90vw);
  max-width: 90vw;
  animation: slideIn 0.2s ease-out;
}

@keyframes slideIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.controls-close {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--textSecondary);
  cursor: pointer;
  transition: all 0.2s ease;
  z-index: 1;
}

.controls-close:hover {
  background: var(--surfaceSecondary);
  color: var(--textPrimary);
}

.controls-close i {
  font-size: 1.2rem;
}

.controls-card .card-content {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
}

.control-row {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
}

.control-label {
  font-weight: 600;
  color: var(--primaryColor);
  white-space: nowrap;
  min-width: 100px;
}

.control-desc {
  color: var(--textPrimary);
  line-height: 1.5;
}

.control-desc kbd {
  display: inline-block;
  padding: 0.2em 0.5em;
  font-size: 0.85em;
  font-family: monospace;
  background: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  margin: 0 0.2em;
}

.color-picker-input {
  width: 50px;
  height: 32px;
  border: 2px solid var(--divider);
  border-radius: 8px;
  cursor: pointer;
  background: transparent;
  padding: 2px;
  vertical-align: middle;
}

.color-picker-input::-webkit-color-swatch-wrapper {
  padding: 0;
  border-radius: 6px;
}

.color-picker-input::-webkit-color-swatch,
.color-picker-input::-moz-color-swatch {
  border: none;
  border-radius: 6px;
}

@media (max-width: 768px) {
  .controls-container { bottom: 1rem; left: 1rem; }
  .controls-toggle { width: 2.5rem; height: 2.5rem; }
  .controls-toggle i { font-size: 1.25rem; }
  .controls-card { width: 95vw; }
  .controls-card .card-content { padding: 0.75rem 1rem; }
  .control-row { flex-direction: column; gap: 0.25rem; }
  .control-label { min-width: auto; }
  .control-desc kbd { font-size: 0.8em; padding: 0.15em 0.4em; }
  .color-picker-input { width: 40px; height: 28px; }
}
</style>
