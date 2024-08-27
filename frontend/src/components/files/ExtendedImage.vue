<template>
  <div
    class="image-ex-container"
    ref="container"
    @touchstart="touchStart"
    @touchmove="touchMove"
    @dblclick="zoomAuto"
    @mousedown="mousedownStart"
    @mousemove="mouseMove"
    @mouseup="mouseUp"
    @wheel="wheelMove"
  >
    <div v-if="!isLoaded">Loading image...</div>

    <img
      v-if="!isTiff && isLoaded"
      :src="src"
      class="image-ex-img"
      ref="imgex"
      @load="onLoad"
    />
    <canvas v-else-if="isLoaded" ref="imgex" class="image-ex-img"></canvas>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import throttle from "@/utils/throttle";
import { showError } from "@/notify";
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
    };
  },
  mounted() {
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
      if (this.src == undefined || this.$refs.imgex == undefined) {
        mutations.setLoading("preview-img", false);
        return;
      }
      this.isTiff = this.checkIfTiff(this.src);
      if (this.isTiff) {
        this.decodeTiff(this.src);
      } else {
        this.$refs.imgex.src = this.src;
      }

      this.scale = 1;
      this.setZoom();
      this.setCenter();
      mutations.setLoading("preview-img", false);
      this.showSpinner = false;
    },
  },
  methods: {
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
        showError("Error decoding TIFF");
        console.error("Error decoding TIFF:", error);
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
      let container = this.$refs.container;
      let img = this.$refs.imgex;

      this.position.center.x = Math.floor((container.clientWidth - img.clientWidth) / 2);
      this.position.center.y = Math.floor(
        (container.clientHeight - img.clientHeight) / 2
      );

      img.style.left = this.position.center.x + "px";
      img.style.top = this.position.center.y + "px";
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
      if (event.targetTouches.length < 2) {
        setTimeout(() => {
          this.touches = 0;
        }, 300);
        this.touches++;
        if (this.touches > 1) {
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
  },
};
</script>

<style>
.image-ex-container {
  max-width: 100%; /* Image container max width */
  max-height: 100%; /* Image container max height */
  overflow: hidden; /* Hide overflow if image exceeds container */
  position: relative; /* Required for absolute positioning of child */
  display: flex;
  justify-content: center;
}

.image-ex-img {
  max-width: 100%; /* Image max width */
  max-height: 100%; /* Image max height */
  position: absolute;
}
</style>
