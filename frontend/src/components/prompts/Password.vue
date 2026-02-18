<template>
  <div class="card-content">
    <div v-if="showWrongCredentials" class="form-invalid">
      {{ $t("login.wrongCredentials") }}
    </div>
    <div class="form-flex-group">
      <input
        class="input share-password"
        v-focus
        type="password"
        :placeholder="$t('general.password')"
        v-model="password"
        @keyup.enter="submit"
      />
    </div>
  </div>

  <div class="card-actions">
    <button
      class="button button--flat"
      @click="submit"
      :aria-label="$t('general.login')"
      :title="$t('general.login')"
    >
      {{ $t("general.login") }}
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
  },
  data() {
    return {
      password: this.initialPassword,
    };
  },
  methods: {
    submit() {
      mutations.closeTopHover();
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
