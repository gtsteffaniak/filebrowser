<template>
  <div ref="root" class="expand-dropdown" :class="{ 'expand-dropdown--open': open }">
    <div
      ref="anchor"
      class="expand-dropdown-anchor menu-panel no-select floating-window"
      :class="{ 'dark-mode': isDarkMode }"
    >
      <button
        ref="trigger"
        type="button"
        class="action expand-dropdown-trigger"
        :id="resolvedInputId"
        :aria-expanded="open ? 'true' : 'false'"
        aria-haspopup="listbox"
        :aria-label="resolvedAriaLabel"
        @click="togglePanel"
        @keydown.down.prevent="openPanel"
        @keydown.enter.prevent="togglePanel"
        @keydown.space.prevent="togglePanel"
        @keydown.esc.prevent="close"
      >
        <span class="expand-dropdown-trigger-label">{{ displayLabel }}</span>
        <i
          class="material-symbols expand-dropdown-chevron"
          :class="{ 'expand-dropdown-chevron--open': isExpanded }"
        >expand_more</i>
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="open"
        class="expand-dropdown-shadow"
        :class="{ 'expand-dropdown-shadow--closing': closing }"
        :style="shadowStyle"
        aria-hidden="true"
      />
      <div
        v-if="open"
        ref="overlay"
        class="expand-dropdown-overlay"
        :class="{ 'dark-mode': isDarkMode }"
        :style="overlayStyle"
      >
        <transition
          name="expand"
          @before-enter="beforeEnter"
          @enter="enter"
          @leave="leave"
          @after-leave="onPanelAfterLeave"
        >
          <div
            v-if="panelOpen"
            ref="panel"
            class="expand-dropdown-body menu-panel no-select"
            role="listbox"
            :aria-multiselectable="allowMultiple ? 'true' : 'false'"
            :aria-label="resolvedAriaLabel"
          >
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
const EXPAND_OPEN_MS = 300;
const EXPAND_CLOSE_MS = 150;

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
      panelOpen: false,
      isExpanded: false,
      closing: false,
      searchQuery: "",
      overlayStyle: {},
      shadowStyle: {},
      localInputId: `expand-dropdown-${expandDropdownIdCounter += 1}`,
      panelResizeObserver: null,
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
        return this.$t("general.selected", { prefix: `${labels.length} ` });
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
        });
      }
    },
    panelOpen(isOpen) {
      if (isOpen) {
        this.$nextTick(() => {
          this.observePanelResize();
          this.updateOverlayPosition();
          if (this.allowSearch) {
            this.$refs.menuOptions?.focusSearch();
          }
        });
      } else {
        this.unobservePanelResize();
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
    this.panelResizeObserver = new ResizeObserver(() => {
      if (this.open) {
        this.updateOverlayPosition();
      }
    });
  },

  beforeUnmount() {
    document.removeEventListener("mousedown", this.onDocumentMouseDown);
    window.removeEventListener("resize", this.onViewportChange);
    window.removeEventListener("scroll", this.onViewportChange, true);
    this.unobservePanelResize();
    this.panelResizeObserver = null;
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
      this.closing = false;
      this.panelOpen = false;
      this.isExpanded = true;
      this.updateOverlayPosition();
      this.open = true;
      this.$nextTick(() => {
        this.panelOpen = true;
      });
    },
    togglePanel() {
      if (this.open) {
        this.close();
        return;
      }
      this.openPanel();
    },
    close() {
      if (!this.open) {
        return;
      }
      this.isExpanded = false;
      if (!this.panelOpen) {
        this.finishClose();
        return;
      }
      this.closing = true;
      this.panelOpen = false;
    },
    finishClose() {
      this.open = false;
      this.panelOpen = false;
      this.isExpanded = false;
      this.closing = false;
      this.searchQuery = "";
      this.overlayStyle = {};
      this.shadowStyle = {};
      this.unobservePanelResize();
    },
    onPanelAfterLeave() {
      if (!this.panelOpen) {
        this.finishClose();
      }
    },
    enter(el, done) {
      expandEnter(el, () => {
        this.updateOverlayPosition();
        done();
      }, EXPAND_OPEN_MS);
    },
    leave(el, done) {
      expandLeave(el, done, EXPAND_CLOSE_MS);
    },
    observePanelResize() {
      const panel = this.$refs.panel;
      if (panel && this.panelResizeObserver) {
        this.panelResizeObserver.observe(panel);
      }
    },
    unobservePanelResize() {
      this.panelResizeObserver?.disconnect();
    },
    updateOverlayPosition() {
      const anchor = this.$refs.anchor;
      if (!anchor) {
        return;
      }
      const anchorRect = anchor.getBoundingClientRect();
      const panel = this.$refs.panel;
      const panelHeight = panel?.getBoundingClientRect().height || 0;
      const totalHeight = anchorRect.height + panelHeight;

      this.overlayStyle = {
        position: "fixed",
        top: `${anchorRect.bottom}px`,
        left: `${anchorRect.left}px`,
        width: `${anchorRect.width}px`,
        zIndex: 1000,
      };

      this.shadowStyle = {
        position: "fixed",
        top: `${anchorRect.top}px`,
        left: `${anchorRect.left}px`,
        width: `${anchorRect.width}px`,
        height: `${totalHeight}px`,
        zIndex: 999,
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

.expand-dropdown-anchor {
  box-sizing: border-box;
  border: 1px solid var(--surfaceSecondary);
  transition:
    border-radius 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    border-color 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.expand-dropdown--open .expand-dropdown-anchor {
  position: relative;
  z-index: 1000;
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
  border-bottom-color: var(--background);
  box-shadow: none;
}

.expand-dropdown-shadow {
  pointer-events: none;
  background: transparent;
  border-radius: 1em;
  box-shadow:
    0 1px 1px hsl(0deg 0% 0% / 0.075),
    0 2px 2px hsl(0deg 0% 0% / 0.075),
    0 4px 4px hsl(0deg 0% 0% / 0.075),
    0 8px 8px hsl(0deg 0% 0% / 0.075),
    0 16px 16px hsl(0deg 0% 0% / 0.075);
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.expand-dropdown-shadow--closing {
  transition-duration: 0.15s;
}

.expand-dropdown-overlay {
  box-sizing: border-box;
  background: transparent;
  overflow: visible;
  z-index: 1000;
}

.expand-dropdown-body {
  box-sizing: border-box;
  overflow: hidden;
  width: 100%;
  background-color: var(--background);
  border-style: solid;
  border-color: var(--surfaceSecondary);
  border-width: 1px 1px 1px;
  border-top-color: var(--background);
  border-top-left-radius: 0;
  border-top-right-radius: 0;
  border-bottom-left-radius: 1em;
  border-bottom-right-radius: 1em;
  box-shadow: none;
  padding-top: 0;
}

.expand-dropdown-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  padding-left: 0.5em;
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
  padding: 0;
  transform: rotate(0deg);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.expand-dropdown-chevron--open {
  transform: rotate(180deg);
}

.expand-enter-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-leave-active {
  transition: height 0.15s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.15s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}
</style>
