<template>
  <div class="form-flex-group">
    <ExpandDropdown
      :model-value="viewMode"
      :options="viewModeOptions"
      :disabled="disabled"
      :aria-label="$t('buttons.normalView')"
      @update:model-value="$emit('update:viewMode', $event)"
    />
  </div>
</template>

<script>
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";

export default {
  name: "ViewMode",
  components: {
    ExpandDropdown,
  },
  props: {
    viewMode: {
      type: String,
      default: "normal",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:viewMode"],
  computed: {
    viewModeOptions() {
      return this.viewModes.map((mode) => ({
        value: mode,
        label: mode,
      }));
    },
  },
  data() {
    return {
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },
};
</script>
