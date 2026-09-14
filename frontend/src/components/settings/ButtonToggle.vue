<template>
  <div
    class="button-toggle-container"
    :class="{ 'button-toggle-container--disabled': disabled }"
  >
    <div class="button-toggle">
      <span
        class="button-toggle__track"
        :class="{ 'button-toggle__track--on': modelValue }"
        :style="trackStyle"
        aria-hidden="true"
      >
        <span class="button-toggle__slots">
          <span class="button-toggle__label">{{ displayLabel }}</span>
          <button
            type="button"
            class="button-toggle__thumb"
            :class="{ 'button-toggle__thumb--on': modelValue }"
            role="switch"
            :disabled="disabled"
            :aria-checked="modelValue"
            :aria-label="ariaLabel || `${offLabel} / ${onLabel}`"
            @click="onClick"
          />
        </span>
      </span>
    </div>
  </div>
</template>

<script>
export default {
  name: "ButtonToggle",
  props: {
    modelValue: {
      type: Boolean,
      required: true,
    },
    onLabel: {
      type: String,
      required: true,
    },
    offLabel: {
      type: String,
      required: true,
    },
    ariaLabel: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  computed: {
    displayLabel() {
      return this.modelValue ? this.onLabel : this.offLabel;
    },
    /** Minimum thumb width (em) to fit the longer label. */
    thumbContentWidthEm() {
      const chars = Math.max(this.onLabel.length, this.offLabel.length);
      return Math.max(chars * 0.48, 3.25);
    },
    trackPreferredWidthEm() {
      const labelMinTrackEm = this.thumbContentWidthEm / 0.85;
      return Math.max(labelMinTrackEm * 1.65, 10.5);
    },
    trackStyle() {
      return {
        width: `${this.trackPreferredWidthEm}em`,
        maxWidth: "100%",
      };
    },
  },
  methods: {
    onClick() {
      if (this.disabled) {
        return;
      }
      this.$emit("update:modelValue", !this.modelValue);
    },
  },
};
</script>

<style scoped>
.button-toggle-container {
  display: flex;
  align-items: center;
  justify-content: center;
  align-self: center;
  flex: 1 1 auto;
  max-width: 100%;
  min-width: 0;
  font-size: 1rem;
  box-sizing: border-box;
}

.button-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 100%;
  min-width: 0;
}

.button-toggle__track {
  --toggle-track-extra: 0.65em;
  --toggle-pad-active: 0.15em;
  --toggle-pad-base: 0.1em;
  --thumb-width-ratio: 0.85;
  --thumb-width: calc(100% * var(--thumb-width-ratio));
  --thumb-travel: calc(100% - var(--thumb-width));
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  min-width: 0;
  border-radius: 50px;
  background-color: var(--background);
  border: var(--borderWidth, 1px) solid var(--surfaceSecondary);
  padding: calc(var(--toggle-track-extra) / 2);
  pointer-events: none;
}

.button-toggle__track:not(.button-toggle__track--on) {
  padding-left: calc(var(--toggle-track-extra) / 2 + var(--toggle-pad-active));
  padding-right: calc(var(--toggle-track-extra) / 2 + var(--toggle-pad-base));
}

.button-toggle__track--on {
  padding-left: calc(var(--toggle-track-extra) / 2 + var(--toggle-pad-base));
  padding-right: calc(var(--toggle-track-extra) / 2 + var(--toggle-pad-active));
}

.button-toggle__slots {
  position: relative;
  display: block;
  width: 100%;
  height: 1.85em;
  box-sizing: border-box;
}

.button-toggle__label {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 2;
  max-width: 55%;
  transform: translate(-50%, -50%);
  font-size: 0.9rem;
  font-weight: 500;
  line-height: 1.1;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--textPrimary);
  user-select: none;
  pointer-events: none;
}

.button-toggle__thumb {
  --thumb-y-inset: 0.12em;
  position: absolute;
  top: 50%;
  left: 0;
  width: var(--thumb-width);
  height: calc(100% - (2 * var(--thumb-y-inset)));
  margin: 0;
  padding: 0;
  border: none;
  border-radius: 50px;
  background-color: var(--surfacePrimary);
  z-index: 1;
  box-sizing: border-box;
  cursor: pointer;
  pointer-events: auto;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transform: translateY(-50%);
  transform-origin: center center;
  transition:
    left 0.4s,
    transform 0.15s ease,
    box-shadow 0.15s ease;
  font: inherit;
  line-height: 1;
}

.button-toggle__thumb--on {
  left: var(--thumb-travel);
}

.button-toggle__thumb:hover:not(:disabled) {
  transform: translateY(-50%) scale(1.05);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.14);
}

.button-toggle__thumb:active:not(:disabled) {
  transform: translateY(-50%) scale(0.98);
}

.button-toggle__thumb:focus-visible {
  outline: 2px solid var(--textSecondary);
  outline-offset: 2px;
}

.button-toggle-container--disabled,
.button-toggle-container--disabled .button-toggle__thumb,
.button-toggle__thumb:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

@media (prefers-reduced-motion: reduce) {
  .button-toggle__thumb {
    transition: left 0.01ms, transform 0.01ms, box-shadow 0.01ms;
  }

  .button-toggle__thumb:hover:not(:disabled),
  .button-toggle__thumb:active:not(:disabled) {
    transform: translateY(-50%);
  }
}
</style>
