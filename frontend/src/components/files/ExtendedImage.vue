<template>
  <div class="image-ex-container" ref="container" @touchstart="touchStart" @touchmove="touchMove" @touchend="touchEnd" @dblclick="zoomAuto"
    @mousedown="mousedownStart" @mousemove="mouseMove" @mouseup="mouseUp" @wheel="wheelMove">
    <!-- Thumbnail placeholder (shown while full image loads, only if cached thumbnail exists) -->
    <img 
      v-if="cachedThumbnailUrl && !fullImageLoaded && !isTiff" 
      :src="cachedThumbnailUrl" 
      class="image-ex-img" 
      ref="thumbnail"
    />
    
    <!-- Loading spinner overlay (shown while full image loads) -->
    <div v-if="!fullImageLoaded" class="image-loading-overlay">
      <LoadingSpinner size="medium" />
    </div>

    <!-- Full image: 
         - Always loading in background via JavaScript
         - Hidden until loaded if thumbnail exists, otherwise visible for progressive loading -->
    <img 
      v-if="!isTiff" 
      class="image-ex-img" 
      ref="imgex" 
      @load="onLoad" 
      @error="onImageError" 
      :style="{ display: (cachedThumbnailUrl && !fullImageLoaded) ? 'none' : 'block' }" 
    />
    <canvas 
      v-else 
      ref="imgex" 
      class="image-ex-img" 
      :style="{ display: (cachedThumbnailUrl && !fullImageLoaded) ? 'none' : 'block' }"
    ></canvas>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import throttle from "@/utils/throttle";
import { notify } from "@/notify";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { getBestCachedImage } from "@/utils/imageCache";
import { globalVars } from "@/utils/constants";

export default {
  components: {
    LoadingSpinner,
  },
  props: {
    src: {
      type: String,
      default: null,
    },
  },
  data() {
    return {
      moveDisabledTime: 200,
      classList: [],
      zoomStep: 0.25,
      scale: 1,
      lastX: null,
      lastY: null,
      inDrag: false,
      touches: 0,
      lastTouchDistance: 0,
      moveDisabled: false,
      disabledTimer: null,
      imageLoaded: false,
      fullImageLoaded: false,
      loadTimeout: null,
      position: {
        center: { x: 0, y: 0 },
        relative: { x: 0, y: 0 },
      },
      maxScale: 4,
      minScale: 1,
      isTiff: false,
      // Swipe navigation properties
      swipeStartTime: null,
      swipeStartX: 0,
      swipeStartY: 0,
      swipeCurrentX: 0,
      swipeCurrentY: 0,
      isSwipeGesture: false,
      hasStartedSwipe: false,
      gestureDecided: false,
      swipeMinDistance: 150,
      swipeMaxTime: 500,
      swipeMaxVerticalDistance: 50,
    };
  },
  computed: {
    source() {
      return state.req?.source;
    },
    path() {
      return state.req?.path;
    },
    isLoaded() {
      return !("preview-img" in (state?.loading || {}));
    },
    cachedThumbnailUrl() {
      if (!state?.req) {
        return null;
      }
      if (!this.path || !state.req.hasPreview) {
        return null;
      }
      // Don't use thumbnail for HEIC files that need conversion
      const showFullSizeHeic = state.req?.type === "image/heic" && !state.isSafari && globalVars.mediaAvailable && !globalVars.disableHeicConversion;
      if (showFullSizeHeic) {
        return null;
      }
      // Get cached thumbnail URL (prefers large, falls back to small)
      // For shares, use shareInfo.hash as the source; otherwise use this.source
      const source = getters.isShare() ? state.shareInfo?.hash : this.source;
      return getBestCachedImage(source, this.path, state.req?.modified);
    },
  },
  mounted() {
    this.isTiff = this.checkIfTiff(this.src);
    
    // Step 1: Cache check happens automatically via thumbnailUrl computed property
    
    // Step 2: Always start loading the real image
    if (this.isTiff) {
      this.decodeTiff(this.src);
    } else {
      // Use nextTick to ensure element exists
      this.$nextTick(() => {
        this.loadFullImage();
      });
    }
    
    let container = this.$refs.container;
    this.classList.forEach((className) => container.classList.add(className));
    if (getComputedStyle(container).width === "0px") {
      container.style.width = "100%";
    }
    if (getComputedStyle(container).height === "0px") {
      container.style.height = "100%";
    }

    window.addEventListener("resize", this.onResize);
  },
  beforeUnmount() {
    // Clear any pending timeout
    if (this.loadTimeout) {
      clearTimeout(this.loadTimeout);
      this.loadTimeout = null;
    }
    window.removeEventListener("resize", this.onResize);
    document.removeEventListener("mouseup", this.onMouseUp);
  },
  methods: {
    loadFullImage() {
      if (!this.src) return;
      mutations.setLoading("preview-img", true);
      
      // Set src directly via JavaScript to avoid Vue's HTML entity encoding in template bindings
      // Vue HTML-encodes & to &amp; when using :src="src" in templates
      this.$nextTick(() => {
        if (this.$refs.imgex && 'src' in this.$refs.imgex) {
          // Decode any HTML entities (Vue shouldn't encode props, but decode just in case)
          const cleanSrc = String(this.src).replace(/&amp;/g, '&');
          this.$refs.imgex.src = cleanSrc;
        }
      });
    },
    onLoad() {
      // Step 3: Real image loaded - hide thumbnail and show image
      this.imageLoaded = true;
      this.fullImageLoaded = true;
      // Clear the timeout if image loaded successfully
      if (this.loadTimeout) {
        clearTimeout(this.loadTimeout);
        this.loadTimeout = null;
      }
      this.setCenter();
      mutations.setLoading("preview-img", false);
    },
    onImageError(event) {
      const img = event.target;
      const actualSrc = img?.src || '';
      
      // If the error is due to &amp; in URL, try to fix it
      if (actualSrc && actualSrc.includes('&amp;')) {
        const fixedSrc = actualSrc.replace(/&amp;/g, '&');
        if (img) {
          img.src = fixedSrc;
          return; // Let it retry with fixed URL
        }
      }
      
      this.imageLoaded = true;
      this.fullImageLoaded = true;
      mutations.setLoading("preview-img", false);
      if (img) {
        img.style.display = 'block';
      }
    },
    checkIfTiff(src) {
      const sufs = ["tif", "tiff", "dng", "cr2", "nef"];
      const suff = src.split(".").pop().toLowerCase();
      return sufs.includes(suff);
    },
    async decodeTiff(src) {
      try {
        const response = await fetch(src);
        if (!response.ok) {
          throw new Error("Network response was not ok");
        }
        const blob = await response.blob();
        const imgex = this.$refs.imgex;
        if (imgex) {
          imgex.src = URL.createObjectURL(blob);
          imgex.onload = () => URL.revokeObjectURL(imgex.src);
        }
      } catch (error) {
        notify.showError("Error decoding TIFF");
      }
    },
    onMouseUp() {
      this.inDrag = false;
    },
    onResize: throttle(function () {
      if (this.imageLoaded) {
        this.setCenter();
        this.doMove(this.position.relative.x, this.position.relative.y);
      }
    }, 100),
    setCenter() {
      const container = this.$refs.container;
      const img = this.$refs.imgex;
      if (!container || !img || !img.clientWidth || !img.clientHeight) {
        return;
      }
      // Images are centered using CSS (top: 50%, left: 50%, transform: translate(-50%, -50%))
      // Reset pan position when centering
      this.position.relative = { x: 0, y: 0 };
      this.position.center.x = Math.floor((container.clientWidth - img.clientWidth) / 2);
      this.position.center.y = Math.floor((container.clientHeight - img.clientHeight) / 2);
      // Update transform to reflect centered state
      if (this.scale === 1) {
        img.style.transform = 'translate(-50%, -50%) scale(1)';
      } else {
        img.style.transform = `translate(calc(-50% + ${this.position.relative.x}px), calc(-50% + ${this.position.relative.y}px)) scale(${this.scale})`;
      }
    },
    mousedownStart(event) {
      this.lastX = null;
      this.lastY = null;
      this.inDrag = true;
      event.preventDefault();
    },
    mouseMove(event) {
      if (!this.inDrag) return;
      this.doMove(event.movementX, event.movementY);
      event.preventDefault();
    },
    mouseUp(event) {
      this.inDrag = false;
      event.preventDefault();
    },
    touchStart(event) {
      this.lastX = null;
      this.lastY = null;
      this.lastTouchDistance = null;
      if (event.targetTouches.length === 1 && this.scale === 1) {
        const touch = event.targetTouches[0];
        this.swipeStartTime = Date.now();
        this.swipeStartX = touch.pageX;
        this.swipeStartY = touch.pageY;
        this.swipeCurrentX = touch.pageX;
        this.swipeCurrentY = touch.pageY;
        this.isSwipeGesture = false;
        this.hasStartedSwipe = false;
        this.gestureDecided = false;
      } else {
        this.resetSwipeTracking();
      }
      if (event.targetTouches.length < 2) {
        setTimeout(() => {
          this.touches = 0;
        }, 300);
        this.touches++;
        if (this.touches > 1) {
          this.zoomAuto(event);
          event.preventDefault();
        }
      }
      if (this.scale > 1 || event.targetTouches.length >= 2) {
        event.preventDefault();
      }
    },
    zoomAuto(event) {
      switch (this.scale) {
        case 1:
          this.scale = 2;
          break;
        case 2:
          this.scale = 4;
          break;
        default:
        case 4:
          this.scale = 1;
          this.setCenter();
          break;
      }
      this.setZoom();
      event.preventDefault();
    },
    touchMove(event) {
      if (event.targetTouches.length === 1 && this.scale === 1) {
        const touch = event.targetTouches[0];
        this.swipeCurrentX = touch.pageX;
        this.swipeCurrentY = touch.pageY;
        if (!this.gestureDecided) {
          const deltaX = Math.abs(this.swipeCurrentX - this.swipeStartX);
          const deltaY = Math.abs(this.swipeCurrentY - this.swipeStartY);
          if (deltaX > 10 || deltaY > 10) {
            this.gestureDecided = true;
            if (deltaX > deltaY * 2) {
              this.isSwipeGesture = true;
              event.preventDefault();
            } else {
              this.isSwipeGesture = false;
            }
          }
        }
        if (this.gestureDecided && this.isSwipeGesture) {
          event.preventDefault();
          return;
        }
      }
      if (this.scale > 1 || event.targetTouches.length >= 2) {
        event.preventDefault();
      }
      if (this.lastX === null) {
        this.lastX = event.targetTouches[0].pageX;
        this.lastY = event.targetTouches[0].pageY;
        return;
      }
      let step = this.$refs.imgex.width / 5;
      if (event.targetTouches.length === 2) {
        this.moveDisabled = true;
        clearTimeout(this.disabledTimer);
        this.disabledTimer = setTimeout(
          () => (this.moveDisabled = false),
          this.moveDisabledTime
        );
        let p1 = event.targetTouches[0];
        let p2 = event.targetTouches[1];
        let touchDistance = Math.sqrt(
          Math.pow(p2.pageX - p1.pageX, 2) + Math.pow(p2.pageY - p1.pageY, 2)
        );
        if (!this.lastTouchDistance) {
          this.lastTouchDistance = touchDistance;
          return;
        }
        this.scale += (touchDistance - this.lastTouchDistance) / step;
        this.lastTouchDistance = touchDistance;
        this.setZoom();
      } else if (event.targetTouches.length === 1 && this.scale > 1) {
        if (this.moveDisabled) return;
        let x = event.targetTouches[0].pageX - this.lastX;
        let y = event.targetTouches[0].pageY - this.lastY;
        if (Math.abs(x) >= step && Math.abs(y) >= step) return;
        this.lastX = event.targetTouches[0].pageX;
        this.lastY = event.targetTouches[0].pageY;
        this.doMove(x, y);
      }
    },
    doMove(x, y) {
      this.position.relative.x += x;
      this.position.relative.y += y;
      const img = this.$refs.imgex;
      if (img) {
        // Combine centering (-50%) with pan offset and scale
        img.style.transform = `translate(calc(-50% + ${this.position.relative.x}px), calc(-50% + ${this.position.relative.y}px)) scale(${this.scale})`;
      }
    },
    wheelMove(event) {
      event.preventDefault()
      this.scale += -Math.sign(event.deltaY) * this.zoomStep;
      this.setZoom();
    },
    setZoom() {
      this.scale = Math.max(this.minScale, Math.min(this.maxScale, this.scale));
      if (this.scale === 1) {
        this.position.relative = { x: 0, y: 0 };
      }
      const img = this.$refs.imgex;
      if (img) {
        // Combine centering (-50%) with pan offset and scale
        img.style.transform = `translate(calc(-50% + ${this.position.relative.x}px), calc(-50% + ${this.position.relative.y}px)) scale(${this.scale})`;
      }
    },
    pxStringToNumber(style) {
      return +style.replace("px", "");
    },
    touchEnd(event) {
      let handledSwipe = false;
      if (this.isSwipeGesture && this.swipeStartTime && this.scale === 1) {
        const swipeEndTime = Date.now();
        const swipeDuration = swipeEndTime - this.swipeStartTime;
        const deltaX = this.swipeCurrentX - this.swipeStartX;
        const deltaY = Math.abs(this.swipeCurrentY - this.swipeStartY);
        const absDelataX = Math.abs(deltaX);
        if (
          swipeDuration <= this.swipeMaxTime &&
          absDelataX >= this.swipeMinDistance &&
          deltaY <= this.swipeMaxVerticalDistance
        ) {
          if (deltaX > 0) {
            this.$emit('navigate-previous');
          } else {
            this.$emit('navigate-next');
          }
          handledSwipe = true;
          event.preventDefault();
        }
      }
      if (!handledSwipe && this.scale > 1) {
        event.preventDefault();
      }
      this.resetSwipeTracking();
    },
    resetSwipeTracking() {
      this.swipeStartTime = null;
      this.swipeStartX = 0;
      this.swipeStartY = 0;
      this.swipeCurrentX = 0;
      this.swipeCurrentY = 0;
      this.isSwipeGesture = false;
      this.hasStartedSwipe = false;
      this.gestureDecided = false;
    },
  },
  watch: {
    src: function (newSrc) {
      if (!newSrc) {
        mutations.setLoading("preview-img", false);
        return;
      }
      
      // Clear any existing timeout
      if (this.loadTimeout) {
        clearTimeout(this.loadTimeout);
        this.loadTimeout = null;
      }
      
      // Reset and reload when src changes
      this.fullImageLoaded = false;
      this.imageLoaded = false;
      this.isTiff = this.checkIfTiff(newSrc);
      
      // Cache check happens automatically via thumbnailUrl computed property
      
      // Always load the real image
      if (this.isTiff) {
        this.decodeTiff(newSrc);
      } else {
        this.$nextTick(() => {
          this.loadFullImage();
          // Set a timeout to handle cases where image never loads
          this.loadTimeout = setTimeout(() => {
            if (!this.fullImageLoaded && !this.imageLoaded) {
              // Show the image even if load event didn't fire (might be partially loaded)
              this.fullImageLoaded = true;
              mutations.setLoading("preview-img", false);
            }
          }, 30000); // 30 second timeout
        });
      }
      
      this.scale = 1;
      this.position.relative = { x: 0, y: 0 };
      this.resetSwipeTracking();
    },
  },
};
</script>

<style>
.image-ex-container {
  max-width: 100%;
  max-height: 100%;
  overflow: hidden;
  position: relative;
  display: flex;
  justify-content: center;
}

.image-ex-img {
  max-width: 100%;
  max-height: 100%;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  object-fit: contain;
}

.image-loading-overlay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
