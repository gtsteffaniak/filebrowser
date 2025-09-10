<template>
  <div id="login" :class="{ recaptcha: globalVars.recaptcha, 'dark-mode': isDarkMode }">
    <form class="card login-card" @submit="submit">
      <div class="login-brand">
        <Icon mimetype="directory" />
      </div>
      <div class="login-brand brand-text">
        <h3>{{ loginName }}</h3>
      </div>
      <div v-if="passwordAvailable" class="password-entry">
        <div v-if="error !== ''" class="wrong-login card">{{ error }}</div>
        <input
          autofocus
          class="input"
          type="text"
          autocapitalize="off"
          v-model="username"
          :placeholder="$t('general.username')"
        />
        <input
          class="input"
          type="password"
          v-model="password"
          :placeholder="$t('general.password')"
        />
        <input
          class="input"
          v-if="createMode"
          type="password"
          v-model="passwordConfirm"
          :placeholder="$t('login.passwordConfirm')"
        />

        <div v-if="globalVars.recaptcha" id="globalVars.recaptcha"></div>
        <input
          class="button button--block"
          type="submit"
          :value="createMode ? $t('general.signup') : $t('login.submit')"
        />
        <p @click="toggleMode" v-if="signup">
          {{ createMode ? $t("login.loginInstead") : $t("login.createAnAccount") }}
        </p>
      </div>
      <div v-if="oidcAvailable" class="password-entry">
        <div v-if="passwordAvailable" class="or">{{ $t("login.or") }}</div>
        <a :href="loginURL" class="button button--block direct-login">
          <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          OpenID Connect
        </a>
      </div>
    </form>
  </div>
  <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
</template>

<script>
import router from "@/router";
import { mutations, state } from "@/store";
import Prompts from "@/components/prompts/Prompts.vue";
import Icon from "@/components/files/Icon.vue";
import { usersApi } from "@/api";
import { initAuth } from "@/utils/auth";
import { removeLeadingSlash } from "@/utils/url";
import { globalVars, logoURL } from "@/utils/constants";

export default {
  name: "login",
  components: {
    Icon,
    Prompts,
  },
  computed: {
    globalVars: () => globalVars,
    signup: () => globalVars.signup,
    oidcAvailable: () => globalVars.oidcAvailable,
    passwordAvailable: () => globalVars.passwordAvailable,
    name: () => globalVars.name || "FileBrowser Quantum",
    logoURL: () => logoURL,
    isDarkMode() {
      return globalVars.darkMode;
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
      recaptcha: globalVars.recaptcha,
      passwordConfirm: "",
      loginURL: globalVars.baseURL + "api/auth/oidc/login",
    };
  },
  mounted() {
    mutations.setCurrentUser(null);
    mutations.setJWT("");
    document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
    let redirect = state.route.query.redirect;
    if (redirect === "" || redirect === undefined || redirect === null) {
      redirect = globalVars.baseURL + "files/";
    } else {
      redirect = removeLeadingSlash(redirect);
      redirect = globalVars.baseURL + redirect;
    }
    this.loginURL += `?redirect=${encodeURIComponent(redirect)}`;
    if (!globalVars.recaptcha) return;
    window.gglobalVars.recaptcha.ready(function () {
      window.gglobalVars.recaptcha.render("globalVars.recaptcha", {
        sitekey: globalVars.globalVars.recaptchaKey,
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
      if (globalVars.recaptcha) {
        captcha = window.gglobalVars.recaptcha.getResponse();
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
          await usersApi.signupLogin(this.username, this.password);
        }
        await usersApi.login(this.username, this.password, captcha);
        await initAuth();
        router.push({ path: redirect });
      } catch (e) {
        if (e.message.includes("OTP authentication is enforced")) {
          mutations.showHover({
            name: "totp",
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
            },
          });
        }
        if (e.message.includes("OTP is enforced, but user is not yet configured")) {
          mutations.showHover({
            name: "totp",
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
              generate: true,
            },
          });
        } else if (e.message.includes("OTP code is required for user")) {
          mutations.showHover({
            name: "totp",
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
              generate: false,
            },
          });
        } else if (e.message == 409) {
          this.error = this.$t("login.usernameTaken");
        } else {
          this.error = this.$t("login.wrongCredentials");
        }
      }
    },
  },
};
</script>

<style >
.password-entry .input {
  margin-bottom: 0.5em;
}

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