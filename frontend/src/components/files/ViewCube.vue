<template>
  <div class="view-cube" :title="$t('threejs.viewCube')">
    <div class="cube-stage">
      <div ref="cube" class="cube">
        <div
          v-for="face in faces"
          :key="face.view"
          :class="['cube-face', `cube-face--${face.pos}`]"
          @click="$emit('select', face.view)"
        >
          <span>{{ face.label }}</span>
        </div>
      </div>
    </div>
    <button class="cube-iso" @click="$emit('select', 'iso')">
      <i class="material-symbols-outlined">view_in_ar</i>
      <span>ISO</span>
    </button>
  </div>
</template>

<script>
export default {
  name: "ViewCube",
  props: {
    /** () => {x, y, z}: the camera's normalized view direction. */
    getDirection: { type: Function, required: true },
    labels: {
      type: Object,
      default: () => ({
        top: "SUP", bottom: "INF", right: "DER", left: "IZQ",
        front: "FRONT", back: "BACK",
      }),
    },
  },
  data() {
    return {
      faces: [
        { pos: "top", view: "top", label: this.labels.top },
        { pos: "bottom", view: "bottom", label: this.labels.bottom },
        { pos: "right", view: "right", label: this.labels.right },
        { pos: "left", view: "left", label: this.labels.left },
        { pos: "front", view: "front", label: this.labels.front },
        { pos: "back", view: "back", label: this.labels.back },
      ],
      rafId: null,
    };
  },
  mounted() {
    const tick = () => {
      this.syncRotation();
      this.rafId = requestAnimationFrame(tick);
    };
    this.rafId = requestAnimationFrame(tick);
  },
  beforeUnmount() {
    if (this.rafId) cancelAnimationFrame(this.rafId);
  },
  methods: {
    // Mirror the camera orientation on the cube: yaw/pitch of the view
    // direction rotate the CSS cube the same way the model is seen.
    syncRotation() {
      if (!this.$refs.cube) return;
      const dir = this.getDirection();
      const yaw = Math.atan2(dir.x, dir.z);
      const pitch = Math.asin(Math.max(-1, Math.min(1, -dir.y)));
      this.$refs.cube.style.transform =
        `rotateX(${pitch}rad) rotateY(${yaw}rad)`;
    },
  },
};
</script>

<style scoped>
.view-cube {
  position: absolute;
  right: 0.9rem;
  bottom: 0.9rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  z-index: 5;
  user-select: none;
}

.cube-stage {
  width: 64px;
  height: 64px;
  perspective: 260px;
}

.cube {
  width: 100%;
  height: 100%;
  position: relative;
  transform-style: preserve-3d;
  transform: rotateX(-0.4rad) rotateY(0.6rad);
  transition: transform 0.08s linear;
}

.cube-face {
  position: absolute;
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(38, 68, 120, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.45);
  color: #fff;
  font-size: 0.62rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  cursor: pointer;
  backdrop-filter: blur(2px);
}

.cube-face:hover {
  background: rgba(90, 150, 235, 0.75);
}

.cube-face span {
  transform: scale(1); /* keep text upright enough on small labels */
}

.cube-face--top { transform: rotateX(90deg) translateZ(32px); }
.cube-face--bottom { transform: rotateX(-90deg) translateZ(32px); }
.cube-face--right { transform: rotateY(90deg) translateZ(32px); }
.cube-face--left { transform: rotateY(-90deg) translateZ(32px); }
.cube-face--front { transform: translateZ(32px); }
.cube-face--back { transform: rotateY(180deg) translateZ(32px); }

.cube-iso {
  display: flex;
  align-items: center;
  gap: 4px;
  border: 1px solid rgba(255, 255, 255, 0.4);
  background: rgba(0, 0, 0, 0.4);
  color: #fff;
  border-radius: 7px;
  padding: 3px 9px;
  font-size: 0.72rem;
  cursor: pointer;
}

.cube-iso:hover {
  background: rgba(90, 150, 235, 0.6);
}

.cube-iso i {
  font-size: 15px;
}
</style>
