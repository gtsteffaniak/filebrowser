<template>
  <Tooltip />
  <div id="login" :class="{ recaptcha: globalVars.recaptcha, 'dark-mode': isDarkMode, 'halloween-theme': eventTheme === 'halloween' }">
    <!-- Halloween Background Elements -->
    <div v-if="eventTheme === 'halloween'" class="halloween-background">
      <!-- Floating Clouds -->
      <div class="cloud cloud-1"></div>
      <div class="cloud cloud-2"></div>
      <div class="cloud cloud-3"></div>

      <!-- Lightning Flash -->
      <div class="lightning-flash"></div>

      <!-- Lightning Bolts - More Jagged -->
      <svg class="lightning-bolt lightning-1" viewBox="0 0 50 250" xmlns="http://www.w3.org/2000/svg">
        <path d="M 25 0 L 20 60 L 28 60 L 18 100 L 24 100 L 15 150 L 22 150 L 10 250 L 35 140 L 28 140 L 38 95 L 30 95 L 40 50 L 32 50 Z" fill="#fff" opacity="0"/>
      </svg>
      <svg class="lightning-bolt lightning-2" viewBox="0 0 50 250" xmlns="http://www.w3.org/2000/svg">
        <path d="M 25 0 L 22 50 L 30 50 L 20 100 L 26 100 L 17 140 L 24 140 L 12 250 L 37 135 L 29 135 L 35 90 L 28 90 L 38 55 L 30 55 Z" fill="#fff" opacity="0"/>
      </svg>
    </div>

    <form class="card login-card" :class="{ 'tombstone': eventTheme === 'halloween' }" @submit="submit">
      <div class="login-brand">
        <img :src="loginIconUrl" alt="Login Icon" class="login-icon" />
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

    <!-- Halloween Decorations -->
    <div v-if="eventTheme === 'halloween'" class="halloween-decorations">
      <!-- Spooky Black Cat - Side Profile -->
      <svg class="halloween-cat" viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg">
        <!-- Tail (curved up) -->
        <path d="M 30 160 Q 15 140 20 100 Q 22 80 28 70" stroke="#000" stroke-width="12" fill="none" stroke-linecap="round"/>

        <!-- Back Body -->
        <ellipse cx="70" cy="150" rx="45" ry="35" fill="#000"/>

        <!-- Front Body/Chest -->
        <ellipse cx="130" cy="145" rx="38" ry="40" fill="#000"/>

        <!-- Back Leg -->
        <rect x="60" y="175" width="14" height="20" rx="3" fill="#000"/>
        <rect x="58" y="192" width="18" height="6" rx="3" fill="#000"/>

        <!-- Front Leg -->
        <rect x="120" y="175" width="14" height="22" rx="3" fill="#000"/>
        <rect x="118" y="194" width="18" height="6" rx="3" fill="#000"/>

        <!-- Neck -->
        <ellipse cx="145" cy="130" rx="22" ry="28" fill="#000"/>

        <!-- Head -->
        <ellipse cx="165" cy="110" rx="28" ry="32" fill="#000"/>

        <!-- Ear (pointed) -->
        <path d="M 175 85 L 190 65 L 180 95 Z" fill="#000"/>

        <!-- Inner Ear -->
        <path d="M 180 85 L 186 72 L 182 90 Z" fill="#1a1a1a"/>

        <!-- Eye (glowing) -->
        <ellipse class="cat-eye" cx="172" cy="105" rx="7" ry="11" fill="#ff8c00"/>
        <ellipse class="cat-eye" cx="172" cy="107" rx="2" ry="7" fill="#000"/>

        <!-- Nose -->
        <path d="M 185 115 L 182 118 L 185 117 Z" fill="#ff8c00"/>

        <!-- Mouth/Jaw line -->
        <path d="M 185 117 Q 188 120 190 122" stroke="#1a1a1a" stroke-width="2" fill="none"/>

        <!-- Whiskers -->
        <line x1="185" y1="110" x2="210" y2="105" stroke="#888" stroke-width="1.5"/>
        <line x1="185" y1="115" x2="210" y2="115" stroke="#888" stroke-width="1.5"/>
        <line x1="185" y1="120" x2="210" y2="123" stroke="#888" stroke-width="1.5"/>
      </svg>

      <!-- Stylized Skeleton -->
      <svg class="halloween-skeleton" viewBox="0 0 200 300" xmlns="http://www.w3.org/2000/svg">
        <g class="skeleton-head">
          <!-- Skull (rounder, more cartoony) -->
          <ellipse cx="100" cy="45" rx="32" ry="38" fill="#f5f5f5"/>
          <rect x="82" y="65" width="36" height="20" rx="3" fill="#f5f5f5"/>

          <!-- Eye Sockets (bigger, more dramatic) -->
          <ellipse cx="88" cy="42" rx="10" ry="12" fill="#000"/>
          <ellipse cx="112" cy="42" rx="10" ry="12" fill="#000"/>

          <!-- Orange glow in eyes -->
          <ellipse cx="88" cy="42" rx="5" ry="6" fill="#ff8c00" opacity="0.8"/>
          <ellipse cx="112" cy="42" rx="5" ry="6" fill="#ff8c00" opacity="0.8"/>

          <!-- Nose (triangular) -->
          <path d="M 100 52 L 94 62 L 106 62 Z" fill="#000"/>

          <!-- Teeth (bigger gaps) -->
          <rect x="85" y="72" width="6" height="10" rx="1" fill="#000"/>
          <rect x="94" y="72" width="6" height="10" rx="1" fill="#000"/>
          <rect x="103" y="72" width="6" height="10" rx="1" fill="#000"/>
          <rect x="112" y="72" width="6" height="10" rx="1" fill="#000"/>
        </g>

        <!-- Neck vertebrae -->
        <circle cx="100" cy="88" r="5" fill="#f5f5f5"/>

        <!-- Ribcage/Torso -->
        <ellipse cx="100" cy="120" rx="35" ry="40" fill="#f5f5f5"/>

        <!-- Ribs (curved) -->
        <path d="M 75 105 Q 65 115 70 125" stroke="#000" stroke-width="3" fill="none"/>
        <path d="M 75 115 Q 65 125 70 135" stroke="#000" stroke-width="3" fill="none"/>
        <path d="M 75 125 Q 65 135 72 145" stroke="#000" stroke-width="3" fill="none"/>
        <path d="M 125 105 Q 135 115 130 125" stroke="#000" stroke-width="3" fill="none"/>
        <path d="M 125 115 Q 135 125 130 135" stroke="#000" stroke-width="3" fill="none"/>
        <path d="M 125 125 Q 135 135 128 145" stroke="#000" stroke-width="3" fill="none"/>

        <!-- Spine bumps -->
        <circle cx="100" cy="105" r="4" fill="#ddd"/>
        <circle cx="100" cy="115" r="4" fill="#ddd"/>
        <circle cx="100" cy="125" r="4" fill="#ddd"/>
        <circle cx="100" cy="135" r="4" fill="#ddd"/>
        <circle cx="100" cy="145" r="4" fill="#ddd"/>

        <!-- Pelvis (wider) -->
        <ellipse cx="100" cy="165" rx="28" ry="14" fill="#f5f5f5"/>
        <circle cx="85" cy="165" r="6" fill="#000"/>
        <circle cx="115" cy="165" r="6" fill="#000"/>

        <!-- Left Arm (bent at elbow) -->
        <rect x="60" y="95" width="8" height="38" rx="4" fill="#f5f5f5" transform="rotate(-20 64 95)"/>
        <circle cx="58" cy="130" r="6" fill="#f5f5f5"/>
        <rect x="52" y="130" width="8" height="32" rx="4" fill="#f5f5f5" transform="rotate(30 56 130)"/>

        <!-- Right Arm (bent at elbow) -->
        <rect x="132" y="95" width="8" height="38" rx="4" fill="#f5f5f5" transform="rotate(20 136 95)"/>
        <circle cx="142" cy="130" r="6" fill="#f5f5f5"/>
        <rect x="140" y="130" width="8" height="32" rx="4" fill="#f5f5f5" transform="rotate(-30 144 130)"/>

        <!-- Left Leg -->
        <rect x="83" y="178" width="9" height="55" rx="4" fill="#f5f5f5"/>
        <circle cx="87" cy="235" r="6" fill="#f5f5f5"/>
        <rect x="82" y="235" width="9" height="35" rx="4" fill="#f5f5f5"/>
        <!-- Foot -->
        <ellipse cx="86" cy="273" rx="12" ry="6" fill="#f5f5f5"/>

        <!-- Right Leg -->
        <rect x="108" y="178" width="9" height="55" rx="4" fill="#f5f5f5"/>
        <circle cx="112" cy="235" r="6" fill="#f5f5f5"/>
        <rect x="109" y="235" width="9" height="35" rx="4" fill="#f5f5f5"/>
        <!-- Foot -->
        <ellipse cx="113" cy="273" rx="12" ry="6" fill="#f5f5f5"/>
      </svg>
    </div>
  </div>
  <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
</template>

<script>
import router from "@/router";
import { mutations, state, getters } from "@/store";
import Prompts from "@/components/prompts/Prompts.vue";
import { usersApi } from "@/api";
import { initAuth } from "@/utils/auth";
import { removeLeadingSlash } from "@/utils/url";
import { globalVars, logoURL } from "@/utils/constants";
import Tooltip from "@/components/Tooltip.vue";

export default {
  name: "login",
  components: {
    Prompts,
    Tooltip,
  },
  computed: {
    eventTheme: () => getters.eventTheme(),
    globalVars: () => globalVars,
    signup: () => globalVars.signup,
    oidcAvailable: () => globalVars.oidcAvailable,
    passwordAvailable: () => globalVars.passwordAvailable,
    name: () => globalVars.name || "FileBrowser Quantum",
    logoURL: () => logoURL,
    loginIconUrl: () => globalVars.loginIcon || (globalVars.baseURL + "public/static/loginIcon"),
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

.login-icon {
  width: 5em;
  height: 5em;
  object-fit: contain;
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