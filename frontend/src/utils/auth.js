import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getApiPath } from "@/utils/url.js";

/** Session JWT: renew when within 30 minutes of expiry (former X-Renew-Token threshold). */
export const SESSION_REFRESH_BEFORE_MS = 30 * 60 * 1000;

/** View grants: refresh when within 10 minutes of expiry. */
export const VIEW_REFRESH_BEFORE_MS = 10 * 60 * 1000;

export const SESSION_COOKIE_NAME = "filebrowser_quantum_jwt";
const KEEP_ALIVE_INTERVAL_MS = 60 * 1000;

let keepAliveTimer = null;
let renewInFlight = null;
let sessionExpiresAt = null;

function decodeJwtExp(token) {
  if (!token) {
    return null;
  }
  const parts = token.split(".");
  if (parts.length < 2) {
    return null;
  }
  try {
    const padded = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padLength = (4 - (padded.length % 4)) % 4;
    const payload = JSON.parse(atob(padded + "=".repeat(padLength)));
    return typeof payload.exp === "number" ? payload.exp : null;
  } catch {
    return null;
  }
}

function setSessionExpiresAtFromToken(token) {
  sessionExpiresAt = decodeJwtExp(token);
}

/**
 * @param {number|null|undefined} expiresAtSeconds Unix expiry in seconds
 * @param {number} refreshBeforeMs Refresh this many ms before expiry
 * @returns {number} Milliseconds until a refresh should run (0 = refresh now)
 */
export function msUntilRefresh(expiresAtSeconds, refreshBeforeMs) {
  if (expiresAtSeconds === null || expiresAtSeconds === undefined || !Number.isFinite(expiresAtSeconds)) {
    return 0;
  }
  return Math.max(0, expiresAtSeconds * 1000 - Date.now() - refreshBeforeMs);
}

/**
 * @param {number|null|undefined} expiresAtSeconds Unix expiry in seconds
 * @param {number} refreshBeforeMs Refresh this many ms before expiry
 * @returns {boolean} True when the token should be refreshed now
 */
export function shouldRefreshBeforeExpiry(expiresAtSeconds, refreshBeforeMs) {
  return msUntilRefresh(expiresAtSeconds, refreshBeforeMs) === 0;
}

/** Unix `exp` tracked from the last successful renew response, or null if unknown. */
export function getSessionJwtExpiresAt() {
  return sessionExpiresAt;
}

export async function validateLogin(isPublicRoute = false) {
  // Use direct fetch to avoid automatic logout on 401
  // Public routes (e.g. /public/share/...) use the public API base path
  const apiPath = getApiPath('users', { username: 'self' }, false, isPublicRoute);
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
  await mutations.syncEnforcedUserDefaults();
  getters.isLoggedIn()
  // Public share/static routes use the public API; proxy session cookie login is protected-only.
  if (state.user.loginMethod === "proxy" && !isPublicRoute) {
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
  if (!isPublicRoute) {
    startSessionKeepAlive();
  }
  return
}

/**
 * Cookie-based session renewal. Concurrent callers share one in-flight request
 * so mid-upload 401 retries and keep-alive do not stampede /auth/renew.
 */
export async function renew() {
  if (renewInFlight) {
    return renewInFlight;
  }
  renewInFlight = (async () => {
    // Backend reads cookie, validates, and sets new cookie
    const apiPath = getApiPath("auth/renew");
    const res = await fetch(apiPath, {
      method: "POST",
      credentials: "same-origin",
    });
    const body = await res.text();
    if (res.status === 200) {
      setSessionExpiresAtFromToken(body);
      mutations.setSession(generateRandomCode(8));
      return;
    }
    throw new Error(body);
  })().finally(() => {
    renewInFlight = null;
  });
  return renewInFlight;
}

/**
 * Renew the session JWT when it is missing or within the refresh window.
 * Concurrent callers share one in-flight renew; failures return false.
 */
export async function ensureSessionFresh(withinMs = SESSION_REFRESH_BEFORE_MS) {
  if (getters.isShare?.() && !getters.isLoggedIn?.()) {
    return false;
  }
  const exp = getSessionJwtExpiresAt();
  if (!shouldRefreshBeforeExpiry(exp, withinMs)) {
    return false;
  }
  try {
    await renew();
    return true;
  } catch (err) {
    console.warn("session keep-alive renew failed:", err);
    return false;
  }
}

export function startSessionKeepAlive() {
  if (keepAliveTimer !== null) {
    return;
  }
  void ensureSessionFresh();
  keepAliveTimer = setInterval(() => {
    void ensureSessionFresh();
  }, KEEP_ALIVE_INTERVAL_MS);
}

export function stopSessionKeepAlive() {
  if (keepAliveTimer !== null) {
    clearInterval(keepAliveTimer);
    keepAliveTimer = null;
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
  stopSessionKeepAlive();
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
      sessionExpiresAt = null;
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
  stopSessionKeepAlive();
  sessionExpiresAt = null;
  void mutations.setCurrentUser(null);
  // Avoid a redirect loop if we're already on the login page.
  if (window.location.pathname.endsWith("/login")) {
    return;
  }
  const current = window.location.pathname + window.location.search;
  window.location.href = `${globalVars.baseURL}login?redirect=${encodeURIComponent(current)}`;
}

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
