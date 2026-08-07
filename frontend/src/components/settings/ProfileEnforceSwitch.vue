<template>
  <div
    v-if="visible"
    class="profile-enforce-row toggle-row toggle-row--enforced border-radius"
    :class="{ disabled: disabled }"
  >
    <label class="enforced-label" :for="inputId">{{ enforcedLabelText }}</label>
    <label class="switch">
      <input
        :id="inputId"
        type="checkbox"
        :checked="enforced"
        :disabled="disabled"
        :aria-label="enforcedLabelText"
        @change="onChange"
      />
      <span class="slider round"></span>
    </label>
  </div>
</template>

<script>
let idCounter = 0;

export default {
  name: "ProfileEnforceSwitch",
  props: {
    enforced: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    visible: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["update:enforced"],
  data() {
    idCounter += 1;
    return {
      inputId: `profile-enforce-${idCounter}`,
    };
  },
  computed: {
    enforcedLabelText() {
      return this.$t("general.enforce");
    },
  },
  methods: {
    onChange(event) {
      this.$emit("update:enforced", event.target.checked);
    },
  },
};
</script>

<style scoped>
.profile-enforce-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  box-sizing: border-box;
  min-height: 3.25em;
  padding: 0.5em 1em;
  margin-top: 0.35em;
  transition: background-color 0.15s ease;
}

.profile-enforce-row:hover {
  background-color: var(--surfaceSecondary);
}

.profile-enforce-row.disabled {
  opacity: 0.5;
}

.enforced-label {
  flex: 1;
  min-width: 0;
  padding-right: 0.75em;
  cursor: pointer;
  user-select: none;
  font-size: 1rem;
}

.switch {
  position: relative;
  display: inline-block;
  padding-right: 4em;
  height: 34px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  transition: 0.4s;
  background-color: gray;
}

.slider:before {
  position: absolute;
  content: "";
  height: 26px;
  width: 26px;
  left: 6px;
  bottom: 4px;
  background-color: white;
  transition: 0.4s;
}

input:checked + .slider {
  background-color: var(--primaryColor);
}

input:checked + .slider:before {
  transform: translateX(26px);
}

.slider.round {
  border-radius: 50px;
}

.slider.round:before {
  border-radius: 50%;
}

input:disabled + .slider {
  cursor: not-allowed;
  background-color: #ccc;
}

input:disabled:checked + .slider {
  background-color: #999;
}
</style>
