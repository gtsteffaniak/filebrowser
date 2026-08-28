import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getApiPath } from "@/utils/url.js";

export async function validateLogin(isPublicRoute = false) {
  // Use direct fetch to avoid automatic logout on 401
  // Public routes (e.g. /public/share/...) use the public API base path
  const apiPath = getApiPath('users', { id: 'self' }, false, isPublicRoute);
  const res = await fetch(apiPath, {
    credentials: 'same-origin', // Ensure cookies are sent with the request
    headers: {
      "sessionId": state.sessionId,
    }
  });

  if (res.status !== 200) {
    // A 401 from the non-public self check means our session is no longer valid —
    // typically the HttpOnly JWT cookie expired. Redirect to login when the app
    // still considers the user logged in (public routes legitimately 401 for
    // anonymous share visitors).
    if (res.status === 401 && !isPublicRoute && getters.isLoggedIn()) {
      sessionExpired();
    }
    throw new Error(`{"status":${res.status},"message":"${await res.text()}"}`);
  }
  const userInfo = await res.json();
  await mutations.setCurrentUser(userInfo);
  getters.isLoggedIn()
  if (state.user.loginMethod === "proxy") {
    const apiPath = getApiPath("auth/login")
    const res = await fetch(apiPath, {
      method: "POST",
      credentials: 'same-origin', // Ensure cookies are sent and can be set
    });
    const body = await res.text();
    if (res.status !== 200) {
      throw new Error(body);
    }
  }
  return
}

export async function renew() {
  // Cookie-based renewal - no JWT parameter needed
  // Backend reads cookie, validates, and sets new cookie
  const apiPath = getApiPath("auth/renew")
  const res = await fetch(apiPath, {
    method: "POST",
    credentials: 'same-origin', // Cookie is sent automatically, backend renews it
  });
  const body = await res.text();
  if (res.status === 200) {
    mutations.setSession(generateRandomCode(8));
    // Backend sets the new cookie, no state management needed
  } else {
    throw new Error(body);
  }
}

export function generateRandomCode(length) {
  const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let code = '';
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length);
    code += charset.charAt(randomIndex);
  }

  return code;
}

export async function logout(redirectUrl) {
  try {
    const res = await fetch(getApiPath("auth/logout"), {
      method: "POST",
      credentials: 'same-origin'
    });
    if (res.ok) {
      const data = await res.json();
      let destination = data.logoutUrl || `${globalVars.baseURL}login`;
      if (redirectUrl) {
        destination = redirectUrl;
      }
      // Backend clears the HttpOnly session cookie.
      void mutations.setCurrentUser(null);
      // No need to clear state.jwt - cookie is the source of truth
      // Add a small delay to ensure cookie deletion completes before redirect
      setTimeout(() => {
        window.location.href = destination;
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

// Handle an authenticated request that came back 401 because the session is no
// longer valid — typically it expired while the tab was idle. Unlike logout(),
// this does NOT call the server; it clears client state and redirects to login.
export function sessionExpired() {
  void mutations.setCurrentUser(null);
  // Avoid a redirect loop if we're already on the login page.
  if (window.location.pathname.endsWith("/login")) {
    return;
  }
  const current = window.location.pathname + window.location.search;
  window.location.href = `${globalVars.baseURL}login?redirect=${encodeURIComponent(current)}`;
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
