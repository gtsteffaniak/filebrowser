<template>
  <ExpandDropdown
    :model-value="locale"
    :options="languageOptions"
    :aria-label="$t('general.language')"
    :disabled="disabled"
    @update:model-value="$emit('update:locale', $event)"
  />
</template>

<script>
import { defineComponent } from "vue";
import { locales } from "@/i18n/locales.ts";
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
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  emits: {
    "update:locale": (locale) => typeof locale === "string",
  },
  computed: {
    languageOptions() {
      return Object.entries(locales).map(([id, { name }]) => ({
        value: id,
        label: name,
      }));
    },
  },
});
</script>
