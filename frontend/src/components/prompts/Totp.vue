<template>
  <div class="card floating create-api__prompt__card" id="create-api">
    <div class="card-title">
      <h2>{{ $t("otp.name") }}</h2>
    </div>
    <div v-if="error !== ''" class="wrong-login">{{ error }}</div>
    <div v-if="succeeded" >{{ $t("otp.verificationSucceed") }}</div>
    <div v-if="!succeeded" class="card-content">
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
        :aria-label="succeeded ? $t('buttons.close') : $t('buttons.cancel')"
        :title="succeeded ? $t('buttons.close') : $t('buttons.cancel')"
      >
        {{ succeeded ? $t('buttons.close') : $t('buttons.cancel') }}
      </button>
      <button
        v-if="!succeeded"
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
      succeeded: false,
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
          this.$router.push(this.redirect);
        }
        mutations.updateCurrentUser({ otpEnabled: true });
        this.succeeded = true
        this.error = "";
        notify.showSuccess(this.$t("otp.verificationSucceed"));
      } catch (error) {
        this.error = this.$t("otp.verificationFailed");
        return;
      }
    },
    closeHovers() {
      if (!this.succeeded) {
        mutations.updateCurrentUser({ otpEnabled: false });
      }
      mutations.closeHovers();
    },
  },
};
</script>
