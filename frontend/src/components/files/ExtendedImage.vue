<template>
  <div class="image-ex-container" ref="container" @touchstart="touchStart" @touchmove="touchMove" @touchend="touchEnd" @dblclick="zoomAuto"
    @mousedown="mousedownStart" @mousemove="mouseMove" @mouseup="mouseUp" @wheel="wheelMove">
    <div v-if="!isLoaded">{{ $t('files.loading') }}</div>

    <img v-if="!isTiff && isLoaded" :src="src" class="image-ex-img" ref="imgex" @load="onLoad" />
    <canvas v-else-if="isLoaded" ref="imgex" class="image-ex-img"></canvas>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import throttle from "@/utils/throttle";
import { notify } from "@/notify";
export default {
  props: {
    src: String,
    moveDisabledTime: {
      type: Number,
      default: () => 200,
    },
    classList: {
      type: Array,
      default: () => [],
    },
    zoomStep: {
      type: Number,
      default: () => 0.25,
    },
  },
  data() {
    return {
      scale: 1,
      lastX: null,
      lastY: null,
      inDrag: false,
      touches: 0,
      lastTouchDistance: 0,
      moveDisabled: false,
      disabledTimer: null,
      imageLoaded: false,
      position: {
        center: { x: 0, y: 0 },
        relative: { x: 0, y: 0 },
      },
      maxScale: 4,
      minScale: 0.25,
      isTiff: false, // Determine if the image is a TIFF
      // Swipe navigation properties
      swipeStartTime: null,
      swipeStartX: 0,
      swipeStartY: 0,
      swipeCurrentX: 0,
      swipeCurrentY: 0,
      isSwipeGesture: false,
      hasStartedSwipe: false,
      swipeMinDistance: 80, // Minimum horizontal distance for swipe
      swipeMaxTime: 500, // Maximum time for swipe in milliseconds
      swipeMaxVerticalDistance: 50, // Maximum vertical movement to still be considered horizontal swipe
    };
  },
  mounted() {
    console.log('🚀 ExtendedImage mounted with swipe config:', {
      swipeMinDistance: this.swipeMinDistance,
      swipeMaxTime: this.swipeMaxTime,
      swipeMaxVerticalDistance: this.swipeMaxVerticalDistance
    });
    
    this.isTiff = this.checkIfTiff(this.src);
    if (this.isTiff) {
      this.decodeTiff(this.src);
    } else {
      this.$refs.imgex.src = this.src;
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
    window.removeEventListener("resize", this.onResize);
    document.removeEventListener("mouseup", this.onMouseUp);
  },
  computed: {
    isLoaded() {
      return !("preview-img" in state.loading);
    },
  },
  watch: {
    src: function () {
      if (!this.src || !this.$refs.imgex) {
        mutations.setLoading("preview-img", false);
        return;
      }
      this.isTiff = this.checkIfTiff(this.src);
      if (this.isTiff) {
        this.decodeTiff(this.src);
      } else {
        this.$refs.imgex.src = this.src;
      }
      this.scale = 1; // Reset zoom level
      this.position.relative = { x: 0, y: 0 }; // Reset position
      this.showSpinner = true; // Show spinner while loading
      this.resetSwipeTracking(); // Reset swipe tracking for new image
    },
  },
  methods: {
    onLoad() {
      this.imageLoaded = true;
      this.setCenter(); // Center the image after loading
      this.showSpinner = false;
      mutations.setLoading("preview-img", false);
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
        const blob = await response.blob(); // Convert response to a blob
        const imgex = this.$refs.imgex;

        if (imgex) {
          // Create a URL for the blob and set it as the image source
          imgex.src = URL.createObjectURL(blob);
          imgex.onload = () => URL.revokeObjectURL(imgex.src); // Clean up URL object after loading
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
        return; // Exit if dimensions are unavailable
      }

      this.position.center.x = Math.floor((container.clientWidth - img.clientWidth) / 2);
      this.position.center.y = Math.floor((container.clientHeight - img.clientHeight) / 2);

      img.style.left = `${this.position.center.x}px`;
      img.style.top = `${this.position.center.y}px`;
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
      console.log('🟢 TouchStart - touches:', event.targetTouches.length);
      this.lastX = null;
      this.lastY = null;
      this.lastTouchDistance = null;
      
      // Initialize swipe tracking for single touch
      if (event.targetTouches.length === 1) {
        const touch = event.targetTouches[0];
        this.swipeStartTime = Date.now();
        this.swipeStartX = touch.pageX;
        this.swipeStartY = touch.pageY;
        this.swipeCurrentX = touch.pageX;
        this.swipeCurrentY = touch.pageY;
        this.isSwipeGesture = false;
        this.hasStartedSwipe = false;
        
        console.log('📱 Swipe tracking initialized:', {
          startTime: this.swipeStartTime,
          startX: this.swipeStartX,
          startY: this.swipeStartY
        });
      } else {
        // Reset swipe tracking for multi-touch (zoom gestures)
        console.log('👆 Multi-touch detected, resetting swipe tracking');
        this.resetSwipeTracking();
      }
      
      if (event.targetTouches.length < 2) {
        setTimeout(() => {
          this.touches = 0;
        }, 300);
        this.touches++;
        if (this.touches > 1) {
          console.log('🔄 Double tap detected, triggering zoomAuto');
          this.zoomAuto(event);
        }
      }
      event.preventDefault();
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
      event.preventDefault();
      
      // Update current swipe position for single touch
      if (event.targetTouches.length === 1) {
        const touch = event.targetTouches[0];
        this.swipeCurrentX = touch.pageX;
        this.swipeCurrentY = touch.pageY;
        
        console.log('📱 TouchMove single touch:', {
          currentX: this.swipeCurrentX,
          currentY: this.swipeCurrentY,
          hasStartedSwipe: this.hasStartedSwipe,
          isSwipeGesture: this.isSwipeGesture
        });
        
        // Check if this is a potential swipe gesture
        if (!this.hasStartedSwipe && !this.isSwipeGesture) {
          const deltaX = Math.abs(this.swipeCurrentX - this.swipeStartX);
          const deltaY = Math.abs(this.swipeCurrentY - this.swipeStartY);
          
          console.log('🔍 Movement analysis:', {
            deltaX,
            deltaY,
            horizontalRatio: deltaX / (deltaY || 1),
            threshold: deltaY * 2
          });
          
          // Determine if this is a horizontal swipe or image manipulation
          if (deltaX > 10 || deltaY > 10) { // Some movement threshold
            if (deltaX > deltaY * 2) { // Horizontal movement is significantly more than vertical
              this.isSwipeGesture = true;
              console.log('✅ Horizontal swipe gesture detected!');
              return; // Don't process as image pan if it's a swipe
            } else {
              this.hasStartedSwipe = false; // Not a swipe, allow normal pan behavior
              console.log('❌ Not a horizontal swipe, treating as pan gesture');
            }
          }
        }
        
        // If it's determined to be a swipe gesture, don't do normal panning
        if (this.isSwipeGesture) {
          console.log('🚫 Blocking pan behavior - this is a swipe');
          return;
        }
      }
      
      // Existing touch move logic for pan/zoom
      console.log('🖼️ Processing as image pan/zoom');
      if (this.lastX === null) {
        this.lastX = event.targetTouches[0].pageX;
        this.lastY = event.targetTouches[0].pageY;
        return;
      }
      let step = this.$refs.imgex.width / 5;
      if (event.targetTouches.length === 2) {
        console.log('🔍 Two-finger zoom gesture');
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
      } else if (event.targetTouches.length === 1) {
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
      // Update the transform with separate translate and scale values
      this.$refs.imgex.style.transform = `translate(${this.position.relative.x}px, ${this.position.relative.y}px) scale(${this.scale})`;
    },
    wheelMove(event) {
      event.preventDefault()
      this.scale += -Math.sign(event.deltaY) * this.zoomStep;
      this.setZoom();
    },
    setZoom() {
      this.scale = Math.max(this.minScale, Math.min(this.maxScale, this.scale));

      // Update the transform with both translate and scale values
      this.$refs.imgex.style.transform = `translate(${this.position.relative.x}px, ${this.position.relative.y}px) scale(${this.scale})`;
    },
    pxStringToNumber(style) {
      return +style.replace("px", "");
    },
    touchEnd(event) {
      event.preventDefault();
      console.log('🔴 TouchEnd triggered');
      
      // Only process swipe if it was a single touch and we detected a swipe gesture
      if (this.isSwipeGesture && this.swipeStartTime) {
        const swipeEndTime = Date.now();
        const swipeDuration = swipeEndTime - this.swipeStartTime;
        const deltaX = this.swipeCurrentX - this.swipeStartX;
        const deltaY = Math.abs(this.swipeCurrentY - this.swipeStartY);
        const absDelataX = Math.abs(deltaX);
        
        console.log('🏁 Swipe validation:', {
          isSwipeGesture: this.isSwipeGesture,
          hasStartTime: !!this.swipeStartTime,
          duration: swipeDuration,
          maxDuration: this.swipeMaxTime,
          deltaX: deltaX,
          absDeltaX: absDelataX,
          minDistance: this.swipeMinDistance,
          deltaY: deltaY,
          maxVertical: this.swipeMaxVerticalDistance
        });
        
        // Check if swipe meets criteria: fast, horizontal, and long enough
        if (
          swipeDuration <= this.swipeMaxTime &&
          absDelataX >= this.swipeMinDistance &&
          deltaY <= this.swipeMaxVerticalDistance
        ) {
          console.log('✅ Swipe criteria met!');
          // Determine swipe direction and emit navigation event
          if (deltaX > 0) {
            // Swiped right (show previous image)
            console.log('➡️ Swiped RIGHT - emitting navigate-previous');
            this.$emit('navigate-previous');
          } else {
            // Swiped left (show next image)
            console.log('⬅️ Swiped LEFT - emitting navigate-next');
            this.$emit('navigate-next');
          }
        } else {
          console.log('❌ Swipe criteria NOT met:', {
            durationCheck: swipeDuration <= this.swipeMaxTime,
            distanceCheck: absDelataX >= this.swipeMinDistance,
            verticalCheck: deltaY <= this.swipeMaxVerticalDistance
          });
        }
      } else {
        console.log('🚫 No swipe to process:', {
          isSwipeGesture: this.isSwipeGesture,
          hasStartTime: !!this.swipeStartTime
        });
      }
      
      // Reset swipe tracking
      console.log('🔄 Resetting swipe tracking');
      this.resetSwipeTracking();
    },
    resetSwipeTracking() {
      console.log('🔄 Resetting swipe tracking - previous values:', {
        swipeStartTime: this.swipeStartTime,
        isSwipeGesture: this.isSwipeGesture,
        hasStartedSwipe: this.hasStartedSwipe
      });
      
      this.swipeStartTime = null;
      this.swipeStartX = 0;
      this.swipeStartY = 0;
      this.swipeCurrentX = 0;
      this.swipeCurrentY = 0;
      this.isSwipeGesture = false;
      this.hasStartedSwipe = false;
      
      console.log('✅ Swipe tracking reset complete');
    },
  },
};
</script>

<style>
.image-ex-container {
  max-width: 100%;
  /* Image container max width */
  max-height: 100%;
  /* Image container max height */
  overflow: hidden;
  /* Hide overflow if image exceeds container */
  position: relative;
  /* Required for absolute positioning of child */
  display: flex;
  justify-content: center;
}

.image-ex-img {
  max-width: 100%;
  /* Image max width */
  max-height: 100%;
  /* Image max height */
  position: absolute;
}
</style>
