<template>
  <Tooltip />
  <div id="login" :class="{ recaptcha: globalVars.recaptcha, 'halloween-theme': eventTheme === 'halloween' }">
    <!-- Halloween Background Elements -->
    <div v-if="eventTheme === 'halloween'" class="halloween-background">
      <!-- Floating Clouds -->
      <div class="cloud cloud-1"></div>
      <div class="cloud cloud-2"></div>
      <div class="cloud cloud-3"></div>

      <HalloweenLightning />
    </div>

    <form class="card login-card" :class="{ 'tombstone': eventTheme === 'halloween' }" @submit="submit">
      <span v-if="eventTheme === 'halloween'" class="tombstone-rip" aria-hidden="true">{{ $t("login.tombstoneRip") }}</span>
      <div class="login-brand">
        <img :src="loginIconUrl" alt="Login Icon" class="login-icon" />
      </div>
      <div v-if="!inProgress" class="login-brand brand-text">
        <h3>{{ loginName }}</h3>
      </div>
      <transition name="login-options" @before-enter="beforeEnter" @enter="enter" @leave="leave">
        <div v-if="inProgress" class="login-spinner-wrapper">
          <LoadingSpinner size="medium" />
        </div>
        <div v-else class="loginOptions no-padding" key="loginForm">
          <div v-if="passwordAvailable || ldapAvailable" class="password-entry">
            <div v-if="error !== ''" class="wrong-login card">
              <span>{{ $t("login.failedLogin") }}</span>
              <HelpTooltipIcon :text="error" />
            </div>
            <div class="field-wrap" :class="{ 'tombstone-field': eventTheme === 'halloween', born: eventTheme === 'halloween' }">
              <span v-if="eventTheme === 'halloween'" class="tombstone-label" aria-hidden="true"></span>
              <input autofocus class="input" type="text" autocapitalize="off" v-model="username"
                :placeholder="$t('general.username')" />
            </div>
            <div class="field-wrap" :class="{ 'tombstone-field': eventTheme === 'halloween', died: eventTheme === 'halloween' }">
              <span v-if="eventTheme === 'halloween'" class="tombstone-label" aria-hidden="true"></span>
              <input class="input" type="password" v-model="password" :placeholder="$t('general.password')" />
            </div>
            <input class="input" v-if="createMode" type="password" v-model="passwordConfirm"
              :placeholder="$t('login.passwordConfirm')" />

            <div v-if="globalVars.recaptcha" id="globalVars.recaptcha"></div>
            <input class="button button--block" type="submit" :disabled="globalVars.recaptcha && !recaptchaReady"
              :value="createMode ? $t('general.signup') : getLoginButtonValue()" />
            <p @click="toggleMode" v-if="signup" aria-label="sign up toggle">
              {{ createMode ? $t("login.loginInstead") : $t("login.createAnAccount") }}
            </p>
          </div>
          <div v-if="oidcAvailable" class="password-entry">
            <div v-if="passwordAvailable" class="or">{{ getOrLabel() }}</div>
            <a :href="loginURL" class="button button--block direct-login">
              {{ getLoginButtonText() }}
            </a>
          </div>
        </div>
      </transition>
    </form>

    <!-- Halloween Decorations -->
    <div v-if="eventTheme === 'halloween'" class="halloween-decorations">
      <!-- Spooky Black Cat - sitting silhouette with glowing, wandering eyes -->
      <svg class="halloween-cat" viewBox="0 0 200 240" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <clipPath id="cat-eye-clip-l"><path d="M 68 70 Q 82 56 96 70 Q 82 84 68 70 Z"/></clipPath>
          <clipPath id="cat-eye-clip-r"><path d="M 104 70 Q 118 56 132 70 Q 118 84 104 70 Z"/></clipPath>
          <linearGradient id="cat-fur" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#1c1c1f"/>
            <stop offset="1" stop-color="#050505"/>
          </linearGradient>
        </defs>

        <!-- Tail (curled, swaying) -->
        <g class="cat-tail">
          <path d="M 138 218 C 186 220 196 172 176 150 C 166 138 170 124 184 128"
            stroke="#0a0a0a" stroke-width="14" fill="none" stroke-linecap="round"/>
        </g>

        <!-- Body and haunch -->
        <path d="M 100 100 C 55 100 38 160 44 205 C 46 224 60 233 76 233 L 124 233 C 140 233 154 224 156 205 C 162 160 145 100 100 100 Z"
          fill="url(#cat-fur)"/>
        <ellipse cx="136" cy="200" rx="26" ry="30" fill="#0e0e10"/>

        <!-- Front legs and paws -->
        <rect x="76" y="168" width="18" height="62" rx="9" fill="#0b0b0c"/>
        <rect x="106" y="168" width="18" height="62" rx="9" fill="#0b0b0c"/>
        <ellipse cx="85" cy="231" rx="14" ry="7" fill="#0b0b0c"/>
        <ellipse cx="115" cy="231" rx="14" ry="7" fill="#0b0b0c"/>
        <path d="M 100 150 L 100 228" stroke="#1d1d20" stroke-width="1.5"/>

        <!-- Ears -->
        <path d="M 62 54 L 56 12 L 92 38 Z" fill="#0a0a0a"/>
        <path d="M 138 54 L 144 12 L 108 38 Z" fill="#0a0a0a"/>
        <path d="M 65 45 L 62 25 L 80 38 Z" fill="#2b1206"/>
        <path d="M 135 45 L 138 25 L 120 38 Z" fill="#2b1206"/>

        <!-- Head with cheek tufts -->
        <ellipse cx="100" cy="72" rx="43" ry="36" fill="url(#cat-fur)"/>
        <path d="M 58 78 L 47 92 L 66 91 Z" fill="#0a0a0a"/>
        <path d="M 142 78 L 153 92 L 134 91 Z" fill="#0a0a0a"/>

        <!-- Eyes: glowing almond with a slit pupil that wanders -->
        <g class="cat-eye">
          <path d="M 68 70 Q 82 56 96 70 Q 82 84 68 70 Z" fill="#ffb21a"/>
          <g clip-path="url(#cat-eye-clip-l)">
            <ellipse class="cat-pupil" cx="82" cy="70" rx="3.4" ry="13" fill="#000"/>
          </g>
          <path d="M 104 70 Q 118 56 132 70 Q 118 84 104 70 Z" fill="#ffb21a"/>
          <g clip-path="url(#cat-eye-clip-r)">
            <ellipse class="cat-pupil" cx="118" cy="70" rx="3.4" ry="13" fill="#000"/>
          </g>
          <circle cx="87" cy="66" r="1.8" fill="#fff" opacity="0.85"/>
          <circle cx="123" cy="66" r="1.8" fill="#fff" opacity="0.85"/>
        </g>

        <!-- Nose, mouth, whiskers -->
        <path d="M 95 86 L 105 86 L 100 92 Z" fill="#5a2a33"/>
        <path d="M 100 92 Q 95 99 88 95 M 100 92 Q 105 99 112 95" stroke="#3a3a3f" stroke-width="1.6" fill="none" stroke-linecap="round"/>
        <g stroke="#9a9aa2" stroke-width="1.2" opacity="0.7" stroke-linecap="round">
          <line x1="72" y1="88" x2="30" y2="80"/>
          <line x1="72" y1="93" x2="28" y2="96"/>
          <line x1="128" y1="88" x2="170" y2="80"/>
          <line x1="128" y1="93" x2="172" y2="96"/>
        </g>
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
  <prompts></prompts>
</template>

<script>
import router from "@/router";
import { mutations, state, getters } from "@/store";
import Prompts from "@/components/prompts/Prompts.vue";
import { authApi } from "@/api";
import { initAuth } from "@/utils/auth";
import { postLoginRedirectForServer, sanitizePostLoginRedirect } from "@/utils/safeRedirect.js";
import { globalVars } from "@/utils/constants";
import { defaultDarkMode, syncDocumentTheme } from "@/utils/theme";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import Tooltip from "@/components/Tooltip.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import HalloweenLightning from "@/components/HalloweenLightning.vue";

function loadRecaptcha(onReady, onError) {
  if (typeof window.grecaptcha !== "undefined") {
    onReady();
    return;
  }
  const host = globalVars.recaptchaHost; // commonly https://www.google.com/recaptcha/api.js
  if (!/^https:\/\//i.test(host)) { // Backend should already have filtered non-https URLs but we filter it here too just in case
    onError(new Error(`the configured recaptcha host is not a valid https URL`));
    return;
  }
  const existing = document.querySelector(`script[src="${host}"]`);
  if (!existing) {
    const script = document.createElement("script");
    script.src = host;
    script.async = true;
    script.defer = true;
    script.onerror = () => onError(new Error('Failed to load recaptcha script'));
    document.head.appendChild(script);
  }
  const startedAt = Date.now();
  const waitForRecaptcha = () => {
    if (typeof window.grecaptcha !== "undefined") {
      onReady();
      return;
    }
    if (Date.now() - startedAt > 15000) {
      onError(new Error("recaptcha loading timed out"));
      return;
    }
    setTimeout(waitForRecaptcha, 100);
  };
  waitForRecaptcha();
}

export default {
  name: "login",
  components: {
    Prompts,
    HelpTooltipIcon,
    Tooltip,
    LoadingSpinner,
    HalloweenLightning,
  },
  computed: {
    eventTheme: () => getters.eventTheme(),
    globalVars: () => globalVars,
    signup: () => globalVars.signup,
    oidcAvailable: () => globalVars.oidcAvailable,
    passwordAvailable: () => globalVars.passwordAvailable,
    ldapAvailable: () => globalVars.ldapAvailable,
    name: () => globalVars.name || "FileBrowser Quantum",
    loginIconUrl: () => globalVars.loginIcon,
    isDarkMode() {
      return defaultDarkMode();
    },
    loginName() {
      return this.name;
    },
  },
  data: () => ({
    createMode: false,
    error: "",
    username: "",
    password: "",
    recaptcha: globalVars.recaptcha,
    recaptchaReady: false,
    recaptchaId: null,
    passwordConfirm: "",
    loginURL: `${globalVars.baseURL}api/auth/oidc/login`,
    inProgress: false,
  }),
  watch: {
    // To render a new captcha whenever the login form is re-created (which can happen with wrong credentials)
    inProgress(isInProgress, wasInProgress) {
      if (!globalVars.recaptcha || !wasInProgress || isInProgress) return;
      this.recaptchaReady = false;
      this.$nextTick(() => this.renderRecaptcha());
    },
  },
  mounted() {
    syncDocumentTheme(this.isDarkMode);
    if (state.route.query.redirect) {
      const safeRedirect = sanitizePostLoginRedirect(state.route.query.redirect);
      const redirect = postLoginRedirectForServer(safeRedirect, globalVars.baseURL);
      this.loginURL += `?redirect=${encodeURIComponent(redirect)}`;
      // If password auth is disabled and OIDC is available, auto-redirect
      // Only auto-redirect when there's a valid redirect URL (user needs to go somewhere)
      if (!globalVars.passwordAvailable && globalVars.oidcAvailable) {
        window.location.href = this.loginURL;
        return;
      }
    }
    if (!globalVars.recaptcha) return;
    loadRecaptcha(
      () => this.renderRecaptcha(),
      (err) => console.error("recaptcha failed to load:", err)
    );
  },
  methods: {
    renderRecaptcha() {
      const container = document.getElementById("globalVars.recaptcha");
      if (!container) return;
      window.grecaptcha.ready(() => {
        this.recaptchaId = window.grecaptcha.render(container, {
          sitekey: globalVars.recaptchaKey,
        });
        this.recaptchaReady = true;
      });
    },
    getOrLabel() {
      return this.$t("general.or").toUpperCase();
    },
    getLoginButtonText() {
      return globalVars.oidcLoginButtonText || "OpenID Connect";
    },
    getLoginButtonValue() {
      if (globalVars.loginButtonText) {
        return globalVars.loginButtonText;
      }
      return this.$t("general.login");
    },
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
      el.style.height = `${el.scrollHeight}px`;
      el.style.opacity = '1';
      setTimeout(() => {
        el.style.height = 'auto';
        done();
      }, 300);
    },
    leave(el, done) {
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = `${el.scrollHeight}px`;
      el.style.opacity = '1';
      // Force reflow
      void el.offsetHeight;
      el.style.height = '0';
      el.style.opacity = '0';
      setTimeout(done, 300);
    },
    toggleMode() {
      this.createMode = !this.createMode;
    },
    async submit(event) {
      this.inProgress = true;
      event.preventDefault();
      event.stopPropagation();
      const redirect = sanitizePostLoginRedirect(state.route.query.redirect);

      let captcha = "";
      if (globalVars.recaptcha) {
        if (!this.recaptchaReady) {
          this.inProgress = false;
          return;
        }
        captcha = window.grecaptcha.getResponse(this.recaptchaId);
        if (captcha === "") {
          this.error = this.$t("login.recaptchaRequired");
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
          await authApi.signup(this.username, this.password);
        }
        await authApi.login(this.username, this.password, captcha);
        await initAuth();
        void router.push({ path: redirect });
      } catch (e) {
        console.log(e);
        this.inProgress = false;
        if (e.message.includes("OTP authentication is enforced")) {
          mutations.showPrompt({
            name: "totp",
            pinned: true,
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
            },
          });
        }
        if (e.message.includes("OTP is enforced, but user is not configured")) {
          mutations.showPrompt({
            name: "totp",
            pinned: true,
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
              generate: true,
            },
          });
        } else if (e.message.includes("OTP code is required for user")) {
          mutations.showPrompt({
            name: "totp",
            pinned: true,
            props: {
              username: this.username,
              password: this.password,
              recaptcha: captcha,
              redirect: redirect,
              generate: false,
            },
          });
        } else if (e.message.includes("passkey MFA is required")) {
          try {
            await authApi.beginPasskeyLogin(this.username, this.password);
            await initAuth();
            void router.push({ path: redirect });
          } catch (passkeyErr) {
            this.error = passkeyErr.message;
            this.inProgress = false;
          }
        } else if (e.message === 409) {
          this.error = this.$t("login.usernameTaken");
        } else if (e.message === 401) {
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
  padding: 0;
  padding-top: 0.5em;
  display: flex;
  place-content: center center;
  align-items: center;
}

.brand-text {
  padding: 1em;
  padding-top: 0.9em;
  color: var(--textPrimary);
}

.login-brand i {
  font-size: 5em;
  padding-top: 0;
  padding-bottom: 0;
}

.login-icon {
  width: 5em;
  height: 5em;
  object-fit: contain;
}

.password-entry {
  padding: 0;
  width: 100%;
}

.direct-login {
  display: flex;
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

.password-entry .wrong-login {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: .5em;
  background: var(--red);
  color: #fff;
  text-align: center;
  animation: .2s opac forwards;
  margin-bottom: 0.5em;
}

.login-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100px;
}

.loginOptions {
  text-align: center;
  display: flex;
  place-content: center center;
  align-items: center;
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

#login {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: var(--background);
}

#login h1 {
  text-align: center;
  font-size: 2.5em;
  margin: .4em 0 .67em;
}

#login form {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  max-width: 16em;
  width: 90%;
}

#login.recaptcha form {
  min-width: 304px;
}

#login #recaptcha {
  margin: .5em 0 0;
}

@keyframes opac {
  0% {
    opacity: 0;
  }
  100% {
    opacity: 1;
  }
}

#login p {
  cursor: pointer;
  text-align: right;
  color: var(--primaryColor);
  text-transform: lowercase;
  font-weight: 500;
  font-size: 0.9rem;
  margin: .5rem 0;
}

</style>
