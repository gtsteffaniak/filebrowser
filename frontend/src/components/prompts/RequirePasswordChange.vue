<template>
  <div class="card-content">
    <div v-if="error !== ''" class="wrong-login card">{{ error }}</div>
    <p>{{ $t("login.requirePasswordChangeInstructions") }}</p>
    <label for="new-password">{{ $t("general.password") }}</label>
    <input
      id="new-password"
      v-focus
      class="input"
      :class="{ 'form-invalid': showFieldInvalid }"
      type="password"
      autocomplete="new-password"
      v-model="newPassword"
      :placeholder="$t('settings.enterPassword')"
    />
    <input
      class="input"
      :class="{ 'form-invalid': showFieldInvalid }"
      type="password"
      autocomplete="new-password"
      v-model="passwordConfirm"
      :placeholder="$t('settings.enterPasswordAgain')"
      @keydown.enter.prevent="submit"
    />
    <PasswordRequirementsHint :password="newPassword" :confirm-password="passwordConfirm" />
    <template v-if="needsOtpInput">
      <label for="require-change-otp">{{ $t("otp.codeInputPlaceholder") }}</label>
      <input
        id="require-change-otp"
        class="input"
        :class="{ 'form-invalid': showFieldInvalid }"
        type="text"
        autocomplete="one-time-code"
        v-model="otp"
        @keydown.enter.prevent="submit"
      />
    </template>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="submitInFlight || !canSubmit"
      @click="submit"
    >
      {{ $t("general.update") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { notify } from "@/notify";
import { authApi } from "@/api";
import { initAuth } from "@/utils/auth";
import { evaluatePasswordPolicy } from "@/utils/passwordPolicy.js";
import PasswordRequirementsHint from "@/components/PasswordRequirementsHint.vue";

export default {
  name: "requirePasswordChange",
  components: {
    PasswordRequirementsHint,
  },
  props: {
    redirect: {
      type: String,
      default: "",
    },
    username: {
      type: String,
      default: "",
    },
    password: {
      type: String,
      default: "",
    },
    /** When true, show the TOTP field immediately (user already uses 2FA). */
    otpRequired: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      error: "",
      newPassword: "",
      passwordConfirm: "",
      otp: "",
      submitInFlight: false,
      showOtpField: false,
    };
  },
  computed: {
    needsOtpInput() {
      return this.otpRequired || this.showOtpField;
    },
    passwordPolicy() {
      return evaluatePasswordPolicy(this.newPassword, this.passwordConfirm);
    },
    showFieldInvalid() {
      if (this.error !== "") {
        return true;
      }
      const p = this.passwordPolicy;
      return p.showMismatch || (p.showLengthHint && !p.minLengthMet);
    },
    canSubmit() {
      if (
        String(this.password ?? "").trim() === "" ||
        !this.passwordPolicy.valid ||
        this.newPassword === this.password
      ) {
        return false;
      }
      if (this.needsOtpInput && String(this.otp ?? "").trim() === "") {
        return false;
      }
      return true;
    },
  },
  watch: {
    newPassword() {
      this.error = "";
    },
    passwordConfirm() {
      this.error = "";
    },
  },
  methods: {
    otpRequiredForMessage(message) {
      const text = String(message ?? "");
      return text.includes("OTP code is required for user");
    },
    async submit(event) {
      event?.preventDefault?.();
      if (this.submitInFlight || !this.canSubmit) {
        return;
      }
      if (!this.passwordPolicy.valid) {
        this.error = this.$t("settings.passwordRequirementsMinLength", {
          min: this.passwordPolicy.minLength,
        });
        notify.showError(this.error);
        return;
      }
      if (this.newPassword === this.password) {
        this.error = this.$t("login.requirePasswordChangeMustDiffer");
        notify.showError(this.error);
        return;
      }
      this.submitInFlight = true;
      try {
        await authApi.changeRequiredPassword(this.username, this.password, this.newPassword, this.otp);
        await initAuth();
        const path = this.redirect !== "" ? this.redirect : "/files/";
        await this.$router.push(path);
        notify.showSuccessToast(this.$t("login.requirePasswordChangeSuccess"));
        mutations.closeTopPrompt();
      } catch (err) {
        const message = err.message || this.$t("login.failedLogin");
        if (this.otpRequiredForMessage(message)) {
          this.showOtpField = true;
        }
        this.error = message;
        notify.showError(this.error);
      } finally {
        this.submitInFlight = false;
      }
    },
  },
};
</script>

<style scoped>
.wrong-login {
  background: var(--red) !important;
  color: #fff;
  padding: 0.5em;
  text-align: center;
  margin-bottom: 0.5em;
}

.input {
  margin-bottom: 0.5em;
}
</style>
