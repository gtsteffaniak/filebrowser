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
      <path
        class="line top"
        d="m 30,33 h 40 c 0,0 8.5,-0.68551 8.5,10.375 0,8.292653 -6.122707,9.002293 -8.5,6.625 l -11.071429,-11.071429"
        v-show="multistate !== 'close'"
      />
      <path
        class="line top close-line"
        d="M 30,30 L 70,70"
        v-show="multistate === 'close'"
      />

      <path class="line middle" d="m 70,50 h -40" />

      <path
        class="line bottom"
        d="m 30,67 h 40 c 0,0 8.5,0.68551 8.5,-10.375 0,-8.292653 -6.122707,-9.002293 -8.5,-6.625 l -11.071429,11.071429"
        v-show="multistate !== 'close'"
      />
      <path
        class="line bottom close-line"
        d="M 30,70 L 70,30"
        v-show="multistate === 'close'"
      />
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
    <i class="material-icons">{{ icon }}</i>
    <span>{{ label }}</span>
    <span v-if="counter > 0" class="counter">{{ counter }}</span>
  </button>
</template>

<script>
import { mutations, state } from "@/store";

export default {
  name: "action",
  props: ["icon", "label", "counter", "show", "isDisabled"],
  computed: {
    multistate() {
      return state.multiButtonState;
    },
    stickSidebar() {
      return state.stickSidebar;
    },
  },
  methods: {
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
  margin-left: 0.25em;
  cursor: pointer;
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
  transition: all 400ms ease-in-out, transform 400ms ease-in-out,
    opacity 400ms ease-in-out;
}

.ham.menu .line {
  stroke-dasharray: none;
}

.ham.close .line {
  stroke-dasharray: none;
}

.ham.back .line {
  stroke-dasharray: none;
}

.ham5 .top {
  stroke-dasharray: 40 82;
}

.ham5 .bottom {
  stroke-dasharray: 40 82;
}

.ham5.back .top {
  stroke-dasharray: 14 82;
  stroke-dashoffset: -72px;
}

.ham5.back .bottom {
  stroke-dasharray: 14 82;
  stroke-dashoffset: -72px;
}
.action {
  width: 3em;
}

.line.bottom-arrow {
  fill: var(--textPrimary); /* Fill color for arrow */
  transition: fill 400ms; /* Transition for fill color */
}

.ham5.back .bottom-arrow {
  fill: transparent; /* Change fill color on active state */
}

.ham5.close .middle {
  opacity: 0;
}

.ham5.close .middle {
  opacity: 0;
}

.close-line {
  stroke: var(--textPrimary);
  stroke-width: 5.5;
  stroke-linecap: round;
  transition: stroke 400ms ease-in-out;
}
</style>
