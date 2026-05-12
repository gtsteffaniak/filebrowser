<template>
  <div class="card-title">
    <h2>{{ $t("prompts.safeModeUnlock") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.safeModeUnlockMessage") }}</p>

    <div class="safemode-pin-row">
      <input
        ref="pinInput"
        class="input safemode-pin-input"
        type="password"
        inputmode="numeric"
        maxlength="4"
        pattern="[0-9]{4}"
        :placeholder="$t('prompts.safeModePINPlaceholder')"
        v-model="pin"
        v-focus
        @keyup.enter="submit"
        autocomplete="off"
      />
    </div>

    <p v-if="errorMessage" class="safemode-error">{{ errorMessage }}</p>
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
      :disabled="pin.length !== 4 || loading"
      @click="submit"
    >
      {{ $t("prompts.safeModeUnlockAction") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { safeModeApi } from "@/api";
import { notify } from "@/notify";
import { url } from "@/utils";

export default {
  name: "SafeModeUnlock",
  props: {
    target: {
      type: Object,
      default: null,
    },
  },
  data() {
    return {
      pin: "",
      errorMessage: "",
      loading: false,
    };
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async submit() {
      if (this.pin.length !== 4 || this.loading) return;
      this.errorMessage = "";
      this.loading = true;
      try {
        const result = await safeModeApi.verifySafeModePin(this.pin);
        if (result.valid) {
          mutations.setSafeModeUnlocked(true);
          notify.showSuccessToast(this.$t("prompts.safeModeUnlocked"));
          mutations.closeHovers();
          if (this.target) {
            url.goToItem(this.target.source, this.target.path, null);
          }
        } else {
          this.errorMessage = this.$t("prompts.safeModePINIncorrect");
          this.pin = "";
          if (this.$refs.pinInput) this.$refs.pinInput.focus();
        }
      } catch (err) {
        this.errorMessage = err.message || this.$t("prompts.safeModeFailed");
        this.pin = "";
        if (this.$refs.pinInput) this.$refs.pinInput.focus();
      } finally {
        this.loading = false;
      }
    },
  },
};
</script>

<style scoped>
.safemode-pin-row {
  display: flex;
  align-items: center;
  gap: 0.5em;
  margin-top: 0.75em;
}

.safemode-pin-input {
  width: 8em;
  letter-spacing: 0.3em;
  text-align: center;
  font-size: 1.2em;
}

.safemode-error {
  color: var(--red, #e53935);
  font-size: 0.9em;
  margin-top: 0.5em;
}
</style>
