<template>
  <ExpandDropdown
    :model-value="locale"
    :options="languageOptions"
    :aria-label="$t('general.language')"
    @update:model-value="$emit('update:locale', $event)"
  />
</template>

<script>
import { defineComponent } from "vue";
import { availableLocales } from "@/i18n/index.ts";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";

export default defineComponent({
  name: "Languages",
  components: {
    ExpandDropdown,
  },
  props: {
    locale: {
      type: String,
      required: true,
    },
  },
  emits: ["update:locale"],
  computed: {
    languageOptions() {
      return Object.keys(this.locales).map((label) => ({
        value: label,
        label: this.$t(`languages.${label}`),
      }));
    },
  },
  data() {
    return {
      locales: availableLocales,
    };
  },
});
</script>
