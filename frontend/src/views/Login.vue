<template>
  <Tooltip />
  <div id="login" :class="{ recaptcha: globalVars.recaptcha, 'dark-mode': isDarkMode }">
    <form class="card login-card" @submit="submit">
      <div class="login-brand">
        <Icon mimetype="directory" />
      </div>
      <div v-if="!inProgress" class="login-brand brand-text">
        <h3>{{ loginName }}</h3>
      </div>
      <transition name="login-options" @before-enter="beforeEnter" @enter="enter" @leave="leave">
        <div v-if="inProgress" class="loading-spinner">
          <i class="material-icons spin">sync</i>
        </div>
        <div v-else class="loginOptions no-padding" key="loginForm">
          <div v-if="passwordAvailable" class="password-entry">
            <div v-if="error !== ''" class="wrong-login card">
              <span>{{ $t("login.failedLogin") }}</span>
              <i class="no-select material-symbols-outlined tooltip-info-icon" @mouseenter="showTooltip($event, error)"
                @mouseleave="hideTooltip">
                help <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </i>
            </div>
            <input autofocus class="input" type="text" autocapitalize="off" v-model="username"
              :placeholder="$t('general.username')" />
            <input class="input" type="password" v-model="password" :placeholder="$t('general.password')" />
            <input class="input" v-if="createMode" type="password" v-model="passwordConfirm"
              :placeholder="$t('login.passwordConfirm')" />

            <div v-if="globalVars.recaptcha" id="globalVars.recaptcha"></div>
            <input class="button button--block" type="submit"
              :value="createMode ? $t('general.signup') : $t('login.submit')" />
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
        </div>
      </transition>
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
import Tooltip from "@/components/Tooltip.vue";

export default {
  name: "login",
  components: {
    Icon,
    Prompts,
    Tooltip,
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
      inProgress: false,
    };
  },
  mounted() {
    let redirect = state.route.query.redirect;
    if (redirect) {
      redirect = removeLeadingSlash(redirect);
      redirect = globalVars.baseURL + redirect;
      this.loginURL += `?redirect=${encodeURIComponent(redirect)}`;
      // If password auth is disabled and OIDC is available, auto-redirect
      // Only auto-redirect when there's a valid redirect URL (user needs to go somewhere)
      if (!globalVars.passwordAvailable && globalVars.oidcAvailable) {
        window.location.href = this.loginURL;
        return;
      }
    }
    if (!globalVars.recaptcha) return;
    window.globalVars.recaptcha.ready(function () {
      window.globalVars.recaptcha.render("globalVars.recaptcha", {
        sitekey: globalVars.globalVars.recaptchaKey,
      });
    });
  },
  methods: {
    beforeEnter(el) {
      el.style.height = '0';
      el.style.opacity = '0';
    },
    enter(el, done) {
      el.style.transition = '';
      el.style.height = '0';
      el.style.opacity = '0';
      // Force reflow
      void el.offsetHeight;
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = el.scrollHeight + 'px';
      el.style.opacity = '1';
      setTimeout(() => {
        el.style.height = 'auto';
        done();
      }, 300);
    },
    leave(el, done) {
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = el.scrollHeight + 'px';
      el.style.opacity = '1';
      // Force reflow
      void el.offsetHeight;
      el.style.height = '0';
      el.style.opacity = '0';
      setTimeout(done, 300);
    },
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
    toggleMode() {
      this.createMode = !this.createMode;
    },
    async submit(event) {
      this.inProgress = true;
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
          this.inProgress = false;
          return;
        }
      }

      if (this.createMode) {
        if (this.password !== this.passwordConfirm) {
          this.error = this.$t("login.passwordsDontMatch");
          this.inProgress = false;
          return;
        }
      }
      try {
        if (this.createMode) {
          await usersApi.signupLogin(this.username, this.password);
        }
        await usersApi.login(this.username, this.password, captcha);
        await initAuth();
        if (state.user?.defaultLandingPage && state.user.defaultLandingPage !== "") {
          let landingPage = state.user.defaultLandingPage;
          // Remove protocol and domain if full URL was provided
          if (landingPage.includes("://")) {
            const protocolEnd = landingPage.indexOf("://");
            const pathStart = landingPage.indexOf("/", protocolEnd + 3);
            if (pathStart !== -1) {
              landingPage = landingPage.substring(pathStart);
            }
          }
          // Remove baseURL prefix if present
          if (globalVars.baseURL !== "/" && landingPage.startsWith(globalVars.baseURL)) {
            landingPage = landingPage.substring(globalVars.baseURL.length);
          }
          // Ensure single leading slash
          while (landingPage.startsWith("//")) {
            landingPage = landingPage.substring(1);
          }
          if (!landingPage.startsWith("/")) {
            landingPage = "/" + landingPage;
          }
          // Prevent redirect loop if landing page is the login page
          if (!landingPage.includes("/login")) {
            redirect = landingPage;
          }
        }
        router.push({ path: redirect });
      } catch (e) {
        console.log(e);
        this.inProgress = false;
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
        } else if (e.message == 401) {
          this.error = this.$t("login.invalidCredentials");
        } else {
          this.error = e.message;
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
  width: 100%;
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

.wrong-login {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.5em 1em;
}

.loginOptions {
  text-align: center;
  display: flex;
  align-content: center;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-direction: column;
}

.login-options-enter-active,
.login-options-leave-active {
  transition: height 0.3s ease, opacity 0.3s ease;
}

.login-options-enter-from,
.login-options-leave-to {
  height: 0;
  opacity: 0;
}
</style>