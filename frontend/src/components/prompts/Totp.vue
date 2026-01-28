<template>
  <div class="card-title">
    <h2>{{ $t("otp.name") }}</h2>
  </div>
  <div v-if="error !== ''" class="wrong-login card">{{ error }}</div>
  <div v-if="succeeded">{{ $t("otp.verificationSucceed") }}</div>
  <div v-if="!succeeded" class="card-content">
    <p v-if="generate">{{ $t("otp.generate") }}</p>
    <div v-if="generate" class="box__element box__center">
      <p aria-label="otp-url">{{ this.url }}</p>
      <qrcode-vue class="qrcode" :value="this.url" size="200" level="M"></qrcode-vue>
    </div>
    <p>{{ $t("otp.verifyInstructions") }}</p>
    <input :class="{ 'form-invalid': !succeeded }" v-focus class="input" type="text" v-model="code" @keyup.enter="verifyCode"
      :placeholder="$t('otp.codeInputPlaceholder')" />
  </div>

  <div class="card-action">
    <button @click="closeHovers" class="button button--flat button--grey"
      :aria-label="succeeded ? $t('general.close') : $t('general.cancel')"
      :title="succeeded ? $t('general.close') : $t('general.cancel')">
      {{ succeeded ? $t('general.close') : $t('general.cancel') }}
    </button>
    <button v-if="!succeeded" class="button button--flat button--blue" @click="verifyCode"
      :title="$t('general.verify')">
      {{ $t("general.verify") }}
    </button>
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
        notify.showSuccessToast(this.$t("otp.verificationSucceed"));
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

<style scoped>
.box {
  box-shadow: rgba(0, 0, 0, 0.06) 0px 1px 3px, rgba(0, 0, 0, 0.12) 0px 1px 2px;
  background: #fff;
  border-radius: 1em;
  margin: 5px;
  overflow: hidden;
}

.box__header {
  padding: 1em;
  text-align: center;
}

.box__icon i {
  font-size: 10em;
  color: #40c4ff;
}

.box__center {
  text-align: center;
}

.box__info {
  flex: 1 1 18em;
}

.box__element {
  padding: 1em;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  word-break: break-all;
}

.box__element .button {
  display: inline-block;
}

.box__element .button i {
  display: block;
  margin-bottom: 4px;
}

.box__items {
  text-align: left;
  flex: 10 0 25em;
}

</style>