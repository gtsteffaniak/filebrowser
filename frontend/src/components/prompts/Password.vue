<template>
  <div class="card-content">
    <div v-if="showWrongCredentials" class="form-invalid">
      {{ $t("login.wrongCredentials") }}
    </div>
    <div v-if="infoText">
      <p>{{ infoText }}</p>
    </div>
    <div class="form-flex-group">
      <input
        class="input share-password"
        v-focus
        type="password"
        :placeholder="$t('general.password')"
        v-model="password"
        @keyup.enter="canSubmit && submit()"
      />
    </div>
  </div>

  <div class="card-actions">
    <button
      class="button button--flat"
      type="button"
      @click="submit"
      :disabled="!canSubmit"
      :aria-label="submitButtonTitle"
      :title="submitButtonTitle"
    >
      {{ submitButtonTitle }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";

export default {
  name: "password",
  props: {
    submitCallback: {
      type: Function,
      required: true,
    },
    showWrongCredentials: {
      type: Boolean,
      default: false,
    },
    initialPassword: {
      type: String,
      default: "",
    },
    infoText: {
      type: String,
      default: "",
    },
    submitLabel: {
      type: String,
      default: "",
    },
  },
  computed: {
    submitButtonTitle() {
      return this.submitLabel || this.$t("general.login");
    },
    canSubmit() {
      return String(this.password ?? "").trim().length > 0;
    },
  },
  data() {
    return {
      password: this.initialPassword,
    };
  },
  methods: {
    submit() {
      if (!this.canSubmit) {
        return;
      }
      mutations.closeTopPrompt();
      this.submitCallback(this.password);
    },
  },
};
</script>

<style scoped>
.wrong__password {
  color: #ff4757;
  text-align: center;
  padding: 1em 0;
}

.share-password {
  width: 100%;
}
</style>
