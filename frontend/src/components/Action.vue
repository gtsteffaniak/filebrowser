<template>
  <button
    v-if="icon === 'close_back'"
    :disabled="isDisabled"
    @click="actionMultiIcon"
    :aria-label="label"
    :title="label"
    class="action no-select"
  >
    <svg
      id="button-toggle-navbar"
      class="ham hamRotate180 ham5"
      viewBox="0 0 100 100"
      width="30"
      :class="{
        back: multistate == 'back',
        close: multistate == 'close',
      }"
    >
      <path class="line top" d="M 30,33 H 70" />
      <path class="line middle" d="M 30,50 H 70" />
      <path class="line bottom" d="M 30,67 H 70" />
    </svg>
  </button>
  <button
    v-else
    :disabled="isDisabled"
    @click="action"
    :aria-label="label"
    :title="label"
    class="action no-select"
  >
    <i v-if="icon == 'table_rows_narrow'" class="material-symbols-outlined">{{ icon }}</i>
    <i v-else class="material-icons">{{ icon }}</i>

    <span>{{ label }}</span>
    <span v-if="counter > 0" class="counter">{{ counter }}</span>
  </button>
</template>

<script>
import { mutations, state, getters } from "@/store";

export default {
  name: "action",
  props: ["icon", "label", "counter", "show", "isDisabled"],
  computed: {
    req() {
      return state.req;
    },
    multistate() {
      return getters.multibuttonState();
    },
    stickSidebar() {
      return state.user?.stickySidebar;
    },
    currentView() {
      return getters.currentView();
    }
  },
  mounted() {
    this.reEvalAction();
  },
  watch: {
    $route() {
      this.reEvalAction();
    },
    req() {
      this.reEvalAction();
    },
    currentView() {
      this.reEvalAction();
    },
  },
  methods: {
    reEvalAction() {
      const currentView = getters.currentView();
      if (currentView == "settings") {
        mutations.setActiveSettingsView(getters.currentHash());
      }
    },
    actionMultiIcon() {
      if (this.show) {
        mutations.showHover(this.show);
      }
      this.$emit("action");
    },
    action() {
      if (this.show) {
        mutations.showHover(this.show);
      }
      this.$emit("action");
    },
  },
};
</script>

<style>
.ham {
  width: 2.5em;
  margin-top: 0.25em;
  -webkit-tap-highlight-color: transparent;
  transition: transform 400ms;
  -moz-user-select: none;
  -webkit-user-select: none;
  -ms-user-select: none;
  user-select: none;
}

.hamRotate180.back {
  transform: rotate(180deg);
}

.line {
  fill: none;
  stroke: var(--textPrimary);
  stroke-width: 5.5;
  stroke-linecap: round;
  transition: stroke 400ms ease, transform 400ms ease, opacity 400ms ease,
    stroke-dasharray 400ms ease, stroke-dashoffset 400ms ease;
  transform-origin: 50% 50%;
}

/* Menu (default) */
.ham5 .top,
.ham5 .middle,
.ham5 .bottom {
  transform: none;
  opacity: 1;
  stroke-dasharray: 40 82;
  stroke-dashoffset: 0;
}

/* Close (X) */
.ham5.close .top {
  transform: translateY(1em) translateX(-0.75em) rotate(45deg);
}

.ham5.close .middle {
  opacity: 0;
}

.ham5.close .bottom {
  transform: translateY(-0.5em) translateX(-0.75em) rotate(-45deg);
}

/* Back (Arrow) */
.ham5.back .top {
  transform: translate(0, 0.2em) rotate(45deg) scaleX(0.5);
}

.ham5.back .bottom {
  transform: translate(0, -0.2em) rotate(-45deg) scaleX(0.5);
}
</style>
