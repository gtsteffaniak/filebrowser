<template>
  <div class="error-message chunk-load-error">
    <h2 class="message">
      <i class="material-symbols-outlined">error</i>
      <span>{{ $t('errors.internal') }}</span>
    </h2>
  </div>
</template>

<script>
import { notify } from "@/notify";

export default {
  name: "load-failed",
  props: {
    error: {
      type: Error,
      default: null,
    },
  },
  mounted() {
    console.error("[LoadFailed]", this.error);
    notify.showError(this.error?.message ?? this.$t("prompts.error"));
  },
};
</script>
