<template>
  <div class="threejs-icon-container" ref="container">
    <div v-if="loading" class="loading-icon">
      <i class="material-icons">hourglass_empty</i>
    </div>
    <div v-if="error" class="error-icon">
      <i class="material-icons">view_in_ar</i>
    </div>
  </div>
</template>

<script>
import { markRaw } from 'vue';
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';
import { FBXLoader } from 'three/addons/loaders/FBXLoader.js';
import { STLLoader } from 'three/addons/loaders/STLLoader.js';
import { PLYLoader } from 'three/addons/loaders/PLYLoader.js';
import { ColladaLoader } from 'three/addons/loaders/ColladaLoader.js';
import { ThreeMFLoader } from 'three/addons/loaders/3MFLoader.js';
import { state, getters } from "@/store";
import { filesApi, publicApi } from "@/api";

export default {
  name: "threeJsIcon",
  props: {
    filename: {
      type: String,
      required: true,
    },
    path: {
      type: String,
      required: true,
    },
    source: {
      type: String,
      default: null,
    },
  },
  data() {
    return {
      scene: null,
      camera: null,
      renderer: null,
      controls: null,
      model: null,
      loading: true,
      error: false,
      animationId: null,
      observer: null,
      isInView: false,
      hasInitialized: false,
    };
  },
  computed: {
    darkMode() {
      return state.user?.darkMode || false;
    },
  },
  mounted() {
    // Set up IntersectionObserver for lazy loading
    this.observer = new IntersectionObserver(this.handleIntersect, {
      root: null,
      rootMargin: "200px",
      threshold: 0.01,
    });

    this.$nextTick(() => {
      if (this.$el && this.$el instanceof Element) {
        this.observer.observe(this.$el);
      }
    });
  },
  beforeUnmount() {
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
    this.cleanup();
  },
  methods: {
    handleIntersect(entries) {
      entries.forEach(entry => {
        if (entry.isIntersecting && !this.hasInitialized) {
          this.isInView = true;
          this.hasInitialized = true;
          this.initScene();
          this.loadModel();
        }
      });
    },

    initScene() {
      const container = this.$refs.container;
      if (!container) return;

      const size = container.offsetWidth || 64; // Icon size

      // Scene
      this.scene = markRaw(new THREE.Scene());
      this.scene.background = new THREE.Color(0x000000);

      // Camera
      this.camera = markRaw(new THREE.PerspectiveCamera(45, 1, 0.1, 1000));
      this.camera.position.set(0, 0, 5);

      // Renderer
      this.renderer = markRaw(new THREE.WebGLRenderer({ 
        antialias: true,
        alpha: false 
      }));
      this.renderer.setSize(size, size);
      this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
      container.appendChild(this.renderer.domElement);

      // Controls (for rotation)
      this.controls = markRaw(new OrbitControls(this.camera, this.renderer.domElement));
      this.controls.enableZoom = false;
      this.controls.enablePan = false;
      this.controls.autoRotate = true;
      this.controls.autoRotateSpeed = 2;

      // Lighting
      const ambientLight = markRaw(new THREE.AmbientLight(0xffffff, 0.6));
      this.scene.add(ambientLight);

      const directionalLight = markRaw(new THREE.DirectionalLight(0xffffff, 0.8));
      directionalLight.position.set(5, 5, 5);
      this.scene.add(directionalLight);

      // Start animation loop
      this.animate();
    },
    
    async loadModel() {
      try {
        const ext = this.filename.split('.').pop().toLowerCase();
        
        // Get the download URL using the proper API functions
        let downloadUrl;
        if (getters.isShare()) {
          downloadUrl = publicApi.getDownloadURL(
            {
              path: this.path,
              hash: state.shareInfo.hash,
              token: state.shareInfo.token
            },
            [this.path],
            true // inline
          );
        } else {
          downloadUrl = filesApi.getDownloadURL(
            this.source || state.req.source,
            this.path,
            true // inline
          );
        }

        let loader;
        switch (ext) {
          case 'gltf':
          case 'glb':
            loader = new GLTFLoader();
            break;
          case 'obj':
            loader = new OBJLoader();
            break;
          case 'fbx':
            loader = new FBXLoader();
            break;
          case 'stl':
            loader = new STLLoader();
            break;
          case 'ply':
            loader = new PLYLoader();
            break;
          case 'dae':
            loader = new ColladaLoader();
            break;
          case '3mf':
            loader = new ThreeMFLoader();
            break;
          default:
            throw new Error(`Unsupported format: ${ext}`);
        }

        // Use loader.load() just like the main viewer does
        loader.load(
          downloadUrl,
          (loadedData) => {
            this.onModelLoaded(loadedData, ext);
          },
          undefined, // progress callback
          (error) => {
            console.error('Error loading 3D model for icon:', error);
            this.error = true;
            this.loading = false;
            this.$emit('error', error);
          }
        );
      } catch (err) {
        console.error('Error initializing loader for icon:', err);
        this.error = true;
        this.loading = false;
        this.$emit('error', err);
      }
    },

    onModelLoaded(loadedData, ext) {
      let object;

      // Handle different loader return types (same as main viewer)
      if (ext === 'gltf' || ext === 'glb') {
        object = loadedData.scene || loadedData;
      } else if (ext === 'dae') {
        object = loadedData.scene;
      } else if (ext === '3mf') {
        object = loadedData;
        // Apply z-up to y-up conversion if needed
        object.rotation.set(-Math.PI / 2, 0, 0);
      } else if (ext === 'stl' || ext === 'ply') {
        // STL and PLY loaders return geometry
        const geometry = loadedData;
        const material = new THREE.MeshStandardMaterial({
          color: 0x4fc3f7,
          metalness: 0.3,
          roughness: 0.4,
        });
        object = new THREE.Mesh(geometry, material);
      } else {
        object = loadedData;
      }

      this.model = markRaw(object);
      this.scene.add(this.model);
      this.centerAndScaleModel();
      this.loading = false;
    },

    centerAndScaleModel() {
      if (!this.model) return;

      // Calculate bounding box
      const box = new THREE.Box3().setFromObject(this.model);
      const center = box.getCenter(new THREE.Vector3());
      const size = box.getSize(new THREE.Vector3());
      
      // Get the maximum dimension
      const maxDim = Math.max(size.x, size.y, size.z);
      
      // Center the model at origin
      this.model.position.x = -center.x;
      this.model.position.y = -center.y;
      this.model.position.z = -center.z;
      
      // Position camera to fit the model in view
      // Use the field of view to calculate optimal distance
      const fov = this.camera.fov * (Math.PI / 180); // Convert to radians
      let cameraDistance = Math.abs(maxDim / 2 / Math.tan(fov / 2));
      
      // Add some padding (1.5x)
      cameraDistance *= 1.5;
      
      // Position camera (same as main viewer)
      this.camera.position.set(cameraDistance, cameraDistance * 0.5, cameraDistance);
      this.camera.lookAt(0, 0, 0);
      
      // Update controls target
      this.controls.target.set(0, 0, 0);
      this.controls.update();
      
      // Update camera near/far planes based on model size
      this.camera.near = cameraDistance / 100;
      this.camera.far = cameraDistance * 100;
      this.camera.updateProjectionMatrix();
    },

    animate() {
      this.animationId = requestAnimationFrame(this.animate);
      
      if (this.controls) {
        this.controls.update();
      }
      
      if (this.renderer && this.scene && this.camera) {
        this.renderer.render(this.scene, this.camera);
      }
    },

    cleanup() {
      if (this.animationId) {
        cancelAnimationFrame(this.animationId);
      }

      if (this.model) {
        this.scene.remove(this.model);
        if (this.model.geometry) this.model.geometry.dispose();
        if (this.model.material) {
          if (Array.isArray(this.model.material)) {
            this.model.material.forEach(m => m.dispose());
          } else {
            this.model.material.dispose();
          }
        }
      }

      if (this.renderer) {
        this.renderer.dispose();
        if (this.$refs.container && this.renderer.domElement) {
          this.$refs.container.removeChild(this.renderer.domElement);
        }
      }
    },
  },
};
</script>

<style scoped>
.threejs-icon-container {
  width: 100%;
  height: 100%;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #000;
}

.threejs-icon-container canvas {
  display: block;
  width: 100%;
  height: 100%;
}

.loading-icon,
.error-icon {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.loading-icon i,
.error-icon i {
  font-size: 2rem;
  color: purple;
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}
</style>
