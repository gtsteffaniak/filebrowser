<template>
  <div class="menu-option-list">
    <div v-if="allowSearch" class="menu-option-list-search">
      <input
        ref="searchInput"
        :value="searchQuery"
        type="search"
        class="input menu-option-list-search-input"
        :placeholder="resolvedSearchPlaceholder"
        @input="$emit('update:searchQuery', $event.target.value)"
        @keydown.esc.prevent="$emit('close')"
      />
    </div>
    <div ref="scroll" class="menu-option-list-scroll">
      <button
        v-for="option in filteredOptions"
        :key="optionKey(option.value)"
        type="button"
        role="option"
        class="action menu-option"
        :class="{
          'menu-option--selected': isSelected(option.value),
          'menu-option--disabled': option.disabled,
        }"
        :aria-selected="isSelected(option.value) ? 'true' : 'false'"
        :disabled="option.disabled"
        @click="$emit('select', option)"
      >
        <span>{{ option.label }}</span>
      </button>
      <p v-if="filteredOptions.length === 0" class="menu-option-list-empty">
        {{ resolvedEmptyLabel }}
      </p>
    </div>
  </div>
</template>

<script>
export default {
  name: "MenuOptionList",

  props: {
    options: {
      type: Array,
      default: () => [],
    },
    selectedKeys: {
      type: Array,
      default: () => [],
    },
    allowSearch: {
      type: Boolean,
      default: false,
    },
    searchQuery: {
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
  },

  emits: ["select", "update:searchQuery", "close"],

  computed: {
    resolvedSearchPlaceholder() {
      return this.searchPlaceholder || this.$t("general.search");
    },
    resolvedEmptyLabel() {
      return this.emptyLabel || this.$t("search.noResults");
    },
    normalizedOptions() {
      return (this.options || []).map((option) => ({
        value: option.value,
        label: option.label,
        disabled: !!option.disabled,
      }));
    },
    filteredOptions() {
      const query = this.searchQuery.trim().toLowerCase();
      if (!this.allowSearch || !query) {
        return this.normalizedOptions;
      }
      return this.normalizedOptions.filter((option) => {
        return String(option.label).toLowerCase().includes(query);
      });
    },
  },

  methods: {
    optionKey(value) {
      return String(value);
    },
    isSelected(value) {
      return this.selectedKeys.includes(this.optionKey(value));
    },
    focusSearch() {
      this.$refs.searchInput?.focus();
    },
  },
};
</script>

<style scoped>
.menu-option-list {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
}

.menu-option-list-search {
  padding: 0 0.25em 0.35em;
}

.menu-option-list-search-input {
  width: 100%;
  margin-top: 0.1em;
}

.menu-option-list-scroll {
  display: flex;
  flex-direction: column;
  gap: 0.1em;
  max-height: 14rem;
  overflow-y: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.menu-option-list-scroll::-webkit-scrollbar {
  display: none;
}

.menu-option-list :deep(.menu-option) {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  padding: 0.5em;
  color: var(--textPrimary);
  font: inherit;
}

.menu-option-list :deep(.menu-option span) {
  color: var(--textPrimary);
}

.menu-option-list :deep(.menu-option--selected) {
  background-color: rgba(0, 0, 0, 0.08);
  font-weight: 600;
}

.menu-option-list :deep(.menu-option--disabled) {
  opacity: 0.45;
  cursor: not-allowed;
}

.menu-option-list-empty {
  margin: 0.35em 0.5em;
  font-size: 0.9em;
  color: var(--textSecondary);
}
</style>

<style>
.dark-mode .menu-option-list .menu-option--selected {
  background-color: rgba(255, 255, 255, 0.08);
}
</style>
