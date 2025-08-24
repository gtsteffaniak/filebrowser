<template>
  <div class="card-title">
    <h2>{{ $t("general.password") }}</h2>
  </div>

  <div class="card-content">
    <div v-if="showWrongCredentials" class="share__wrong__password">
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

  <div class="card-action">
    <button
      class="button button--flat"
      @click="submit"
      :aria-label="$t('buttons.submit')"
      :title="$t('buttons.submit')"
    >
      {{ $t("buttons.submit") }}
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
      mutations.closeHovers();
      this.submitCallback(this.password);
    },
  },
};
</script>

<style scoped>
.share__wrong__password {
  color: #ff4757;
  text-align: center;
  padding: 1em 0;
}

.share-password {
  width: 100%;
}
</style>
