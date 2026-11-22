<template>
  <div class="view-cube" :title="$t('threejs.viewCube')">
    <div class="view-cube__toolbar">
      <button
        type="button"
        class="view-cube__tool"
        :aria-label="$t('threejs.iso')"
        :title="$t('threejs.iso')"
        @click.stop="resetView"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" fill="currentColor" />
        </svg>
      </button>
      <button
        type="button"
        class="view-cube__tool"
        :aria-label="$t('threejs.rotateLeft')"
        :title="$t('threejs.rotateLeft')"
        @click.stop="rotateView(-1)"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path
            d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"
            fill="currentColor"
          />
        </svg>
      </button>
      <button
        type="button"
        class="view-cube__tool"
        :aria-label="$t('threejs.rotateRight')"
        :title="$t('threejs.rotateRight')"
        @click.stop="rotateView(1)"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false" style="transform: scaleX(-1)">
          <path
            d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"
            fill="currentColor"
          />
        </svg>
      </button>
    </div>

    <div class="view-cube__compass">
      <div class="view-cube__ring" aria-hidden="true"></div>
      <span
        v-for="direction in compassDirections"
        :key="direction.label"
        class="view-cube__direction"
        :style="direction.style"
        aria-hidden="true"
      >{{ direction.label }}</span>

      <div class="cube-stage">
        <div ref="cube" class="cube" :style="{ transform: cubeTransform }">
          <button
            v-for="face in faces"
            :key="face.key"
            type="button"
            :class="['cube-face', `cube-face--${face.pos}`, { 'cube-face--active': activeFace === face.pos }]"
            :aria-label="`${$t('threejs.viewCube')}: ${face.label}`"
            :title="face.label"
            @click.stop="selectView(face.view)"
          ><span>{{ face.label }}</span></button>
          <button
            v-for="edge in edgeTargets"
            :key="edge.key"
            type="button"
            class="cube-marker cube-edge"
            :style="targetStyle(edge)"
            :aria-label="`${$t('threejs.viewCube')}: ${edge.label}`"
            :title="edge.label"
            @click.stop="selectView(edge.view)"
          ></button>
          <button
            v-for="corner in cornerTargets"
            :key="corner.key"
            type="button"
            class="cube-marker cube-corner"
            :style="targetStyle(corner)"
            :aria-label="`${$t('threejs.viewCube')}: ${$t('threejs.iso')} ${corner.label}`"
            :title="corner.label"
            @click.stop="selectView(corner.view)"
          ></button>
        </div>
      </div>
    </div>

    <div class="view-cube__footer">
      <button
        type="button"
        class="view-cube__wcs"
        :aria-label="$t('threejs.viewCube')"
        :aria-expanded="wcsOpen"
        aria-haspopup="menu"
        title="WCS — World Coordinate System"
        @click.stop="wcsOpen = !wcsOpen"
      >
        {{ $t('threejs.wcs') }}<span class="view-cube__caret" aria-hidden="true"></span>
      </button>
      <div v-if="wcsOpen" class="view-cube__menu" role="menu" :aria-label="$t('threejs.viewCube')">
        <button
          v-for="view in wcsViews"
          :key="view.key"
          type="button"
          role="menuitem"
          class="view-cube__menu-item"
          :class="{ 'view-cube__menu-item--active': activeFace === view.key }"
          @click.stop="chooseWcs(view.key)"
        >{{ view.label }}</button>
      </div>
    </div>
  </div>
</template>

<script>
const FACE_KEYS = {
  x: (value) => value > 0 ? "right" : "left",
  y: (value) => value > 0 ? "top" : "bottom",
  z: (value) => value > 0 ? "front" : "back",
};

const CUBE_HALF = 23;
const COMPASS_RADIUS = 60;

function orientationTarget(x, y, z, up = [0, 1, 0]) {
  return { cameraOffset: [x, y, z], up };
}

export default {
  name: "ViewCube",
  props: {
    /** Returns the camera basis in world coordinates: right, up, and back. */
    getOrientation: { type: Function, required: true },
  },
  data() {
    return {
      cubeTransform: "matrix3d(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1)",
      compassAngle: 0,
      activeFace: "front",
      orientationSignature: "",
      wcsOpen: false,
      rafId: null,
    };
  },
  computed: {
    faces() {
      return [
        { pos: "top", key: "top", x: 0, y: 1, z: 0, up: [0, 0, -1] },
        { pos: "bottom", key: "bottom", x: 0, y: -1, z: 0, up: [0, 0, 1] },
        { pos: "right", key: "right", x: 1, y: 0, z: 0 },
        { pos: "left", key: "left", x: -1, y: 0, z: 0 },
        { pos: "front", key: "front", x: 0, y: 0, z: 1 },
        { pos: "back", key: "back", x: 0, y: 0, z: -1 },
      ].map((face) => ({
        ...face,
        label: this.faceLabel(face.key),
        view: orientationTarget(face.x, face.y, face.z, face.up),
      }));
    },
    edgeTargets() {
      const targets = [];
      for (const x of [-1, 0, 1]) {
        for (const y of [-1, 0, 1]) {
          for (const z of [-1, 0, 1]) {
            if ([x, y, z].filter(Boolean).length !== 2) continue;
            targets.push({
              key: `edge-${x}-${y}-${z}`,
              view: orientationTarget(x, y, z),
              position: { x, y, z },
              label: this.targetLabel(x, y, z),
            });
          }
        }
      }
      return targets;
    },
    cornerTargets() {
      const targets = [];
      for (const x of [-1, 1]) {
        for (const y of [-1, 1]) {
          for (const z of [-1, 1]) {
            targets.push({
              key: `corner-${x}-${y}-${z}`,
              view: orientationTarget(x, y, z),
              position: { x, y, z },
              label: this.targetLabel(x, y, z),
            });
          }
        }
      }
      return targets;
    },
    wcsViews() {
      return ["iso", "top", "bottom", "front", "back", "right", "left"].map((key) => ({
        key,
        label: key === "iso" ? this.$t("threejs.iso") : this.faceLabel(key),
      }));
    },
    /** Compass letters follow the world +Z direction projected on screen. */
    compassDirections() {
      return [
        { label: "N", offset: 0 },
        { label: "E", offset: Math.PI / 2 },
        { label: "S", offset: Math.PI },
        { label: "W", offset: (3 * Math.PI) / 2 },
      ].map(({ label, offset }) => {
        const angle = this.compassAngle + offset;
        const dx = Math.sin(angle) * COMPASS_RADIUS;
        const dy = -Math.cos(angle) * COMPASS_RADIUS;
        return {
          label,
          style: {
            left: `calc(50% + ${dx.toFixed(1)}px)`,
            top: `calc(50% + ${dy.toFixed(1)}px)`,
          },
        };
      });
    },
  },
  mounted() {
    document.addEventListener("click", this.onDocumentClick);
    document.addEventListener("keydown", this.onDocumentKeydown);
    const tick = () => {
      this.syncRotation();
      this.rafId = requestAnimationFrame(tick);
    };
    this.rafId = requestAnimationFrame(tick);
  },
  beforeUnmount() {
    document.removeEventListener("click", this.onDocumentClick);
    document.removeEventListener("keydown", this.onDocumentKeydown);
    if (this.rafId) cancelAnimationFrame(this.rafId);
  },
  methods: {
    faceLabel(key) {
      const labels = {
        right: this.$t("threejs.right"),
        left: this.$t("threejs.left"),
        top: this.$t("threejs.top"),
        bottom: this.$t("threejs.bottom"),
        front: this.$t("threejs.front"),
        back: this.$t("threejs.back"),
      };
      return labels[key];
    },
    targetLabel(x, y, z) {
      return [
        x ? this.faceLabel(FACE_KEYS.x(x)) : "",
        y ? this.faceLabel(FACE_KEYS.y(y)) : "",
        z ? this.faceLabel(FACE_KEYS.z(z)) : "",
      ].filter(Boolean).join(" · ");
    },
    targetStyle(target) {
      const { x, y, z } = target.position;
      return {
        left: `calc(50% + ${x * CUBE_HALF}px)`,
        top: `calc(50% - ${y * CUBE_HALF}px)`,
        transform: `translate(-50%, -50%) translateZ(${z * CUBE_HALF}px)`,
      };
    },
    syncRotation() {
      if (!this.$refs.cube) return;
      const basis = this.getOrientation();
      if (!basis?.right || !basis?.up || !basis?.back) return;
      const [r, u, b] = [basis.right, basis.up, basis.back];
      const values = [
        r[0], u[0], b[0], 0,
        r[1], u[1], b[1], 0,
        r[2], u[2], b[2], 0,
        0, 0, 0, 1,
      ];
      const rounded = values.map((value) => Number(value.toFixed(5)));
      const transform = `matrix3d(${rounded.join(",")})`;
      // North points along world +Z as seen on screen.
      const angle = Math.atan2(r[2], u[2]);
      const signature = `${transform}|${angle.toFixed(4)}`;
      if (signature === this.orientationSignature) return;
      this.orientationSignature = signature;
      this.cubeTransform = transform;
      this.compassAngle = angle;
      // Highlight the face the camera is currently looking at.
      const candidates = [
        ["right", b[0]], ["left", -b[0]],
        ["top", b[1]], ["bottom", -b[1]],
        ["front", b[2]], ["back", -b[2]],
      ];
      this.activeFace = candidates.reduce(
        (best, candidate) => candidate[1] > best[1] ? candidate : best,
        candidates[0],
      )[0];
    },
    /** Orbit the camera a quarter turn around the current view-up axis. */
    rotateView(direction) {
      const basis = this.getOrientation();
      if (!basis?.right || !basis?.up) return;
      const offset = direction < 0
        ? basis.right.map((value) => -value)
        : basis.right.slice();
      this.selectView({ cameraOffset: offset, up: basis.up.slice() });
    },
    resetView() {
      this.selectView("iso");
    },
    selectView(view) {
      this.wcsOpen = false;
      this.$emit("select", view);
    },
    chooseWcs(key) {
      this.selectView(key);
    },
    onDocumentClick(event) {
      if (this.wcsOpen && this.$el && !this.$el.contains(event.target)) {
        this.wcsOpen = false;
      }
    },
    onDocumentKeydown(event) {
      if (event.key === "Escape" && this.wcsOpen) {
        this.wcsOpen = false;
      }
    },
  },
};
</script>

<style scoped>
.view-cube {
  position: absolute;
  right: 0.9rem;
  bottom: 0.9rem;
  z-index: 5;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 6px 6px 5px;
  border: 1px solid rgb(255 255 255 / 18%);
  border-radius: 12px;
  background: rgb(20 32 48 / 55%);
  box-shadow: 0 6px 20px rgb(0 0 0 / 25%);
  backdrop-filter: blur(8px);
  user-select: none;
}

.view-cube__toolbar {
  display: flex;
  height: 20px;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.view-cube__tool {
  display: inline-flex;
  width: 20px;
  height: 20px;
  min-width: 0;
  min-height: 0;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border: 0;
  border-radius: 5px;
  color: #cbd8e6;
  background: transparent;
  cursor: pointer;
}

.view-cube__tool svg {
  width: 100%;
  height: 100%;
}

.view-cube__tool:hover,
.view-cube__tool:focus-visible {
  color: #fff;
  background: rgb(255 255 255 / 12%);
}

.view-cube__compass {
  position: relative;
  width: 136px;
  height: 136px;
}

.view-cube__ring {
  position: absolute;
  inset: 14px;
  border: 6px solid rgb(205 218 231 / 88%);
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px rgb(12 24 40 / 25%), 0 1px 4px rgb(0 0 0 / 30%);
}

.view-cube__direction {
  position: absolute;
  z-index: 3;
  color: #eaf2fb;
  font-size: 0.62rem;
  font-weight: 700;
  line-height: 1;
  text-shadow: 0 1px 2px rgb(0 0 0 / 70%);
  transform: translate(-50%, -50%);
}

.cube-stage {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 56px;
  height: 56px;
  perspective: 300px;
  transform: translate(-50%, -50%);
}

.cube {
  position: absolute;
  top: 5px;
  left: 5px;
  width: 46px;
  height: 46px;
  transform-style: preserve-3d;
}

.cube-face {
  position: absolute;
  display: flex;
  width: 46px;
  height: 46px;
  min-width: 0;
  min-height: 0;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid #9fb0c2;
  color: #26384c;
  background: linear-gradient(150deg, #f8fafc 0%, #d3dee9 100%);
  box-shadow: inset 0 0 8px rgb(255 255 255 / 55%);
  cursor: pointer;
  backface-visibility: hidden;
}

.cube-face:hover,
.cube-face:focus-visible {
  color: #14476e;
  background: linear-gradient(150deg, #eaf4ff 0%, #bcd8f2 100%);
}

.cube-face span {
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  font-size: 0.47rem;
  font-weight: 700;
  letter-spacing: 0;
  text-align: center;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.cube-face--top { transform: rotateX(90deg) translateZ(23px); }
.cube-face--bottom { transform: rotateX(-90deg) translateZ(23px); }
.cube-face--right { transform: rotateY(90deg) translateZ(23px); }
.cube-face--left { transform: rotateY(-90deg) translateZ(23px); }
.cube-face--front { transform: translateZ(23px); }
.cube-face--back { transform: rotateY(180deg) translateZ(23px); }

.cube-face--active {
  border-color: #4a90d9;
  box-shadow: inset 0 0 0 1px rgb(74 144 217 / 55%), inset 0 0 8px rgb(255 255 255 / 55%);
}

.cube-marker {
  position: absolute;
  z-index: 2;
  min-width: 0;
  min-height: 0;
  padding: 0;
  border: 1px solid rgb(255 255 255 / 90%);
  background: rgb(235 242 249 / 85%);
  box-shadow: 0 0 0 1px rgb(24 53 82 / 45%);
  cursor: pointer;
}

.cube-marker:hover,
.cube-marker:focus-visible {
  background: #fff;
  box-shadow: 0 0 0 2px rgb(66 165 245 / 85%);
}

.cube-edge {
  width: 7px;
  height: 7px;
  border-radius: 2px;
}

.cube-corner {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}

.view-cube__footer {
  position: relative;
  display: flex;
  justify-content: center;
}

.view-cube__wcs {
  display: inline-flex;
  min-width: 0;
  min-height: 0;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border: 1px solid rgb(255 255 255 / 25%);
  border-radius: 5px;
  color: #dce8f4;
  background: rgb(255 255 255 / 8%);
  font-size: 0.6rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  cursor: pointer;
}

.view-cube__wcs:hover,
.view-cube__wcs:focus-visible,
.view-cube__wcs[aria-expanded="true"] {
  border-color: rgb(120 180 235 / 70%);
  background: rgb(255 255 255 / 16%);
}

.view-cube__menu {
  position: absolute;
  right: 0;
  bottom: calc(100% + 6px);
  z-index: 6;
  display: flex;
  width: 116px;
  flex-direction: column;
  padding: 4px;
  border: 1px solid rgb(255 255 255 / 22%);
  border-radius: 7px;
  background: rgb(17 28 42 / 94%);
  box-shadow: 0 6px 18px rgb(0 0 0 / 35%);
  backdrop-filter: blur(8px);
}

.view-cube__menu-item {
  min-width: 0;
  min-height: 0;
  padding: 5px 8px;
  border: 0;
  border-radius: 4px;
  color: #dce8f4;
  background: transparent;
  font-size: 0.65rem;
  text-align: left;
  cursor: pointer;
}

.view-cube__menu-item:hover,
.view-cube__menu-item:focus-visible {
  color: #fff;
  background: rgb(83 144 195 / 55%);
}

.view-cube__menu-item--active {
  color: #fff;
  background: rgb(74 144 217 / 35%);
}

.view-cube__caret {
  width: 0;
  height: 0;
  border-top: 4px solid currentcolor;
  border-right: 3px solid transparent;
  border-left: 3px solid transparent;
  opacity: 0.8;
}

.cube-face:focus-visible,
.cube-marker:focus-visible,
.view-cube__tool:focus-visible,
.view-cube__wcs:focus-visible,
.view-cube__menu-item:focus-visible {
  outline: 2px solid #fff;
  outline-offset: 2px;
}
</style>
