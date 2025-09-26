import { mutations, getters,state } from "@/store";
import { usersApi } from "@/api";
import { getApiPath } from "@/utils/url.js";
import { globalVars } from "@/utils/constants";

export async function setNewToken(token) {
  document.cookie = `auth=${token}; path=/`;
  mutations.setJWT(token);
}

export async function validateLogin() {
  let userInfo = await usersApi.get("self");
  mutations.setCurrentUser(userInfo);
  getters.isLoggedIn()
  if (state.user.loginMethod == "proxy") {
    let apiPath = getApiPath("api/auth/login")
    const res = await fetch(apiPath, {
      method: "POST",
    });
    const body = await res.text();
    if (res.status === 200) {
      await setNewToken(body);
    } else {
      throw new Error(body);
    }
  }
  return
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

export function generateRandomCode(length) {
  const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let code = '';
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length);
    code += charset[randomIndex];
  }

  return code;
}

export async function logout() {
  try {
    const res = await fetch(getApiPath("api/auth/logout"), { method: "POST" });
    if (res.ok) {
      const data = await res.json();
      let logoutUrl = data.logoutUrl;
      document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
      mutations.setCurrentUser(null);
      mutations.setJWT("");
      if (!logoutUrl) {
        logoutUrl = globalVars.baseURL+"login";
      }
      // Add a small delay to ensure cookie deletion completes before redirect
      setTimeout(() => {
        window.location.href = logoutUrl;
      }, 100);
      return; // Stop execution
    } else {
      // Handle potential errors from the API, e.g., res.status 401, 500
      console.error("Logout API call failed:", res.status, res.statusText);
    }
  } catch (e) {
    console.error("An error occurred during logout:", e);
  }
}

// Helper function to retrieve the value of a specific cookie
//function getCookie(name) {
//  return document.cookie
//    .split('; ')
//    .find(row => row.startsWith(name + '='))
//    ?.split('=')[1];
//}

export async function initAuth() {
  if (!getters.isShare()) {
    console.log("validating login");
    await validateLogin();
  }
  if (globalVars.recaptcha) {
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