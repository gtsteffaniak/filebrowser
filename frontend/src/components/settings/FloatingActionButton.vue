<template>
  <div
    v-if="showZone"
    ref="zoneEl"
    class="fab-zone"
    :class="`fab-zone--${position}`"
    :style="zoneStyle"
  ></div>

  <button
    type="button"
    v-if="showButton"
    class="fab-button floating"
    :class="[
      `fab-button--${position}`,
      `fab-button--${effectiveSize}`,
      `fab-button--${variant}`,
      {
        'dark-mode': darkMode,
        'fab-button--extended': extended,
        'fab-button--slide-in-visible': slideInVisible,
      },
    ]"
    :style="buttonStyle"
    :disabled="disabled"
    @click="handleClick"
    @touchstart="resetButtonTimer"
    @pointerenter="setZoneActive(true, $event)"
    @pointerleave="setZoneActive(false, $event)"
    :aria-label="label"
    :title="label"
  >
    <i :class="iconOutlined ? 'material-symbols-outlined' : 'material-symbols'">{{ icon }}</i>
    <span v-if="extended && label" class="fab-label">{{ label }}</span>
    <span v-if="badge" class="fab-badge">{{ badge }}</span>
  </button>
</template>

<script lang="ts">
import { reactive } from "vue";
import { state, getters } from "@/store";

const buttonGroup = new Map<string, { visible: boolean; zoneActive: boolean; timer: ReturnType<typeof setTimeout> | null }>();

function getGroupState(group: string) {
  let entry = buttonGroup.get(group);
  if (!entry) {
    entry = reactive({ visible: false, zoneActive: false, timer: null });
    buttonGroup.set(group, entry);
  }
  return entry;
}

const POSITIONS = ["top-right", "top-left", "bottom-right", "bottom-left", "top-center"] as const;

export default {
  name: "floatingActionButton",
  emits: ["click"],
  props: {
    icon: {
      type: String,
      default: "",
    },
    iconOutlined: {
      type: Boolean,
      default: false,
    },
    extended: {
      type: Boolean,
      default: false,
    },
    badge: {
      type: [String, Number],
      default: null,
    },
    position: {
      type: String,
      default: "top-right",
      validator: (v: string) => POSITIONS.includes(v as typeof POSITIONS[number]),
    },
    positionMode: {
      type: String,
      default: "fixed",
      validator: (v: string) => ["fixed", "absolute"].includes(v),
    },
    edgeOffset: {
      type: [String, Object],
      default: null,
    },
    size: {
      type: String,
      default: "normal",
      validator: (v: string) => ["normal", "small"].includes(v),
    },
    variant: {
      type: String,
      default: "neutral",
      validator: (v: string) => ["neutral", "primary"].includes(v),
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    label: {
      type: String,
      default: "",
    },
    offset: {
      type: Object,
      default: () => ({}),
    },
    autoHide: {
      type: Boolean,
      default: true,
    },
    autoHideDelay: {
      type: Number,
      default: 3000,
    },
    zoneWidth: {
      type: String,
      default: "5em",
    },
    zoneHeight: {
      type: String,
      default: "5em",
    },
    group: {
      type: String,
      default: "",
    },
    slideIn: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      buttonVisible: false,
      buttonZone: false,
      buttonTimer: null as ReturnType<typeof setTimeout> | null,
      pointerInsideZone: false,
      slideInVisible: false,
    };
  },
  computed: {
    darkMode(): boolean {
      return getters.isDarkMode();
    },
    effectiveSize(): string {
      return this.extended ? "normal" : this.size;
    },
    sharedState() {
      return this.group ? getGroupState(this.group) : null;
    },
    isRevealed(): boolean {
      return this.sharedState ? this.sharedState.visible : this.buttonVisible;
    },
    isZoneActive(): boolean {
      return this.sharedState ? this.sharedState.zoneActive : this.buttonZone;
    },
    showZone(): boolean {
      return this.autoHide;
    },
    showButton(): boolean {
      return !this.autoHide || this.isRevealed || this.isZoneActive;
    },
    normalizedEdgeOffset(): Record<string, string> {
      if (!this.edgeOffset) return {};
      if (typeof this.edgeOffset === "string") {
        return {
          top: this.edgeOffset,
          right: this.edgeOffset,
          bottom: this.edgeOffset,
          left: this.edgeOffset,
        };
      }
      const edge = this.edgeOffset as { top?: string; right?: string; bottom?: string; left?: string };
      const result: Record<string, string> = {};
      if (edge.top !== undefined) result.top = edge.top;
      if (edge.right !== undefined) result.right = edge.right;
      if (edge.bottom !== undefined) result.bottom = edge.bottom;
      if (edge.left !== undefined) result.left = edge.left;
      return result;
    },
    buttonStyle(): Record<string, string> {
      const style: Record<string, string> = {
        position: this.positionMode,
        ...(this.offset as Record<string, string>),
        ...this.normalizedEdgeOffset,
      };

      const isLeftPositioned = this.position === "top-left" || this.position === "bottom-left";
      const pushedBySidebar = isLeftPositioned && getters.isSidebarVisible() && getters.isStickySidebar();
      if (pushedBySidebar && style.left === undefined) {
        style.left = `calc(var(--fab-edge-offset-media) + ${state.sidebar.width}em)`;
      }

      return style;
    },
    zoneStyle(): Record<string, string> {
      return {
        width: this.zoneWidth,
        height: this.zoneHeight,
        position: this.positionMode,
      };
    },
  },
  watch: {
    slideIn(visible: boolean) {
      if (visible) {
        this.slideInVisible = true;
      }
    },
  },
  mounted() {
    if (this.slideIn) {
      requestAnimationFrame(() => {
        this.slideInVisible = true;
      });
    }
    if (this.autoHide) {
      this.resetButtonTimer();
      window.addEventListener("pointermove", this.handleGlobalPointerMove, { passive: true });
      window.addEventListener("touchstart", this.handleGlobalTouchStart, { passive: true });
    }
  },
  beforeUnmount() {
    if (this.buttonTimer) clearTimeout(this.buttonTimer);
    window.removeEventListener("pointermove", this.handleGlobalPointerMove);
    window.removeEventListener("touchstart", this.handleGlobalTouchStart);
  },
  methods: {
    isInsideZone(x: number, y: number): boolean {
      const el = this.$refs.zoneEl as HTMLElement | undefined;
      if (!el) return false;
      const rect = el.getBoundingClientRect();
      return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
    },
    handleGlobalPointerMove(event: PointerEvent) {
      if (event.pointerType !== "mouse") return;
      if (!this.autoHide) return;
      const inside = this.isInsideZone(event.clientX, event.clientY);
      if (inside !== this.pointerInsideZone) {
        this.pointerInsideZone = inside;
        this.setZoneActive(inside, event);
      } else if (inside) {
        this.resetButtonTimer();
      }
    },
    handleGlobalTouchStart(event: TouchEvent) {
      const touch = event.touches[0];
      if (!touch) return;
      if (!this.autoHide) return;
      if (this.isInsideZone(touch.clientX, touch.clientY)) this.resetButtonTimer();
    },
    setZoneActive(active: boolean, event?: PointerEvent) {
      if (event && event.pointerType !== "mouse") return;
      if (this.sharedState) {
        this.sharedState.zoneActive = active;
      } else {
        this.buttonZone = active;
      }
      if (active) this.resetButtonTimer();
    },
    resetButtonTimer() {
      if (!this.autoHide) return;
      const shared = this.sharedState;
      if (shared) {
        shared.visible = true;
        if (shared.timer) clearTimeout(shared.timer);
        shared.timer = setTimeout(() => {
          if (!shared.zoneActive) shared.visible = false;
          shared.timer = null;
        }, this.autoHideDelay);
        return;
      }
      this.buttonVisible = true;
      if (this.buttonTimer) clearTimeout(this.buttonTimer);
      this.buttonTimer = setTimeout(() => {
        if (!this.buttonZone) this.buttonVisible = false;
        this.buttonTimer = null;
      }, this.autoHideDelay);
    },
    handleClick(event: MouseEvent) {
      this.resetButtonTimer();
      this.$emit("click", event);
    },
  },
};
</script>

<style scoped>
.fab-zone {
  pointer-events: none;
  z-index: 1000;
  background: transparent;
}

.fab-zone--top-right {
  top: 4em;
  right: 0;
}

.fab-zone--top-left {
  top: 4em;
  left: 0;
}

.fab-zone--top-center {
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 8em !important;
}

.fab-zone--bottom-right {
  bottom: 0;
  right: 0;
}

.fab-zone--bottom-left {
  bottom: 0;
  left: 0;
}

.fab-button {
  width: var(--fab-size);
  height: var(--fab-size);
  border: none;
  border-radius: 50%;
  background: var(--background);
  color: var(--textPrimary);
  cursor: pointer;
  transition:
    background-color var(--fab-transition),
    color var(--fab-transition),
    transform var(--fab-transition),
    box-shadow var(--fab-transition),
    opacity var(--fab-transition);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--fab-shadow);
  outline: none;
  z-index: 9998;
  animation: fab-button-show 200ms cubic-bezier(0.2, 0, 0, 1);
}

.fab-button.dark-mode:not(.fab-button--primary) {
  background: var(--surfacePrimary);
}

.fab-button--neutral:hover:not(:disabled) {
  background: var(--primaryColor);
  transform: translateY(-2px) scale(1.05);
  box-shadow: var(--fab-elevation-hover);
  color: white;
}

.fab-button--primary,
.fab-button--primary.dark-mode {
  background: var(--primaryColor);
  color: white;
}

.fab-button--primary:hover:not(:disabled) {
  transform: translateY(-2px) scale(1.05);
  box-shadow: var(--fab-elevation-hover);
}

.fab-button:active:not(:disabled) {
  box-shadow: var(--fab-elevation-pressed);
}

.fab-button:focus-visible {
  outline: 2px solid var(--primaryColor);
  outline-offset: 2px;
}

.fab-button i.material-symbols,
.fab-button i.material-symbols-outlined {
  font-size: var(--fab-icon-size);
  transition: transform var(--fab-transition);
}

.fab-button--neutral:hover:not(:disabled) i.material-symbols,
.fab-button--neutral:hover:not(:disabled) i.material-symbols-outlined,
.fab-button--primary:hover:not(:disabled) i.material-symbols,
.fab-button--primary:hover:not(:disabled) i.material-symbols-outlined {
  transform: scale(1.1);
}

.fab-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

.fab-button--extended {
  width: auto;
  min-width: var(--fab-size);
  padding: 0 20px;
  gap: 8px;
  border-radius: calc(var(--fab-size) / 2);
}

.fab-label {
  font-size: 14px;
  font-weight: 500;
  line-height: 20px;
  white-space: nowrap;
}

.fab-button--small {
  width: var(--fab-size-small);
  height: var(--fab-size-small);
  min-width: var(--fab-size-small);
}

.fab-button--top-right {
  top: 80px;
  right: var(--fab-edge-offset-media);
}

.fab-button--top-left {
  top: 80px;
  left: var(--fab-edge-offset-media);
}

.fab-button--top-center {
  top: 0;
  left: 50%;
  transform: translate(-50%, -5em);
  animation: none;
  transition:
    transform 0.4s ease,
    background-color var(--fab-transition),
    color var(--fab-transition),
    box-shadow var(--fab-transition);
}

.fab-button--top-center.fab-button--slide-in-visible {
  transform: translate(-50%, 2.75em);
}

.fab-button--top-center.fab-button--neutral:hover:not(:disabled) {
  transform: translate(-50%, calc(2.75em - 2px)) scale(1.05);
}

.fab-button--top-center.fab-button--primary:hover:not(:disabled) {
  transform: translate(-50%, calc(2.75em - 2px)) scale(1.05);
}

.fab-button--bottom-right {
  bottom: calc(env(safe-area-inset-bottom, 0px) + var(--fab-edge-offset-media));
  right: calc(env(safe-area-inset-right, 0px) + var(--fab-edge-offset-media));
}

.fab-button--bottom-left {
  bottom: calc(env(safe-area-inset-bottom, 0px) + var(--fab-edge-offset-media));
  left: calc(env(safe-area-inset-left, 0px) + var(--fab-edge-offset-media));
}

.fab-button--slide-in {
  animation: none;
}

.fab-badge {
  position: absolute;
  top: -5px;
  right: -5px;
  background: var(--accentColor);
  color: white;
  border-radius: 50%;
  width: 20px;
  height: 20px;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  text-shadow:
    0 0 3px rgba(0, 0, 0, 0.9),
    0 0 5px rgba(0, 0, 0, 0.7),
    0 0 8px rgba(0, 0, 0, 0.5),
    0 0 8px rgba(0, 0, 0, 0.3);
}

@keyframes fab-button-show {
  0% {
    opacity: 0;
    transform: translateY(-2px) scale(0.8);
  }
  100% {
    opacity: 1;
    transform: translateY(-2px) scale(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .fab-button {
    animation: none;
    transition: background-color var(--fab-transition), color var(--fab-transition), box-shadow var(--fab-transition);
  }

  .fab-button--top-center.fab-button--slide-in-visible {
    transform: translate(-50%, 2.75em);
  }
}
</style>
