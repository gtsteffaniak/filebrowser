<template>
  <div ref="root" class="expand-dropdown">
    <div
      ref="anchor"
      class="expand-dropdown-anchor menu-panel no-select floating-window"
      :class="{
        'expand-dropdown-anchor--hidden': open,
        'dark-mode': isDarkMode,
      }"
    >
      <button
        ref="trigger"
        type="button"
        class="action expand-dropdown-trigger"
        :id="resolvedInputId"
        :aria-expanded="open ? 'true' : 'false'"
        aria-haspopup="listbox"
        :aria-label="resolvedAriaLabel"
        @click="openPanel"
        @keydown.down.prevent="openPanel"
        @keydown.enter.prevent="openPanel"
        @keydown.space.prevent="openPanel"
      >
        <span class="expand-dropdown-trigger-label">{{ displayLabel }}</span>
        <i class="material-symbols expand-dropdown-chevron">expand_more</i>
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="open"
        ref="overlay"
        class="expand-dropdown-overlay menu-panel no-select floating-window"
        :class="{ 'dark-mode': isDarkMode }"
        :style="overlayStyle"
      >
        <button
          type="button"
          class="action expand-dropdown-trigger"
          :aria-expanded="'true'"
          aria-haspopup="listbox"
          :aria-label="resolvedAriaLabel"
          @click="close"
          @keydown.esc.prevent="close"
        >
          <span class="expand-dropdown-trigger-label">{{ displayLabel }}</span>
          <i class="material-symbols expand-dropdown-chevron expand-dropdown-chevron--open">expand_more</i>
        </button>

        <transition
          name="expand"
          @before-enter="beforeEnter"
          @enter="enter"
          @leave="leave"
        >
          <div
            v-if="open"
            ref="panel"
            class="expand-dropdown-body"
            role="listbox"
            :aria-multiselectable="allowMultiple ? 'true' : 'false'"
            :aria-label="resolvedAriaLabel"
          >
            <hr class="divider">
            <MenuOptionList
              ref="menuOptions"
              :options="normalizedOptions"
              :selected-keys="selectedKeys"
              :allow-search="allowSearch"
              :search-query="searchQuery"
              :search-placeholder="searchPlaceholder"
              :empty-label="emptyLabel"
              @select="selectOption"
              @update:search-query="searchQuery = $event"
              @close="close"
            />
          </div>
        </transition>
      </div>
    </Teleport>
  </div>
</template>

<script>
import MenuOptionList from "@/components/MenuOptionList.vue";
import { getters } from "@/store";
import {
  expandBeforeEnter,
  expandEnter,
  expandLeave,
} from "@/utils/expandTransition.js";

let expandDropdownIdCounter = 0;

export default {
  name: "ExpandDropdown",

  components: {
    MenuOptionList,
  },

  props: {
    modelValue: {
      type: [String, Number, Array],
      default: "",
    },
    options: {
      type: Array,
      default: () => [],
    },
    allowMultiple: {
      type: Boolean,
      default: false,
    },
    allowSearch: {
      type: Boolean,
      default: false,
    },
    defaultPlaceholderIfEmpty: {
      type: String,
      default: "",
    },
    defaultValue: {
      type: [String, Number, Array],
      default: undefined,
    },
    ariaLabel: {
      type: String,
      default: "",
    },
    searchPlaceholder: {
      type: String,
      default: "",
    },
    emptyLabel: {
      type: String,
      default: "",
    },
    inputId: {
      type: String,
      default: "",
    },
    allSelectedLabel: {
      type: String,
      default: "",
    },
    allValue: {
      type: [String, Number],
      default: undefined,
    },
    emptyMeansAll: {
      type: Boolean,
      default: false,
    },
  },

  emits: ["update:modelValue"],

  data() {
    return {
      open: false,
      searchQuery: "",
      overlayStyle: {},
      localInputId: `expand-dropdown-${expandDropdownIdCounter += 1}`,
    };
  },

  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    resolvedInputId() {
      return this.inputId || this.localInputId;
    },
    resolvedAriaLabel() {
      return this.ariaLabel || this.displayLabel;
    },
    normalizedOptions() {
      return (this.options || []).map((option) => ({
        value: option.value,
        label: option.label,
        disabled: !!option.disabled,
      }));
    },
    normalizedModelArray() {
      if (!this.allowMultiple) {
        return [];
      }
      if (Array.isArray(this.modelValue)) {
        return this.modelValue.map((value) => this.optionKey(value));
      }
      if (this.modelValue === undefined || this.modelValue === null || this.modelValue === "") {
        return [];
      }
      return [this.optionKey(this.modelValue)];
    },
    selectedKeys() {
      if (this.allowMultiple) {
        if (
          this.emptyMeansAll
          && this.normalizedModelArray.length === 0
          && this.normalizedOptions.length > 0
        ) {
          return this.normalizedOptions.map((option) => this.optionKey(option.value));
        }
        return this.normalizedModelArray;
      }
      return [this.optionKey(this.effectiveSingleValue)];
    },
    effectiveSingleValue() {
      if (this.allowMultiple) {
        return "";
      }
      if (this.modelValue !== undefined && this.modelValue !== null) {
        return this.modelValue;
      }
      if (this.defaultValue !== undefined && this.defaultValue !== null) {
        return this.defaultValue;
      }
      return "";
    },
    resolvedAllSelectedLabel() {
      return this.allSelectedLabel || this.$t("general.all");
    },
    isAllSelected() {
      if (this.allowMultiple) {
        if (this.normalizedOptions.length === 0) {
          return false;
        }
        const selected = this.normalizedModelArray;
        if (this.emptyMeansAll && selected.length === 0) {
          return true;
        }
        return selected.length === this.normalizedOptions.length;
      }
      if (this.allValue === undefined) {
        return false;
      }
      return this.optionKey(this.effectiveSingleValue) === this.optionKey(this.allValue);
    },
    displayLabel() {
      if (this.isAllSelected) {
        return this.resolvedAllSelectedLabel;
      }
      if (this.allowMultiple) {
        const selected = this.normalizedModelArray;
        if (selected.length === 0) {
          return this.defaultPlaceholderIfEmpty || this.labelForValue(this.defaultValue) || "";
        }
        const labels = selected
          .map((key) => this.labelForKey(key))
          .filter(Boolean);
        if (labels.length === 0) {
          return this.defaultPlaceholderIfEmpty || "";
        }
        if (labels.length <= 2) {
          return labels.join(", ");
        }
        return `${labels.length} ${this.$t("general.selected")}`;
      }
      return this.labelForValue(this.effectiveSingleValue)
        || this.defaultPlaceholderIfEmpty
        || "";
    },
  },

  watch: {
    open(isOpen) {
      if (isOpen) {
        this.$nextTick(() => {
          this.updateOverlayPosition();
          if (this.allowSearch) {
            this.$refs.menuOptions?.focusSearch();
          }
        });
      } else {
        this.searchQuery = "";
        this.overlayStyle = {};
      }
    },
    options: {
      deep: true,
      handler() {
        if (this.open) {
          this.$nextTick(() => this.updateOverlayPosition());
        }
      },
    },
  },

  mounted() {
    document.addEventListener("mousedown", this.onDocumentMouseDown);
    window.addEventListener("resize", this.onViewportChange);
    window.addEventListener("scroll", this.onViewportChange, true);
  },

  beforeUnmount() {
    document.removeEventListener("mousedown", this.onDocumentMouseDown);
    window.removeEventListener("resize", this.onViewportChange);
    window.removeEventListener("scroll", this.onViewportChange, true);
  },

  methods: {
    optionKey(value) {
      return String(value);
    },
    labelForValue(value) {
      const match = this.normalizedOptions.find(
        (option) => this.optionKey(option.value) === this.optionKey(value),
      );
      return match ? match.label : "";
    },
    labelForKey(key) {
      const match = this.normalizedOptions.find(
        (option) => this.optionKey(option.value) === key,
      );
      return match ? match.label : key;
    },
    openPanel() {
      if (this.open) {
        return;
      }
      this.updateOverlayPosition();
      this.open = true;
    },
    close() {
      this.open = false;
    },
    updateOverlayPosition() {
      const anchor = this.$refs.anchor;
      if (!anchor) {
        return;
      }
      const rect = anchor.getBoundingClientRect();
      this.overlayStyle = {
        position: "fixed",
        top: `${rect.top}px`,
        left: `${rect.left}px`,
        width: `${rect.width}px`,
        zIndex: 1000,
      };
    },
    onViewportChange() {
      if (this.open) {
        this.updateOverlayPosition();
      }
    },
    onDocumentMouseDown(event) {
      if (!this.open) {
        return;
      }
      const overlay = this.$refs.overlay;
      const anchor = this.$refs.anchor;
      if (overlay?.contains(event.target) || anchor?.contains(event.target)) {
        return;
      }
      this.close();
    },
    selectOption(option) {
      if (option.disabled) {
        return;
      }
      if (this.allowMultiple) {
        const key = this.optionKey(option.value);
        const next = [...this.normalizedModelArray];
        const index = next.indexOf(key);
        if (index >= 0) {
          next.splice(index, 1);
        } else {
          next.push(key);
        }
        const values = next.map((entryKey) => {
          const match = this.normalizedOptions.find(
            (item) => this.optionKey(item.value) === entryKey,
          );
          return match ? match.value : entryKey;
        });
        this.$emit("update:modelValue", values);
        return;
      }
      this.$emit("update:modelValue", option.value);
      this.close();
    },
    beforeEnter: expandBeforeEnter,
    enter: expandEnter,
    leave: expandLeave,
  },
};
</script>

<style scoped>
.expand-dropdown {
  position: relative;
  width: 100%;
  min-width: 0;
}

.menu-panel {
  background-color: var(--background);
  border-radius: 1em;
  padding: 0.5em;
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  min-width: 13em;
}

.expand-dropdown-anchor--hidden {
  visibility: hidden;
}

.expand-dropdown-overlay {
  box-sizing: border-box;
}

.expand-dropdown-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  min-height: 2.25em;
  padding-left: 0.5em;
  border-radius: 0.65em;
  text-align: left;
  cursor: pointer;
}

.expand-dropdown-trigger-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--textPrimary);
}

.expand-dropdown-chevron {
  flex-shrink: 0;
  font-size: 1.25rem;
  color: var(--textSecondary);
  transition: transform 0.2s ease;
}

.expand-dropdown-chevron--open {
  transform: rotate(180deg);
}

.expand-dropdown-body {
  overflow: hidden;
  width: 100%;
}

.expand-dropdown-body .divider {
  margin: 0.35em 0;
}

.expand-enter-active,
.expand-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}
</style>
