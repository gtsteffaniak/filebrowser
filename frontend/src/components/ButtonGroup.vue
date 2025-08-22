<template>
  <div @click="preventDefaults" class="button-group">
    <button v-if="isDisabled" >
      {{ disableMessage }}
    </button>
    <template v-else>
      <button
        v-for="(btn, index) in buttons"
        :key="index"
        class="clickable"
        :class="{ active: activeButton === index }"
        @click="setActiveButton(index, btn.value)"
      >
        {{ btn.label }}
      </button>
    </template>
  </div>
</template>

<script>
export default {
  props: {
    disableMessage: {
      type: String,
      default: "No options for folders",
    },
    buttons: {
      type: Array,
      default: () => [],
    },
    isDisabled: {
      type: Boolean,
      default: false,
    },
    initialActive: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      activeButton: null, // Initially no button is active
    };
  },
  methods: {
    preventDefaults(e) {
      e.preventDefault();
      e.stopPropagation();
    },
    setActiveButton(index, value) {
      if (value === "Only Folders" && this.activeButton !== index) {
        this.$emit("disableAll");
      }
      if (value === "Only Folders" && this.activeButton === index) {
        this.$emit("enableAll");
      }
      if (value === "Only Files" && this.activeButton !== index) {
        this.$emit("enableAll");
      }
      // If the clicked button is already active, de-select it
      if (this.activeButton === index) {
        this.activeButton = null;
        this.$emit("remove-button-clicked", value);
      } else {
        // Emit remove-button-clicked for all other indexes
        this.buttons.forEach((button, idx) => {
          if (idx !== index) {
            this.$emit("remove-button-clicked", button.value);
          }
        });

        this.activeButton = index;
        this.$emit("button-clicked", value);
      }
    },
  },
  watch: {
    initialActive: {
      immediate: true,
      handler(newVal) {
        // Find the button whose value matches initialActive
        const initialIndex = this.buttons.findIndex((btn) => btn.value === newVal);
        this.activeButton = initialIndex !== -1 ? initialIndex : null; // Set to matching button index or null
      },
    },
  },
};
</script>

<style scoped>
.button-group {
  margin: 1em;
  display: flex;
  flex-wrap: wrap;
  border: 1px solid #ccc;
  border-top:none;
  border-radius: 1em;
  overflow: hidden;
}

button {
  cursor: pointer;
  flex: 1;
  height: 3em;
  padding: 8px 16px;
  border: none;
  background: #f5f5f5;
  transition: background-color 0.3s;
  /* Add borders */
  border-right: 1px solid #ccc;
  border-top: 1px solid #ccc;
}

.button-group > button:last-child {
  border-right: none;
}

button:disabled {
  cursor: not-allowed !important;
}

button.active {
  background-color: var(--primaryColor) !important;
  color: #ffffff;
}
</style>
