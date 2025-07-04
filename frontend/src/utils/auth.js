import { mutations, getters,state } from "@/store";
import router from "@/router";
import { usersApi } from "@/api";
import { getApiPath } from "@/utils/url.js";
import { recaptcha, loginPage } from "@/utils/constants";

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
  if (state.user.loginMethod === "oidc" || state.user.loginMethod === "proxy") {
    try {
      const res = await fetch(getApiPath("api/auth/logout"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json", // Good practice
          "Authorization": `Bearer ${state.jwt}`,
        },
      });

      // --- CHANGE IS HERE ---
      // Check if the request was successful and handle the JSON response
      if (res.ok) {
        const data = await res.json(); // Parse the JSON body
        const logoutUrl = data.logoutUrl;

        if (logoutUrl) {
          // Clean up local state *before* navigating away
          document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
          mutations.setCurrentUser(null);
          mutations.setJWT("");
          // Redirect the browser window
          window.location.href = logoutUrl;
          return; // Stop execution
        }
      } else {
        // Handle potential errors from the API, e.g., res.status 401, 500
        console.error("Logout API call failed:", res.status, res.statusText);
      }

    } catch (e) {
      console.error("An error occurred during logout:", e);
    }
  }

  // Fallback for non-oidc/proxy users or if the above logic fails
  document.cookie = "auth=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
  mutations.setCurrentUser(null);
  mutations.setJWT("");
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