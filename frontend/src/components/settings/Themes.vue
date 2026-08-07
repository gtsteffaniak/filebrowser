<template>
  <ExpandDropdown
    :model-value="theme"
    :options="themeOptions"
    :aria-label="$t('settings.themeColor')"
    @update:model-value="$emit('update:theme', $event)"
  />
</template>

<script>
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";

export default {
  name: "themes",
  components: {
    ExpandDropdown,
  },
  props: ["theme"],
  emits: ["update:theme"],
  computed: {
    themeOptions() {
      return [
        { value: "", label: this.$t("general.light") },
        { value: "dark", label: this.$t("general.dark") },
      ];
    },
  },
};
</script>
