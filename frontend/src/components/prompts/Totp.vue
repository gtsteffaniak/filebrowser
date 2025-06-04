<template>
  <div class="card floating create-api__prompt__card" id="create-api">
    <div class="card-title">
      <h2>{{ $t("otp.name") }}</h2>
    </div>
    <div v-if="error !== ''" class="wrong-login">{{ error }}</div>

    <div class="card-content">
      <p v-if="generate">{{ $t("otp.generate") }}</p>
      <div v-if="generate" class="share__box__element share__box__center">
        <p>{{ this.url }}</p>
        <qrcode-vue class="qrcode" :value="this.url" size="200" level="M"></qrcode-vue>
      </div>
      <p>{{ $t("otp.verifyInstructions") }}</p>
      <input
        class="input input--block"
        type="text"
        v-model="code"
        @keyup.enter="verifyCode"
        :placeholder="$t('otp.codeInputPlaceholder')"
      />
    </div>
    <div class="card-action">
      <button
        @click="closeHovers"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat button--blue"
        @click="verifyCode"
        :title="$t('buttons.verify')"
      >
        {{ $t("buttons.verify") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";
import { initAuth } from "@/utils/auth";
import QrcodeVue from "qrcode.vue";

export default {
  name: "totp",
  components: {
    QrcodeVue,
  },
  data() {
    return {
      error: "",
      code: "",
      url: "",
    };
  },
  props: {
    redirect: {
      type: String,
      default: "",
    },
    generate: {
      type: Boolean,
      default: false,
    },
    username: {
      type: String,
      default: "",
    },
    password: {
      type: String,
      default: "",
    },
  },
  async mounted() {
    if (this.generate) {
      this.generateNewCode();
    }
  },
  methods: {
    async generateNewCode() {
      try {
        const resp = await usersApi.generateOTP(this.username, this.password);
        this.url = resp.url;
      } catch (error) {
        this.error = this.$t("otp.generationFailed");
        return;
      }
    },
    async verifyCode(event) {
      event.preventDefault();
      if (!this.code) {
        this.error = this.$t("otp.invalidCodeType");
        notify.showError(this.$t("otp.invalidCodeType"));
        return;
      }
      try {
        await usersApi.verifyOtp(this.username, this.password, this.code);
        if (this.redirect != "") {
          await usersApi.login(this.username, this.password, this.redirect, this.code);
          await initAuth();
          if (this.redirect) {
            this.$router.push(this.redirect);
          }
        } else {
          mutations.closeHovers();
        }
      } catch (error) {
        this.error = this.$t("otp.verificationFailed");
        return;
      }
    },
    closeHovers() {
      mutations.closeHovers();
    },
  },
};
</script>
