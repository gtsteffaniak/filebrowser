import { mutations, getters, state } from "@/store";
import router from "@/router";
import { usersApi,filesApi } from "@/api";
import { getApiPath } from "@/utils/url.js";
import { recaptcha, loginPage, disableUsedPercentage } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";

export async function setNewToken(token) {
  document.cookie = `auth=${token}; path=/`;
  mutations.setJWT(token);
  mutations.setSession(generateRandomCode(8));
}

export async function validateLogin() {
  try {
    let userInfo = await usersApi.get("self");
    mutations.setCurrentUser(userInfo);
  } catch (error) {
    console.log("Error validating login");
  }
  if (!disableUsedPercentage) {
    for (const source of Object.keys(state.sources.info)) {
      let usage = await filesApi.usage(source);
      let sourceInfo = state.sources.info[source];
      sourceInfo.used = getHumanReadableFilesize(usage.used);
      sourceInfo.total = getHumanReadableFilesize(usage.total);
      sourceInfo.usedPercentage = Math.round((usage.used / usage.total) * 100);
      mutations.updateSource(source, sourceInfo);
    }
  }

  return getters.isLoggedIn()
}

export async function login(username, password, recaptcha) {
  const data = { username, password, recaptcha };
  try {
    let apiPath = getApiPath("api/auth/login")
    const res = await fetch(apiPath, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });
    const body = await res.text();

    if (res.status === 200) {
      await setNewToken(body);
    } else {
      throw new Error(body);
    }
  } catch (error) {
    throw new Error("Login failed");
  }
}

export async function renew(jwt) {
  let apiPath = getApiPath("api/auth/renew")
  const res = await fetch(apiPath, {
    method: "POST",
    headers: {
      "X-Auth": jwt,
    },
  });
  const body = await res.text();
  if (res.status === 200) {
    mutations.setSession(generateRandomCode(8));
    await setNewToken(body);
  } else {
    throw new Error(body);
  }
}

function generateRandomCode(length) {
  const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let code = '';
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length);
    code += charset[randomIndex];
  }

  return code;
}

export async function signupLogin(username, password) {
  const data = { username, password };
  let apiPath = getApiPath("api/auth/signup")
  const res = await fetch(apiPath, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (res.status !== 200) {
    throw new Error(res.status);
  }
}

export function logout() {
  document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
  mutations.setCurrentUser(null);
  router.push({ path: "/login" });
}

// Helper function to retrieve the value of a specific cookie
//function getCookie(name) {
//  return document.cookie
//    .split('; ')
//    .find(row => row.startsWith(name + '='))
//    ?.split('=')[1];
//}

export async function initAuth() {
  if (loginPage && !getters.isShare()) {
    console.log("validating login");
    await validateLogin();
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
}