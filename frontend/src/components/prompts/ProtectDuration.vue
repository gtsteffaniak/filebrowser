<template>
  <div class="card-title">
    <h2>{{ $t("prompts.protectDuration") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.protectDurationMessage") }}</p>
    <div class="protect-input-row">
      <input
        aria-label="Protection duration in hours"
        class="input protect-hours-input"
        type="number"
        min="1"
        max="87600"
        v-model.number="hours"
        v-focus
        @keyup.enter="submit"
      />
      <span class="protect-unit">{{ $t("prompts.protectHours") }}</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </div>
  </div>

  <div class="card-action">
    <button
      class="button button--flat button--grey"
      @click="closeHovers"
      :aria-label="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      class="button button--flat"
      :disabled="!validHours"
      @click="submit"
      :aria-label="$t('buttons.protect')"
    >
      {{ $t("buttons.protect") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { chainfsApi } from "@/api";
import { notify } from "@/notify";

export default {
  name: "ProtectDuration",
  props: {
    item: {
      type: Object,
      required: true,
    },
    source: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      hours: 24,
    };
  },
  computed: {
    validHours() {
      return Number.isInteger(this.hours) && this.hours >= 1;
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async submit() {
      if (!this.validHours) return;
      mutations.closeHovers();
      const toastId = notify.showToast("info", this.$t("prompts.protectUploading"), {
        icon: "sync",
        duration: 0,
      });
      const minEnd = Date.now() + 1500;
      try {
        await chainfsApi.protectFile(this.source, this.item.path, this.hours);
        const remaining = minEnd - Date.now();
        if (remaining > 0) await new Promise((r) => setTimeout(r, remaining));
        notify.closeToast(toastId);
        notify.showSuccessToast(this.$t("buttons.protectSuccess"));
        mutations.setReload(true);
      } catch (_) {
        const remaining = minEnd - Date.now();
        if (remaining > 0) await new Promise((r) => setTimeout(r, remaining));
        notify.closeToast(toastId);
        // error already shown by API layer
      }
    },
  },
};
</script>

<style scoped>
.protect-input-row {
  display: flex;
  align-items: center;
  gap: 0.5em;
  margin-top: 0.75em;
}

.protect-hours-input {
  width: 7em;
}

.protect-unit {
  color: var(--textSecondary, #888);
  font-size: 0.9em;
}
</style>
