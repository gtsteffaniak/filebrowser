<template>
  <div id="threejs-viewer" ref="container">
    <div v-if="loading" class="loading-overlay">
      <LoadingSpinner size="medium" />
      <p>Loading 3D Model...</p>
    </div>
    <div v-if="error" class="error-overlay">
      <div class="error-content">
        <i class="material-icons">error_outline</i>
        <h3>Failed to Load 3D Model</h3>
        <p>{{ error }}</p>
      </div>
    </div>
    
    <div class="controls-info">
      <div class="controls-section">
        <div class="control-row">
          <span class="control-label">⌨️ Keyboard:</span>
          <span class="control-desc">
            <kbd>R</kbd> Reset • 
            <kbd>Q</kbd>/<kbd>E</kbd> Rotate Y-axis • 
            <kbd>W</kbd>/<kbd>S</kbd> Rotate X-axis • 
            <kbd>+</kbd>/<kbd>-</kbd> Zoom
          </span>
        </div>
        <div class="control-row">
          <span class="control-label">🎨 Background:</span>
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
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { state, mutations, getters } from "@/store";
import { filesApi, publicApi } from "@/api";

export default {
  name: "threeJsViewer",
  components: {
    LoadingSpinner,
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
      backgroundColor: '#000000', // Default black
    };
  },
  computed: {
    req() {
      return state.req;
    },
    modelUrl() {
      if (getters.isShare()) {
        return publicApi.getDownloadURL(
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          },
          [state.req.path],
          true,
        );
      }
      return filesApi.getDownloadURL(
        state.req.source,
        state.req.path,
        true,
      );
    },
    fileExtension() {
      return state.req.name.split('.').pop().toLowerCase();
    }
  },
  watch: {
    req() {
      this.reinit();
    },
  },
  mounted() {
    // Set initial background color to black
    this.backgroundColor = '#000000';
    
    this.initScene();
    this.loadModel();
    this.setupKeyboardShortcuts();
    window.addEventListener('resize', this.onWindowResize);
    
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
    });
  },
  beforeUnmount() {
    this.cleanup();
    window.removeEventListener('resize', this.onWindowResize);
    if (this.keyboardHandler) {
      window.removeEventListener('keydown', this.keyboardHandler);
    }
  },
  methods: {
    initScene() {
      // Create scene - mark as non-reactive
      this.scene = markRaw(new THREE.Scene());
      this.updateBackgroundColor();
      
      // Create camera - mark as non-reactive
      const container = this.$refs.container;
      const width = container.clientWidth;
      const height = container.clientHeight;
      this.camera = markRaw(new THREE.PerspectiveCamera(75, width / height, 0.1, 1000));
      this.camera.position.set(0, 0, 5);
      
      // Create renderer - mark as non-reactive
      this.renderer = markRaw(new THREE.WebGLRenderer({ antialias: true }));
      this.renderer.setSize(width, height);
      this.renderer.setPixelRatio(window.devicePixelRatio);
      container.appendChild(this.renderer.domElement);
      
      // Add lights - mark as non-reactive
      const ambientLight = markRaw(new THREE.AmbientLight(0xffffff, 0.5));
      this.scene.add(ambientLight);
      
      const directionalLight1 = markRaw(new THREE.DirectionalLight(0xffffff, 0.8));
      directionalLight1.position.set(1, 1, 1);
      this.scene.add(directionalLight1);
      
      const directionalLight2 = markRaw(new THREE.DirectionalLight(0xffffff, 0.4));
      directionalLight2.position.set(-1, -1, -1);
      this.scene.add(directionalLight2);
      
      // Add orbit controls - mark as non-reactive
      this.controls = markRaw(new OrbitControls(this.camera, this.renderer.domElement));
      this.controls.enableDamping = true;
      this.controls.dampingFactor = 0.05;
      
      // Start animation loop
      this.animate();
    },
    
    updateBackgroundColor() {
      if (this.scene) {
        this.scene.background = new THREE.Color(this.backgroundColor);
        
        // Update material color if model is STL/PLY
        if (this.model && this.model.isMesh && this.model.material) {
          const ext = this.fileExtension;
          if (ext === 'stl' || ext === 'ply') {
            // Use light blue for better contrast on dark background
            const modelColor = 0x4fc3f7;
            this.model.material.color.setHex(modelColor);
          }
        }
      }
    },
    
    updateCustomBackground() {
      // Update scene background when user changes color
      if (this.scene) {
        this.scene.background = new THREE.Color(this.backgroundColor);
      }
    },
    
    async loadModel() {
      this.loading = true;
      this.error = null;
      
      try {
        let loader;
        const extension = this.fileExtension;
        
        switch (extension) {
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
            throw new Error(`Unsupported 3D format: .${extension}`);
        }
        
        loader.load(
          this.modelUrl,
          (loadedData) => {
            this.onModelLoaded(loadedData, extension);
          },
          (xhr) => {
            // Progress callback
            if (xhr.lengthComputable) {
              const percentComplete = (xhr.loaded / xhr.total) * 100;
              console.log(`Loading: ${Math.round(percentComplete)}%`);
            }
          },
          (error) => {
            console.error('Error loading model:', error);
            this.error = `Failed to load model: ${error.message}`;
            this.loading = false;
          }
        );
      } catch (error) {
        console.error('Error initializing loader:', error);
        this.error = error.message;
        this.loading = false;
      }
    },
    
    onModelLoaded(loadedData, extension) {
      // Remove previous model if exists
      if (this.model) {
        this.scene.remove(this.model);
      }
      
      let object;
      
      // Handle different loader return types
      if (extension === 'gltf' || extension === 'glb') {
        object = loadedData.scene;
      } else if (extension === 'dae') {
        object = loadedData.scene;
      } else if (extension === '3mf') {
        // 3MF loader returns a Group object
        object = loadedData;
        // Apply z-up to y-up conversion if needed
        object.rotation.set(-Math.PI / 2, 0, 0);
      } else if (extension === 'stl' || extension === 'ply') {
        // STL and PLY loaders return geometry
        const geometry = loadedData;
        // Use light blue for better contrast on dark background
        const modelColor = 0x4fc3f7;
        const material = new THREE.MeshStandardMaterial({
          color: modelColor,
          flatShading: extension === 'stl',
          metalness: 0.3,
          roughness: 0.6,
        });
        object = new THREE.Mesh(geometry, material);
      } else {
        object = loadedData;
      }
      
      // Mark the model as non-reactive
      this.model = markRaw(object);
      this.scene.add(this.model);
      
      // Center and scale the model
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
      
      // Position camera
      this.camera.position.set(cameraDistance, cameraDistance * 0.5, cameraDistance);
      this.camera.lookAt(0, 0, 0);
      
      // Update controls target
      this.controls.target.set(0, 0, 0);
      this.controls.update();
      
      // Update camera near/far planes based on model size
      this.camera.near = cameraDistance / 100;
      this.camera.far = cameraDistance * 100;
      this.camera.updateProjectionMatrix();
      
      // Store initial camera position for reset
      this.initialCameraPosition = this.camera.position.clone();
      this.initialControlsTarget = this.controls.target.clone();
    },
    
    animate() {
      this.animationFrameId = requestAnimationFrame(this.animate);
      
      if (this.controls) {
        this.controls.update();
      }
      
      if (this.renderer && this.scene && this.camera) {
        this.renderer.render(this.scene, this.camera);
      }
    },
    
    onWindowResize() {
      if (!this.$refs.container) return;
      
      const container = this.$refs.container;
      const width = container.clientWidth;
      const height = container.clientHeight;
      
      this.camera.aspect = width / height;
      this.camera.updateProjectionMatrix();
      this.renderer.setSize(width, height);
    },
    
    cleanup() {
      // Cancel animation loop
      if (this.animationFrameId) {
        cancelAnimationFrame(this.animationFrameId);
      }
      
      // Dispose of geometries and materials
      if (this.model) {
        this.model.traverse((child) => {
          if (child.geometry) {
            child.geometry.dispose();
          }
          if (child.material) {
            if (Array.isArray(child.material)) {
              child.material.forEach(material => material.dispose());
            } else {
              child.material.dispose();
            }
          }
        });
      }
      
      // Dispose of renderer
      if (this.renderer) {
        this.renderer.dispose();
        if (this.renderer.domElement && this.renderer.domElement.parentNode) {
          this.renderer.domElement.parentNode.removeChild(this.renderer.domElement);
        }
      }
      
      // Dispose of controls
      if (this.controls) {
        this.controls.dispose();
      }
    },
    
    reinit() {
      this.cleanup();
      this.initScene();
      this.loadModel();
      
      mutations.resetSelected();
      mutations.addSelected({
        name: state.req.name,
        path: state.req.path,
        size: state.req.size,
        type: state.req.type,
        source: state.req.source,
      });
    },
    
    setupKeyboardShortcuts() {
      this.keyboardHandler = (event) => {
        const { key } = event;
        
        // Prevent default for keys we're handling
        if (['+', '=', '-', '_'].includes(key) || 
            ['q', 'e', 'w', 's', 'r'].includes(key.toLowerCase())) {
          event.preventDefault();
        }
        
        const rotationSpeed = Math.PI / 2; // 90 degrees in radians
        const zoomSpeed = 0.3; // More significant zoom increments
        
        switch (key.toLowerCase()) {
          case 'r':
            // Reset camera to initial position
            this.resetCamera();
            break;
            
          case 'q':
            // Rotate model left around Y-axis (90 degrees)
            if (this.model) {
              this.model.rotation.y -= rotationSpeed;
            }
            break;
            
          case 'e':
            // Rotate model right around Y-axis (90 degrees)
            if (this.model) {
              this.model.rotation.y += rotationSpeed;
            }
            break;
            
          case 'w':
            // Rotate model up around X-axis (90 degrees)
            if (this.model) {
              this.model.rotation.x -= rotationSpeed;
            }
            break;
            
          case 's':
            // Rotate model down around X-axis (90 degrees)
            if (this.model) {
              this.model.rotation.x += rotationSpeed;
            }
            break;
            
          case '+':
          case '=':
            // Zoom in (more significant)
            this.zoomCamera(-zoomSpeed);
            break;
            
          case '-':
          case '_':
            // Zoom out (more significant)
            this.zoomCamera(zoomSpeed);
            break;
        }
      };
      window.addEventListener('keydown', this.keyboardHandler);
    },
    
    resetCamera() {
      if (this.initialCameraPosition && this.initialControlsTarget) {
        this.camera.position.copy(this.initialCameraPosition);
        this.controls.target.copy(this.initialControlsTarget);
        this.controls.update();
        
        // Reset model rotation
        if (this.model) {
          this.model.rotation.set(0, 0, 0);
          // Re-apply 3MF rotation if needed
          if (this.fileExtension === '3mf') {
            this.model.rotation.set(-Math.PI / 2, 0, 0);
          }
        }
      } else {
        // Fallback: recalculate from scratch
        this.centerAndScaleModel();
      }
    },
    
    zoomCamera(delta) {
      const direction = new THREE.Vector3();
      direction.subVectors(this.camera.position, this.controls.target).normalize();
      
      const distance = this.camera.position.distanceTo(this.controls.target);
      const newDistance = distance * (1 + delta);
      
      // Prevent zooming too close or too far
      if (newDistance > this.camera.near * 2 && newDistance < this.camera.far / 2) {
        this.camera.position.copy(
          this.controls.target.clone().add(direction.multiplyScalar(newDistance))
        );
        this.controls.update();
      }
    },
  },
};
</script>

<style scoped>
#threejs-viewer {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
}

/* Ensure canvas fills the container */
#threejs-viewer canvas {
  display: block;
  width: 100% !important;
  height: 100% !important;
  position: absolute;
  top: 0;
  left: 0;
}

.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10;
}

.loading-overlay p {
  margin-top: 1rem;
  color: var(--textPrimary);
}

.error-overlay {
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

.controls-info {
  position: absolute;
  bottom: 1.5rem;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.85);
  color: white;
  padding: 1rem 1.5rem;
  border-radius: 12px;
  font-size: 0.9rem;
  z-index: 10; /* Lower than navigation buttons (1001) */
  backdrop-filter: blur(10px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.1);
  width: clamp(30em, 30em, 90vw); /* min, preferred, max */
}

.controls-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.control-row {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
}

.control-label {
  font-weight: 600;
  color: #4fc3f7;
  white-space: nowrap;
  min-width: 100px;
}

.control-desc {
  color: #e0e0e0;
  line-height: 1.5;
}

.control-desc kbd {
  display: inline-block;
  padding: 0.2em 0.5em;
  font-size: 0.85em;
  font-family: monospace;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 4px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  margin: 0 0.2em;
}

.control-desc .color-picker-input {
  width: 50px;
  height: 32px;
  border: 2px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  cursor: pointer;
  background: transparent;
  padding: 2px;
  vertical-align: middle;
}

.control-desc .color-picker-input::-webkit-color-swatch-wrapper {
  padding: 0;
  border-radius: 6px;
}

.control-desc .color-picker-input::-webkit-color-swatch {
  border: none;
  border-radius: 6px;
}

.control-desc .color-picker-input::-moz-color-swatch {
  border: none;
  border-radius: 6px;
}

@media (max-width: 768px) {
  .controls-info {
    font-size: 0.75rem;
    padding: 0.75rem 1rem;
    bottom: 1rem;
    max-width: 95%;
  }
  
  .control-row {
    flex-direction: column;
    gap: 0.25rem;
  }
  
  .control-label {
    min-width: auto;
  }
  
  .control-desc kbd {
    font-size: 0.8em;
    padding: 0.15em 0.4em;
  }
  
  .control-desc .color-picker-input {
    width: 40px;
    height: 28px;
  }
}
</style>
