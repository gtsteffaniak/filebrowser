<template>
  <div
    v-if="showZone"
    ref="zoneEl"
    class="fab-zone"
    :class="`fab-zone--${position}`"
    :style="{ width: zoneWidth, height: zoneHeight }"
  ></div>

  <button
    type="button"
    v-if="showButton"
    class="fab-button floating"
    :class="[
      `fab-button--${position}`,
      `fab-button--${size}`,
      { 'dark-mode': darkMode },
    ]"
    :style="accountSidebar"
    :disabled="disabled"
    @click="handleClick"
    @touchstart="resetButtonTimer"
    @pointerenter="setZoneActive(true, $event)"
    @pointerleave="setZoneActive(false, $event)"
    :aria-label="label"
    :title="label"
  >
    <i :class="iconOutlined ? 'material-symbols-outlined' : 'material-symbols'">{{ icon }}</i>
    <span v-if="badge" class="fab-badge">{{ badge }}</span>
  </button>
</template>

<script lang="ts">
import { reactive } from "vue";
import { state, getters } from "@/store";

// Buttons that show and dissapear together, on plyr the buttons at the right and left for example
const buttonGroup = new Map<string, { visible: boolean; zoneActive: boolean; timer: ReturnType<typeof setTimeout> | null }>();

function getGroupState(group: string) {
  let entry = buttonGroup.get(group);
  if (!entry) {
    entry = reactive({ visible: false, zoneActive: false, timer: null });
    buttonGroup.set(group, entry);
  }
  return entry;
}

export default {
  name: "floatingButton",
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
    // this is optional, plyr uses it for the queue button which has the number of tracks at the right top of the button
    badge: {
      type: [String, Number],
      default: null,
    },
    position: {
      type: String,
      default: "top-right",
      validator: (v: string) => ["top-right", "top-left", "bottom-right", "bottom-left"].includes(v),
    },
    size: {
      type: String,
      default: "normal",
      validator: (v: string) => ["normal", "small"].includes(v),
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    // Sets both the title and aria-label
    label: {
      type: String,
      default: "",
    },
    // Inline style overrides for positioning
    offset: {
      type: Object,
      default: () => ({}),
    },
    // To hide the button after 3s. If set to false will keep the button always visible
    // (in which case no detection zone at all)
    autoHide: {
      type: Boolean,
      default: true,
    },
    zoneWidth: {
      type: String,
      default: "5em",
    },
    zoneHeight: {
      type: String,
      default: "5em",
    },
    // Buttons that show and dissapear together, on plyr the buttons at the right and left for example
    group: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      buttonVisible: false,
      buttonZone: false,
      buttonTimer: null as ReturnType<typeof setTimeout> | null,
      pointerInsideZone: false,
    };
  },
  computed: {
    darkMode(): boolean {
      return getters.isDarkMode();
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
    // Account for the sidebar width for button positioned at the left
    // skipped if some caller specifies their own left in the offset props
    accountSidebar(): Record<string, string> {
      const isLeftPositioned = this.position === "top-left" || this.position === "bottom-left";
      const pushedBySidebar = isLeftPositioned && getters.isSidebarVisible() && getters.isStickySidebar();
      if (!pushedBySidebar || (this.offset as Record<string, string>).left !== undefined) return this.offset;
      return { ...this.offset, left: `calc(20px + ${state.sidebar.width}em)` };
    },
  },
  mounted() {
    if (this.autoHide) {
      this.resetButtonTimer(); // show buttons initially
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
    // click/touch pass through to whatever is behind, so detection happens here against the zone bounds
    isInsideZone(x: number, y: number): boolean {
      const el = this.$refs.zoneEl as HTMLElement | undefined;
      if (!el) return false;
      const rect = el.getBoundingClientRect();
      return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
    },
    handleGlobalPointerMove(event: PointerEvent) {
      if (event.pointerType !== "mouse") return;
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
        }, 3000);
        return;
      }
      this.buttonVisible = true;
      if (this.buttonTimer) clearTimeout(this.buttonTimer);
      this.buttonTimer = setTimeout(() => {
        if (!this.buttonZone) this.buttonVisible = false;
        this.buttonTimer = null;
      }, 3000);
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
  position: fixed;
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

.fab-zone--bottom-right {
  bottom: 0;
  right: 0;
}

.fab-zone--bottom-left {
  bottom: 0;
  left: 0;
}

.fab-button {
  position: fixed;
  width: 50px;
  height: 50px;
  border: none;
  border-radius: 50%;
  background: var(--background);
  color: var(--textPrimary);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  outline: none;
  z-index: 9998;
  animation: fab-button-show 0.4s ease-out;
}

.fab-button.dark-mode {
  background: var(--surfacePrimary);
}

.fab-button:hover {
  background: var(--primaryColor);
  transform: translateY(-2px) scale(1.05);
  box-shadow: 0 8px 25px rgba(var(--primaryColor-rgb), 0.3), 0 4px 12px rgba(0, 0, 0, 0.2);
  color: white;
}

.fab-button i.material-symbols,
.fab-button i.material-symbols-outlined {
  font-size: 24px;
  transition: transform 0.2s ease, font-variation-settings 0.25s ease;
}

.fab-button:hover i.material-symbols,
.fab-button:hover i.material-symbols-outlined {
  transform: scale(1.1);
}

.fab-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* sizes */
.fab-button--small {
  width: 36px;
  height: 36px;
}

.fab-button--small i.material-symbols,
.fab-button--small i.material-symbols-outlined {
  font-size: 24px;
}

/* positions */
.fab-button--top-right {
  top: 80px;
  right: 20px;
}

.fab-button--top-left {
  top: 80px;
  left: 20px;
}

.fab-button--bottom-right {
  bottom: calc(env(safe-area-inset-bottom, 0px) + 20px);
  right: calc(env(safe-area-inset-right, 0px) + 20px);
}

.fab-button--bottom-left {
  bottom: calc(env(safe-area-inset-bottom, 0px) + 20px);
  left: calc(env(safe-area-inset-left, 0px) + 20px);
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
</style>
