<template>
  <ul v-if="visible" class="password-requirements" aria-live="polite">
    <li :class="{ met: state.passwordsMatch, unmet: state.showMismatch }">
      <i class="material-symbols-outlined" aria-hidden="true">{{
        state.passwordsMatch ? "check_circle" : "cancel"
      }}</i>
      {{ $t("settings.passwordRequirementsMatch") }}
    </li>
    <li :class="{ met: state.minLengthMet, unmet: state.showLengthHint && !state.minLengthMet }">
      <i class="material-symbols-outlined" aria-hidden="true">{{
        state.minLengthMet ? "check_circle" : "cancel"
      }}</i>
      {{ $t("settings.passwordRequirementsMinLength", { min: state.minLength }) }}
    </li>
  </ul>
</template>

<script>
import { evaluatePasswordPolicy } from "@/utils/passwordPolicy.js";

export default {
  name: "PasswordRequirementsHint",
  props: {
    password: {
      type: String,
      default: "",
    },
    confirmPassword: {
      type: String,
      default: "",
    },
    /** When false, hide until the user types in either field. */
    showWhenEmpty: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    state() {
      return evaluatePasswordPolicy(this.password, this.confirmPassword);
    },
    visible() {
      if (this.showWhenEmpty) {
        return true;
      }
      return (
        String(this.password ?? "").length > 0 || String(this.confirmPassword ?? "").length > 0
      );
    },
  },
};
</script>

<style scoped>
.password-requirements {
  list-style: none;
  padding: 0;
  margin: 0.25em 0 0.75em;
  font-size: 0.9rem;
}

.password-requirements li {
  display: flex;
  align-items: center;
  gap: 0.35em;
  color: var(--textSecondary, #666);
}

.password-requirements li.met {
  color: var(--green, #2e7d32);
}

.password-requirements li.unmet {
  color: var(--red, #c62828);
}

.password-requirements i {
  font-size: 1.1em;
}
</style>
