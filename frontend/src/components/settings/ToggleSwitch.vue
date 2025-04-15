<template>
  <div class="toggle-container">
    <span class="toggle-name">{{ name }}</span>
    <label class="switch">
      <input type="checkbox" :checked="modelValue" @change="updateValue" />
      <span class="slider round"></span>
    </label>
  </div>
</template>

<script>
export default {
  name: "ToggleSwitch",
  props: {
    modelValue: {
      type: Boolean,
      required: true,
    },
    name: {
      type: String,
      required: true,
    },
  },
  methods: {
    updateValue(event) {
      this.$emit("update:modelValue", event.target.checked);
    },
  },
};
</script>

<style scoped>
.toggle-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 1rem;
}

.toggle-name {
  margin-right: 10px;
}

.switch {
  position: relative;
  display: inline-block;
  width: 60px;
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
  background-color: #ccc;
  transition: 0.4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 26px;
  width: 26px;
  left: 4px;
  bottom: 4px;
  background-color: white;
  transition: 0.4s;
}

input:checked + .slider {
  background-color: var(--primaryColor);
}

input:focus + .slider {
  box-shadow: 0 0 1px var(--primaryColor);
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
</style>
