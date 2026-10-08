<template>
  <button
    type="button"
    class="button button--flat list-filter-toggle"
    :disabled="disabled"
    @click="toggle"
    :aria-label="open ? $t('general.cancel') : $t('general.search')"
    :title="open ? $t('general.cancel') : $t('general.search')"
  >
    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
    <i class="material-symbols">{{ open ? "close" : "search" }}</i>
  </button>
  <input
    v-if="open"
    ref="input"
    type="text"
    class="input filter-input"
    :value="modelValue"
    :placeholder="$t('general.search')"
    :aria-label="$t('general.search')"
    @input="$emit('update:modelValue', $event.target.value)"
    @keydown.enter.prevent.stop
    @keydown.esc.prevent.stop="toggle"
  />
</template>

<script>
export default {
  name: "listing-filter",
  props: {
    modelValue: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  data() {
    return {
      open: false,
    };
  },
  beforeUnmount() {
    this.$emit("update:modelValue", "");
  },
  methods: {
    async toggle() {
      this.open = !this.open;
      this.$emit("update:modelValue", "");
      if (this.open) {
        await this.$nextTick();
        this.$refs.input?.focus();
      }
    },
  },
};
</script>

<style scoped>
.list-filter-toggle {
  margin-right: auto;
}

.filter-input {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  padding-block: 0;
}
</style>
