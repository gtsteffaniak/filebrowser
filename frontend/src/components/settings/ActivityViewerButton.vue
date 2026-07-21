<template>
  <SettingsButton
    :name="name || $t('tools.activityViewer.viewActivity')"
    :description="description || $t('tools.activityViewer.description')"
    :disabled="disabled"
    @click="openActivityViewer"
  />
</template>

<script>
import SettingsButton from "@/components/settings/SettingsButton.vue";
import { navigateActivityViewerHref } from "@/utils/activityViewerLink";

export default {
  name: "ActivityViewerButton",
  components: {
    SettingsButton,
  },
  props: {
    href: {
      type: String,
      required: true,
    },
    name: {
      type: String,
      default: "",
    },
    description: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  methods: {
    openActivityViewer() {
      navigateActivityViewerHref(this.href);
    },
  },
};
</script>
