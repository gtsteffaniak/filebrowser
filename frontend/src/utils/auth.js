import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getCookie } from "@/utils/cookie.js";
import { getApiPath } from "@/utils/url.js";

/** Session JWT: renew when within 30 minutes of expiry (former X-Renew-Token threshold). */
export const SESSION_REFRESH_BEFORE_MS = 30 * 60 * 1000;

/** View grants: refresh when within 10 minutes of expiry. */
export const VIEW_REFRESH_BEFORE_MS = 10 * 60 * 1000;

export const SESSION_COOKIE_NAME = "filebrowser_quantum_jwt";
const KEEP_ALIVE_INTERVAL_MS = 60 * 1000;

let keepAliveTimer = null;
let renewInFlight = null;

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

function decodeBase64UrlJson(segment) {
  const padded = segment.replace(/-/g, "+").replace(/_/g, "/");
  const padLength = (4 - (padded.length % 4)) % 4;
  const base64 = padded + "=".repeat(padLength);
  return JSON.parse(atob(base64));
}

/** Unix `exp` from the session JWT cookie, or null if missing/unreadable. */
export function getSessionJwtExpiresAt() {
  const raw = getCookie(SESSION_COOKIE_NAME);
  if (!raw) {
    return null;
  }
  // Cookie value may be URI-encoded depending on how it was set.
  let token = raw;
  try {
    token = decodeURIComponent(raw);
  } catch {
    // use raw
  }
  const parts = token.split(".");
  if (parts.length < 2) {
    return null;
  }
  try {
    const payload = decodeBase64UrlJson(parts[1]);
    return typeof payload.exp === "number" ? payload.exp : null;
  } catch {
    return null;
  }
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
    // A 401 from the non-public self check means our session cookie (JWT) is no
    // longer valid — typically it expired. Clear it and redirect to login so the
    // user can re-authenticate, instead of leaving the stale cookie in place
    // (which otherwise leaves the app stuck on reload). Only do this for a real,
    // non-public session: public routes legitimately 401 for anonymous share
    // visitors, and we skip it when there is no session cookie to clear.
    if (
      res.status === 401 &&
      !isPublicRoute &&
      document.cookie
        .split(";")
        .some((c) => c.trim().startsWith(`${SESSION_COOKIE_NAME}=`))
    ) {
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

/**
 * Renew the session JWT when it is missing or within the refresh window.
 * Concurrent callers share one in-flight renew.
 */
export async function ensureSessionFresh(withinMs = SESSION_REFRESH_BEFORE_MS) {
  if (getters.isShare?.() && !getters.isLoggedIn?.()) {
    return false;
  }
  const exp = getSessionJwtExpiresAt();
  if (!shouldRefreshBeforeExpiry(exp, withinMs)) {
    return false;
  }
  if (renewInFlight) {
    await renewInFlight;
    return true;
  }
  renewInFlight = renew()
    .catch((err) => {
      console.warn("session keep-alive renew failed:", err);
      throw err;
    })
    .finally(() => {
      renewInFlight = null;
    });
  try {
    await renewInFlight;
    return true;
  } catch {
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
      // Backend clears the cookie, but frontend does it as fail-safe cleanup
      document.cookie = `${SESSION_COOKIE_NAME}=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/`;
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

// Handle an authenticated request that came back 401 because the session cookie
// (JWT) is no longer valid — typically it expired while the tab was idle. Unlike
// logout(), this does NOT call the server (which would itself 401 on an expired
// token and leave the stale cookie in place); it clears the cookie locally and
// redirects to the login page so the user can re-authenticate, instead of being
// stuck on a raw "token is expired" error.
export function sessionExpired() {
  stopSessionKeepAlive();
  document.cookie =
    `${SESSION_COOKIE_NAME}=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/`;
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
