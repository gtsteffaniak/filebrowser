<template>
  <div id="login" :class="{ recaptcha: recaptcha, 'dark-mode': isDarkMode }">
    <form class="card login-card" @submit="submit">
      <div class="login-brand">
        <Icon mimetype="directory" />
      </div>
      <div class="login-brand brand-text">
        <h3>{{ loginName }}</h3>
      </div>
      <div v-if="passwordAvailable" class="password-entry">
        <div v-if="error !== ''" class="wrong">{{ error }}</div>
        <input
          autofocus
          class="input input--block"
          type="text"
          autocapitalize="off"
          v-model="username"
          :placeholder="$t('login.username')"
        />
        <input
          class="input input--block"
          type="password"
          v-model="password"
          :placeholder="$t('login.password')"
        />
        <input
          class="input input--block"
          v-if="createMode"
          type="password"
          v-model="passwordConfirm"
          :placeholder="$t('login.passwordConfirm')"
        />

        <div v-if="recaptcha" id="recaptcha"></div>
        <input
          class="button button--block"
          type="submit"
          :value="createMode ? $t('login.signup') : $t('login.submit')"
        />
        <p @click="toggleMode" v-if="signup">
          {{ createMode ? $t("login.loginInstead") : $t("login.createAnAccount") }}
        </p>
      </div>
      <div v-if="oidcAvailable" class="password-entry">
        <div v-if="passwordAvailable" class="or">{{ $t('login.or') }}</div>
        <a href="/api/auth/oidc/login" class="button button--block direct-login"> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          OpenID Connect
        </a>
      </div>
    </form>
  </div>
</template>

<script>
import router from "@/router";
import { state } from "@/store";
import Icon from "@/components/files/Icon.vue";
import { signupLogin, login, initAuth } from "@/utils/auth";
import {
  name,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
  darkMode,
  oidcAvailable,
  passwordAvailable,
} from "@/utils/constants";

export default {
  name: "login",
  components: {
    Icon,
  },
  computed: {
    signup: () => signup,
    oidcAvailable: () => oidcAvailable,
    passwordAvailable: () => passwordAvailable,
    name: () => name || "FileBrowser Quantum",
    logoURL: () => logoURL,
    isDarkMode() {
      return darkMode;
    },
    loginName() {
      return name;
    },
  },
  data: function () {
    return {
      createMode: false,
      error: "",
      username: "",
      password: "",
      recaptcha: recaptcha,
      passwordConfirm: "",
    };
  },
  mounted() {
    if (!recaptcha) return;
    window.grecaptcha.ready(function () {
      window.grecaptcha.render("recaptcha", {
        sitekey: recaptchaKey,
      });
    });
  },
  methods: {
    toggleMode() {
      this.createMode = !this.createMode;
    },
    async submit(event) {
      event.preventDefault();
      event.stopPropagation();
      let redirect = state.route.query.redirect;
      if (redirect === "" || redirect === undefined || redirect === null) {
        redirect = "/files/";
      }

      let captcha = "";
      if (recaptcha) {
        captcha = window.grecaptcha.getResponse();
        if (captcha === "") {
          this.error = this.$t("login.wrongCredentials");
          return;
        }
      }

      if (this.createMode) {
        if (this.password !== this.passwordConfirm) {
          this.error = this.$t("login.passwordsDontMatch");
          return;
        }
      }
      try {
        if (this.createMode) {
          await signupLogin(this.username, this.password);
        }
        await login(this.username, this.password, captcha);
        await initAuth();
        router.push({ path: redirect });
      } catch (e) {
        console.error(e);
        if (e.message == 409) {
          this.error = this.$t("login.usernameTaken");
        } else {
          this.error = this.$t("login.wrongCredentials");
        }
      }
    },
  },
};
</script>

<style>
.login-card {
  padding: 1em;
}

.login-brand {
  padding-bottom: 0 !important;
  padding: 0em !important;
  padding-top: 0.5em !important;
  display: flex;
  align-content: center;
  justify-content: center;
  align-items: center;
}

.brand-text {
  padding: 1em !important;
  padding-top: 0.9em !important;
}

.login-brand i {
  font-size: 5em !important;
  padding-top: 0em !important;
  padding-bottom: 0em !important;
}

.password-entry {
  padding: 0em !important;
}
.direct-login {
  display: flex !important;
  justify-content: center;
}

.or {
  margin-left: 4em;
  margin-right: 4em;
  position: relative;
  line-height: 50px;
  text-align: center;
}

.or::before,
.or::after {
  position: absolute;
  width: 2em;
  height: 1px;

  top: 24px;

  background-color: #aaa;

  content: "";
}

.or::before {
  left: 0;
}

.or::after {
  right: 0;
}
</style>
