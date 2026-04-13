<template>
  <div class="image-ex-container" ref="container" @touchstart="touchStart" @touchmove.prevent="touchMove" @touchend="touchEnd" @dblclick="zoomAuto"
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
import { state, mutations, getters } from '@/store';
import throttle from "@/utils/throttle";
import { notify } from "@/notify";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { getBestCachedImage } from "@/utils/imageCache";
import { isRawImageMimeType } from "@/utils/mimetype";
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
      // Full-frame edge gestures (touch + mouse), zoom scale === 1 only
      edgeKind: null, // null | 'horizontal' | 'vertical-dismiss'
      edgeStartX: 0,
      edgeStartY: 0,
      edgeDx: 0,
      edgeDy: 0,
      dragOffsetX: 0,
      dragOffsetY: 0,
      gestureDecided: false,
      gestureSnapBack: false,
      edgeMouseActive: false,
      showNavHint: false,
      navHintDir: 'next', // 'prev' | 'next' — chevrons match nextPrevious.vue
      showDismissHint: false,
      dismissFlashActive: false,
      edgeHintPx: 44,
      edgeCommitX: 130,
      edgeCommitY: 110,
      edgeRubberMax: 100,
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
      // Don't use thumbnail when we show original embedded preview (HEIC/HEIF or raw)
      const isHeicOrHeif = state.req?.type === "image/heic" || state.req?.type === "image/heif";
      const useOriginalForHeic = isHeicOrHeif && (state.isSafari || (globalVars.mediaAvailable && globalVars.enableHeicConversion) || globalVars.exiftoolAvailable);
      const useOriginalForRaw = isRawImageMimeType(state.req?.type) && globalVars.exiftoolAvailable;
      if (useOriginalForHeic || useOriginalForRaw) {
        return null;
      }
      // Get cached thumbnail URL (prefers large, falls back to small)
      // For shares, use shareInfo.hash as the source; otherwise use this.source
      const source = getters.isShare() ? state.shareInfo?.hash : this.source;
      return getBestCachedImage(source, this.path, state.req?.modified);
    },
    navigationGestureAllowed() {
      return state.navigation.enabled && getters.currentPrompt() == null;
    },
    hasImagePrevious() {
      return this.navigationGestureAllowed && state.navigation.previousLink !== '';
    },
    hasImageNext() {
      return this.navigationGestureAllowed && state.navigation.nextLink !== '';
    },
    navPrevCommitReady() {
      const ax = Math.abs(this.edgeDx);
      const ay = Math.abs(this.edgeDy);
      return (
        this.hasImagePrevious &&
        this.edgeDx >= this.edgeCommitX &&
        ax >= ay
      );
    },
    navNextCommitReady() {
      const ax = Math.abs(this.edgeDx);
      const ay = Math.abs(this.edgeDy);
      return (
        this.hasImageNext &&
        this.edgeDx <= -this.edgeCommitX &&
        ax >= ay
      );
    },
    dismissCommitReady() {
      const ax = Math.abs(this.edgeDx);
      const ay = Math.abs(this.edgeDy);
      return this.edgeDy >= this.edgeCommitY && ay >= ax;
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
    this.teardownEdgeMouseListeners();
    mutations.setNavigationGestureHint({});
    window.removeEventListener("resize", this.onResize);
    document.removeEventListener("mouseup", this.onMouseUp);
  },
  methods: {
    syncNavigationGestureHintToStore() {
      let kind = null;
      let commitReady = false;
      let flashClose = false;
      if (this.dismissFlashActive) {
        kind = 'close';
        commitReady = this.dismissCommitReady;
        flashClose = true;
      } else if (this.showDismissHint) {
        kind = 'close';
        commitReady = this.dismissCommitReady;
      } else if (this.showNavHint && this.navHintDir === 'prev' && this.hasImagePrevious) {
        kind = 'previous';
        commitReady = this.navPrevCommitReady;
      } else if (this.showNavHint && this.navHintDir === 'next' && this.hasImageNext) {
        kind = 'next';
        commitReady = this.navNextCommitReady;
      }
      mutations.setNavigationGestureHint({ kind, commitReady, flashClose });
    },
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
      mutations.setLoading("preview-img", false);
      // fullImageLoaded flips display from none → block via :style. Vue hasn't
      // updated the DOM yet, so setCenter() in the same tick sees clientWidth/Height 0.
      // Defer until after layout so centering / transform apply correctly.
      this.scheduleSetCenter();
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
      this.scheduleSetCenter();
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
    scheduleSetCenter() {
      this.$nextTick(() => {
        requestAnimationFrame(() => {
          this.setCenter();
          const img = this.$refs.imgex;
          const container = this.$refs.container;
          if (
            img &&
            this.fullImageLoaded &&
            (!img.clientWidth || !img.clientHeight)
          ) {
            requestAnimationFrame(() => this.setCenter());
          }
          // Force layout + compositor to pick up the decoded bitmap. Without this,
          // WebKit/Blink sometimes leave the promoted layer black until display/position
          // is toggled in DevTools or the window is resized.
          requestAnimationFrame(() => {
            if (!img || !this.fullImageLoaded) {
              return;
            }
            void container?.offsetHeight;
            void img.offsetHeight;
            const prevOp = img.style.opacity;
            img.style.opacity = '0.9999';
            requestAnimationFrame(() => {
              img.style.opacity = prevOp;
            });
          });
        });
      });
    },
    onResize: throttle(function () {
      if (this.imageLoaded) {
        this.setCenter();
      }
    }, 100),
    rubberband(value, max) {
      const sign = value < 0 ? -1 : 1;
      const a = Math.abs(value);
      if (a <= max) {
        return value;
      }
      return sign * (max + (a - max) * 0.32);
    },
    applyImgTransform() {
      const img = this.$refs.imgex;
      if (!img) {
        return;
      }
      const transition = this.gestureSnapBack
        ? 'transform 0.22s cubic-bezier(0.32, 0.72, 0, 1)'
        : 'none';
      img.style.transition = transition;
      if (this.scale === 1) {
        img.style.transform = `translate3d(calc(-50% + ${this.dragOffsetX}px), calc(-50% + ${this.dragOffsetY}px), 0) scale(1)`;
      } else {
        const { x: rx, y: ry } = this.position.relative;
        img.style.transform = `translate3d(calc(-50% + ${rx}px), calc(-50% + ${ry}px), 0) scale(${this.scale})`;
      }
    },
    decideEdgeKind() {
      if (this.edgeKind) {
        return;
      }
      const ax = Math.abs(this.edgeDx);
      const ay = Math.abs(this.edgeDy);
      if (ax < 12 && ay < 12) {
        return;
      }
      if (this.edgeDy > ax * 1.12 && this.edgeDy > 14) {
        this.edgeKind = 'vertical-dismiss';
        this.gestureDecided = true;
      } else if (ax > ay * 1.12 && ax > 14) {
        this.edgeKind = 'horizontal';
        this.gestureDecided = true;
      }
    },
    applyEdgeVisuals() {
      if (!this.edgeKind) {
        const ax = Math.abs(this.edgeDx);
        const ay = Math.abs(this.edgeDy);
        if (ax <= 8 && ay <= 8) {
          this.dragOffsetX = 0;
          this.dragOffsetY = 0;
          this.showNavHint = false;
          this.showDismissHint = false;
          this.applyImgTransform();
          this.syncNavigationGestureHintToStore();
          return;
        }
        if (ax > ay) {
          this.dragOffsetX = this.rubberband(this.edgeDx, this.edgeRubberMax);
          this.dragOffsetY = 0;
          this.showNavHint = ax >= this.edgeHintPx;
          this.navHintDir = this.edgeDx > 0 ? 'prev' : 'next';
          if (this.navHintDir === 'prev' && !this.hasImagePrevious) {
            this.showNavHint = false;
          }
          if (this.navHintDir === 'next' && !this.hasImageNext) {
            this.showNavHint = false;
          }
          this.showDismissHint = false;
        } else {
          this.dragOffsetX = 0;
          const downward = this.edgeDy > 0 ? this.edgeDy : 0;
          this.dragOffsetY = this.rubberband(downward, this.edgeRubberMax);
          this.showDismissHint = this.edgeDy >= this.edgeHintPx;
          this.showNavHint = false;
        }
        this.applyImgTransform();
        this.syncNavigationGestureHintToStore();
        return;
      }
      if (this.edgeKind === 'horizontal') {
        this.dragOffsetX = this.rubberband(this.edgeDx, this.edgeRubberMax);
        this.dragOffsetY = 0;
        const adx = Math.abs(this.edgeDx);
        this.showNavHint = adx >= this.edgeHintPx;
        this.navHintDir = this.edgeDx > 0 ? 'prev' : 'next';
        if (this.navHintDir === 'prev' && !this.hasImagePrevious) {
          this.showNavHint = false;
        }
        if (this.navHintDir === 'next' && !this.hasImageNext) {
          this.showNavHint = false;
        }
        this.showDismissHint = false;
      } else {
        this.dragOffsetX = 0;
        const downward = this.edgeDy > 0 ? this.edgeDy : 0;
        this.dragOffsetY = this.rubberband(downward, this.edgeRubberMax);
        this.showDismissHint = this.edgeDy >= this.edgeHintPx;
        this.showNavHint = false;
      }
      this.applyImgTransform();
      this.syncNavigationGestureHintToStore();
    },
    snapBackEdgeGesture() {
      this.gestureSnapBack = true;
      this.dragOffsetX = 0;
      this.dragOffsetY = 0;
      this.showNavHint = false;
      this.showDismissHint = false;
      this.edgeKind = null;
      this.gestureDecided = false;
      this.edgeDx = 0;
      this.edgeDy = 0;
      this.applyImgTransform();
      mutations.setNavigationGestureHint({});
      setTimeout(() => {
        this.gestureSnapBack = false;
        this.applyImgTransform();
      }, 240);
    },
    resetEdgeGestureImmediate() {
      this.edgeKind = null;
      this.gestureDecided = false;
      this.edgeDx = 0;
      this.edgeDy = 0;
      this.dragOffsetX = 0;
      this.dragOffsetY = 0;
      this.showNavHint = false;
      this.showDismissHint = false;
      this.gestureSnapBack = false;
      this.dismissFlashActive = false;
      mutations.setNavigationGestureHint({});
    },
    finishEdgeGesture() {
      if (this.scale !== 1) {
        this.resetEdgeGestureImmediate();
        this.applyImgTransform();
        return;
      }
      let kind = this.edgeKind;
      if (!kind) {
        const ax = Math.abs(this.edgeDx);
        const ay = Math.abs(this.edgeDy);
        if (ax < this.edgeHintPx && ay < this.edgeHintPx) {
          this.snapBackEdgeGesture();
          return;
        }
        kind = ax >= ay ? 'horizontal' : 'vertical-dismiss';
      }
      if (kind === 'horizontal') {
        if (this.edgeDx >= this.edgeCommitX && this.hasImagePrevious) {
          this.$emit('navigate-previous');
          this.resetEdgeGestureImmediate();
          this.applyImgTransform();
          return;
        }
        if (this.edgeDx <= -this.edgeCommitX && this.hasImageNext) {
          this.$emit('navigate-next');
          this.resetEdgeGestureImmediate();
          this.applyImgTransform();
          return;
        }
      } else if (kind === 'vertical-dismiss') {
        if (this.edgeDy >= this.edgeCommitY) {
          this.dismissFlashActive = true;
          this.showDismissHint = true;
          this.dragOffsetX = 0;
          this.dragOffsetY = 0;
          this.edgeKind = null;
          this.gestureDecided = false;
          this.applyImgTransform();
          this.syncNavigationGestureHintToStore();
          setTimeout(() => {
            this.$emit('close-preview');
          }, 120);
          setTimeout(() => {
            this.dismissFlashActive = false;
            this.showDismissHint = false;
            mutations.setNavigationGestureHint({});
          }, 420);
          return;
        }
      }
      this.snapBackEdgeGesture();
    },
    teardownEdgeMouseListeners() {
      document.removeEventListener('mousemove', this.onEdgeMouseMove, true);
      document.removeEventListener('mouseup', this.onEdgeMouseUp, true);
      this.edgeMouseActive = false;
    },
    onEdgeMouseMove(event) {
      if (!this.edgeMouseActive || this.scale !== 1) {
        return;
      }
      this.edgeDx = event.clientX - this.edgeStartX;
      this.edgeDy = event.clientY - this.edgeStartY;
      this.decideEdgeKind();
      this.applyEdgeVisuals();
      event.preventDefault();
    },
    onEdgeMouseUp(event) {
      if (!this.edgeMouseActive) {
        return;
      }
      this.teardownEdgeMouseListeners();
      this.edgeDx = event.clientX - this.edgeStartX;
      this.edgeDy = event.clientY - this.edgeStartY;
      this.finishEdgeGesture();
      event.preventDefault();
    },
    setCenter() {
      const container = this.$refs.container;
      const img = this.$refs.imgex;
      if (!container || !img || !img.clientWidth || !img.clientHeight) {
        return;
      }
      this.position.relative = { x: 0, y: 0 };
      this.position.center.x = Math.floor((container.clientWidth - img.clientWidth) / 2);
      this.position.center.y = Math.floor((container.clientHeight - img.clientHeight) / 2);
      this.resetEdgeGestureImmediate();
      this.applyImgTransform();
    },
    mousedownStart(event) {
      if (event.button !== 0) return;
      if (this.scale === 1) {
        this.teardownEdgeMouseListeners();
        this.edgeMouseActive = true;
        this.edgeStartX = event.clientX;
        this.edgeStartY = event.clientY;
        this.edgeDx = 0;
        this.edgeDy = 0;
        this.edgeKind = null;
        this.gestureDecided = false;
        document.addEventListener('mousemove', this.onEdgeMouseMove, true);
        document.addEventListener('mouseup', this.onEdgeMouseUp, true);
        event.preventDefault();
        return;
      }
      this.lastX = null;
      this.lastY = null;
      this.inDrag = true;
      event.preventDefault();
    },
    mouseMove(event) {
      if (event.button !== 0) return;
      if (this.scale > 1 && this.inDrag) {
        this.doMove(event.movementX, event.movementY);
        event.preventDefault();
      }
    },
    mouseUp(event) {
      if (event.button !== 0) return;
      if (this.scale > 1) {
        this.inDrag = false;
        event.preventDefault();
      }
    },
    touchStart(event) {
      this.lastX = null;
      this.lastY = null;
      this.lastTouchDistance = null;
      if (event.targetTouches.length === 1 && this.scale === 1) {
        const touch = event.targetTouches[0];
        this.edgeStartX = touch.pageX;
        this.edgeStartY = touch.pageY;
        this.edgeDx = 0;
        this.edgeDy = 0;
        this.edgeKind = null;
        this.gestureDecided = false;
      } else {
        this.resetEdgeGestureImmediate();
        this.applyImgTransform();
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
      // Default is prevented via @touchmove.prevent so the browser cannot steal
      // vertical drags (pull-to-refresh) before our edge-gesture thresholds.
      if (event.targetTouches.length === 1 && this.scale === 1) {
        const touch = event.targetTouches[0];
        this.edgeDx = touch.pageX - this.edgeStartX;
        this.edgeDy = touch.pageY - this.edgeStartY;
        this.decideEdgeKind();
        this.applyEdgeVisuals();
        return;
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
      this.applyImgTransform();
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
        this.resetEdgeGestureImmediate();
      }
      this.applyImgTransform();
    },
    pxStringToNumber(style) {
      return +style.replace("px", "");
    },
    touchEnd(event) {
      if (this.scale === 1 && event.changedTouches.length > 0) {
        const t = event.changedTouches[0];
        this.edgeDx = t.pageX - this.edgeStartX;
        this.edgeDy = t.pageY - this.edgeStartY;
        this.finishEdgeGesture();
        event.preventDefault();
      } else if (this.scale > 1) {
        event.preventDefault();
      }
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
              this.imageLoaded = true;
              mutations.setLoading("preview-img", false);
              this.scheduleSetCenter();
            }
          }, 30000); // 30 second timeout
        });
      }
      
      this.scale = 1;
      this.position.relative = { x: 0, y: 0 };
      this.resetEdgeGestureImmediate();
      this.applyImgTransform();
    },
  },
};
</script>

<style>
.image-ex-container {
  width: 100%;
  height: 100%;
  min-height: 0;
  max-width: 100%;
  max-height: 100%;
  overflow: hidden;
  position: relative;
  display: flex;
  justify-content: center;
  /* Block browser pull-to-refresh / overscroll so vertical dismiss & edge gestures win */
  overscroll-behavior: none;
  touch-action: none;
}

.image-ex-img {
  max-width: 100%;
  max-height: 100%;
  position: absolute;
  top: 50%;
  left: 50%;
  /* translate3d tends to composite more reliably than translate with decoded <img> bitmaps */
  transform: translate3d(-50%, -50%, 0);
  object-fit: contain;
}

.image-loading-overlay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate3d(-50%, -50%, 0);
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
