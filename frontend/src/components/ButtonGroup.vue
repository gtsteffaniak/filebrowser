<template>
  <div class="button-group">
    <button
      v-for="(btn, index) in buttons"
      :key="index"
      :class="{ active: activeButton === index }"
      @click="setActiveButton(index)"
    >
      {{ btn.label }}
    </button>
  </div>
</template>

<script>
export default {
  props: {
    buttons: {
      type: Array,
      default: () => [],
    },
    initialActive: {
      type: Number,
      default: null,
    },
  },
  data() {
    return {
      activeButton: this.initialActive,
    };
  },
  methods: {
    setActiveButton(index) {
      // If the clicked button is already active, de-select it
      if (this.activeButton === index) {
        this.activeButton = null;
        this.$emit("remove-button-clicked", this.buttons[index].value);
      } else {
        // Emit remove-button-clicked for all other indexes
        this.buttons.forEach((button, idx) => {
          if (idx !== index) {
            this.$emit("remove-button-clicked", button.value);
          }
        });

        this.activeButton = index;
        this.$emit("button-clicked", this.buttons[index].value);
      }
    },
  },
  watch: {
    initialActive: {
      immediate: true,
      handler(newVal) {
        this.activeButton = newVal;
      },
    },
  },
};
</script>

<style scoped>
.button-group {
  display: flex;
  border: 1px solid #ccc;
  border-radius: 4px;
  overflow: hidden;
}

button {
  flex: 1;
  height: 3em;
  padding: 8px 16px;
  border: none;
  background-color: #f5f5f5;
  transition: background-color 0.3s;

  /* Add borders */
  border-right: 1px solid #ccc;
}

/* Remove the border from the last button */
.button-group > button:last-child {
  border-right: none;
}

button:hover {
  background-color: #e0e0e0;
}

button.active {
  background-color: var(--blue);
  color: #ffffff;
}
</style>
