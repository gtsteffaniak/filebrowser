import cssVars from "css-vars-ponyfill";
import router from "@/router";
import i18n from "@/i18n";
import Vue from "@/utils/vue";
import { recaptcha, loginPage } from "@/utils/constants";
import { login, validateLogin } from "@/utils/auth";
import App from "@/App";
import { state } from "@/store"; // Import state from state.js
export const eventBus = new Vue(); // Creating an event bus.
cssVars();

async function start() {
  console.log("state",state)
  try {
    if (loginPage) {
      await validateLogin();
    } else {
      await login("publicUser", "publicUser", "");
    }
  } catch (e) {
    console.log(e);
  }
  if (recaptcha) {
    await new Promise((resolve) => {
      const check = () => {
        if (typeof window.grecaptcha === "undefined") {
          setTimeout(check, 100);
        } else {
          resolve();
        }
      };
      check();
    });
  }

  new Vue({
    el: "#app",
    router,
    i18n,
    data: state,
    template: "<App/>",
    components: { App },
  });
}

start();
