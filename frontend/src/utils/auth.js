import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getApiPath } from "@/utils/url.js";
import { sanitizeLogoutDestination, sanitizePostLoginRedirect } from "@/utils/safeRedirect.js";

/** Session JWT: renew when within 30 minutes of expiry (former X-Renew-Token threshold). */
export const SESSION_REFRESH_BEFORE_MS = 30 * 60 * 1000;

/** View grants: refresh when within 10 minutes of expiry. */
export const VIEW_REFRESH_BEFORE_MS = 10 * 60 * 1000;

export const SESSION_COOKIE_NAME = "filebrowser_quantum_jwt";
const KEEP_ALIVE_INTERVAL_MS = 60 * 1000;

let keepAliveTimer = null;
let renewInFlight = null;
let sessionExpiresAt = null;

/*
 * ============================================================
 * AUTH DEBUG
 * ============================================================
 */

const AUTH_DEBUG_ID =
  `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;

function authDebug(event, data = {}) {
  try {
    console.debug(
      `[AUTH DEBUG][${AUTH_DEBUG_ID}] ${event}`,
      {
        time: new Date().toISOString(),
        page: window.location.pathname,
        visibility: document.visibilityState,
        sessionId: state.sessionId,
        loggedIn: getters.isLoggedIn?.(),
        ...data,
      }
    );
  } catch (e) {
    console.debug(`[AUTH DEBUG] ${event}`, data);
  }
}

function tokenDebug(token) {
  if (!token) {
    return {
      present: false,
    };
  }

  const exp = decodeJwtExp(token);

  return {
    present: true,
    length: token.length,
    prefix: token.slice(0, 12),
    hashHint: simpleTokenHint(token),
    exp,
    expDate: exp ? new Date(exp * 1000).toISOString() : null,
  };
}

function simpleTokenHint(token) {
  if (!token) {
    return null;
  }

  let hash = 0;

  for (let i = 0; i < token.length; i++) {
    hash = ((hash << 5) - hash + token.charCodeAt(i)) | 0;
  }

  return Math.abs(hash).toString(16);
}

/*
 * ============================================================
 * JWT
 * ============================================================
 */

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

    const payload = JSON.parse(
      atob(padded + "=".repeat(padLength))
    );

    return typeof payload.exp === "number"
      ? payload.exp
      : null;
  } catch {
    return null;
  }
}

function setSessionExpiresAtFromToken(token) {
  sessionExpiresAt = decodeJwtExp(token);

  authDebug("setSessionExpiresAtFromToken()", {
    token: tokenDebug(token),
    sessionExpiresAt,
    sessionExpiresAtDate: sessionExpiresAt
      ? new Date(sessionExpiresAt * 1000).toISOString()
      : null,
  });
}

/**
 * @param {number|null|undefined} expiresAtSeconds Unix expiry in seconds
 * @param {number} refreshBeforeMs Refresh this many ms before expiry
 * @returns {number} Milliseconds until a refresh should run (0 = refresh now)
 */
export function msUntilRefresh(expiresAtSeconds, refreshBeforeMs) {
  if (
    expiresAtSeconds === null ||
    expiresAtSeconds === undefined ||
    !Number.isFinite(expiresAtSeconds)
  ) {
    return 0;
  }

  return Math.max(
    0,
    expiresAtSeconds * 1000 -
      Date.now() -
      refreshBeforeMs
  );
}

/**
 * @param {number|null|undefined} expiresAtSeconds Unix expiry in seconds
 * @param {number} refreshBeforeMs Refresh this many ms before expiry
 * @returns {boolean} True when the token should be refreshed now
 */
export function shouldRefreshBeforeExpiry(
  expiresAtSeconds,
  refreshBeforeMs
) {
  return msUntilRefresh(expiresAtSeconds, refreshBeforeMs) === 0;
}

/** Unix `exp` tracked from the last successful renew response, or null if unknown. */
export function getSessionJwtExpiresAt() {
  return sessionExpiresAt;
}

/*
 * ============================================================
 * LOGIN VALIDATION
 * ============================================================
 */

export async function validateLogin(isPublicRoute = false) {

  authDebug("validateLogin START", {
    isPublicRoute,
    sessionExpiresAt,
    sessionExpiresAtDate: sessionExpiresAt
      ? new Date(sessionExpiresAt * 1000).toISOString()
      : null,
  });

  const apiPath = getApiPath(
    "users",
    { username: "self" },
    false,
    isPublicRoute
  );

  authDebug("validateLogin FETCH", {
    apiPath,
  });

  const res = await fetch(apiPath, {
    credentials: "same-origin",
    headers: {
      sessionId: state.sessionId,
    },
  });

  authDebug("validateLogin RESPONSE", {
    status: res.status,
    ok: res.ok,
  });

  if (res.status !== 200) {

    if (
      res.status === 401 &&
      !isPublicRoute &&
      getters.isLoggedIn()
    ) {
      authDebug("validateLogin -> sessionExpired()", {
        reason: "401",
      });

      sessionExpired();
    }

    throw new Error(
      `{"status":${res.status},"message":"${await res.text()}"}`
    );
  }

  const userInfo = await res.json();

  authDebug("validateLogin USER", {
    username: userInfo?.username,
    id: userInfo?.id,
    loginMethod: userInfo?.loginMethod,
  });

  await mutations.setCurrentUser(userInfo);
  await mutations.syncEnforcedUserDefaults();
  await mutations.syncSidebarLinkDefaultsPolicy();
  await mutations.syncToolAccessDefaultsPolicy();
  await mutations.syncShareDefaultsPolicy();

  getters.isLoggedIn();

  if (
    state.user.loginMethod === "proxy" &&
    !isPublicRoute
  ) {

    authDebug("validateLogin PROXY LOGIN");

    const apiPath = getApiPath("auth/login");

    const res = await fetch(apiPath, {
      method: "POST",
      credentials: "same-origin",
    });

    const body = await res.text();

    authDebug("validateLogin PROXY RESPONSE", {
      status: res.status,
      ok: res.ok,
    });

    if (res.status !== 200) {
      throw new Error(body);
    }
  }

  if (!isPublicRoute) {
    authDebug("validateLogin -> startSessionKeepAlive()");

    startSessionKeepAlive();
  }

  authDebug("validateLogin END");
}

/*
 * ============================================================
 * RENEW
 * ============================================================
 */

/**
 * Cookie-based session renewal.
 */
export async function renew() {

  authDebug("renew() CALLED", {
    renewInFlight: !!renewInFlight,
    sessionExpiresAt,
    sessionExpiresAtDate: sessionExpiresAt
      ? new Date(sessionExpiresAt * 1000).toISOString()
      : null,
  });

  if (renewInFlight) {

    authDebug("renew() JOIN EXISTING REQUEST");

    return renewInFlight;
  }

  renewInFlight = (async () => {

    const apiPath = getApiPath("auth/renew");

    authDebug("renew() FETCH START", {
      apiPath,
    });

    const started = Date.now();

    const res = await fetch(apiPath, {
      method: "POST",
      credentials: "same-origin",
    });

    const body = await res.text();

    authDebug("renew() RESPONSE", {
      status: res.status,
      ok: res.ok,
      durationMs: Date.now() - started,
      bodyToken: tokenDebug(body),
    });

    if (res.status === 200) {

      setSessionExpiresAtFromToken(body);

      const oldSessionId = state.sessionId;

      mutations.setSession(generateRandomCode(8));

      authDebug("renew() SUCCESS", {
        oldSessionId,
        newSessionId: state.sessionId,
        token: tokenDebug(body),
      });

      return;
    }

    authDebug("renew() FAILED", {
      status: res.status,
      body,
    });

    throw new Error(body);

  })().finally(() => {

    authDebug("renew() FINALLY");

    renewInFlight = null;
  });

  return renewInFlight;
}

/*
 * ============================================================
 * SESSION FRESHNESS
 * ============================================================
 */

export async function ensureSessionFresh(
  withinMs = SESSION_REFRESH_BEFORE_MS
) {

  authDebug("ensureSessionFresh() CHECK", {
    withinMs,
    sessionExpiresAt,
    sessionExpiresAtDate: sessionExpiresAt
      ? new Date(sessionExpiresAt * 1000).toISOString()
      : null,
    refreshInMs: msUntilRefresh(
      sessionExpiresAt,
      withinMs
    ),
  });

  if (
    getters.isShare?.() &&
    !getters.isLoggedIn?.()
  ) {

    authDebug("ensureSessionFresh() SKIP SHARE");

    return false;
  }

  const exp = getSessionJwtExpiresAt();

  if (!shouldRefreshBeforeExpiry(exp, withinMs)) {

    authDebug("ensureSessionFresh() NO RENEW", {
      exp,
      expDate: exp
        ? new Date(exp * 1000).toISOString()
        : null,
    });

    return false;
  }

  authDebug("ensureSessionFresh() -> renew()");

  try {

    await renew();

    authDebug("ensureSessionFresh() RENEW SUCCESS");

    return true;

  } catch (err) {

    console.warn(
      "[AUTH DEBUG] session keep-alive renew failed:",
      err
    );

    authDebug("ensureSessionFresh() RENEW FAILED", {
      error: String(err),
    });

    return false;
  }
}

/*
 * ============================================================
 * KEEP ALIVE
 * ============================================================
 */

export function startSessionKeepAlive() {

  authDebug("startSessionKeepAlive() CALLED", {
    alreadyRunning: keepAliveTimer !== null,
  });

  if (keepAliveTimer !== null) {

    authDebug(
      "startSessionKeepAlive() ALREADY RUNNING"
    );

    return;
  }

  authDebug(
    "startSessionKeepAlive() STARTING TIMER"
  );

  void ensureSessionFresh();

  keepAliveTimer = setInterval(() => {

    authDebug(
      "KEEP-ALIVE TIMER TICK"
    );

    void ensureSessionFresh();

  }, KEEP_ALIVE_INTERVAL_MS);
}

export function stopSessionKeepAlive() {

  authDebug("stopSessionKeepAlive()", {
    wasRunning: keepAliveTimer !== null,
  });

  if (keepAliveTimer !== null) {

    clearInterval(keepAliveTimer);

    keepAliveTimer = null;
  }
}

/*
 * ============================================================
 * SESSION ID
 * ============================================================
 */

export function generateRandomCode(length) {

  const charset =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

  let code = "";

  for (let i = 0; i < length; i++) {

    const randomIndex =
      Math.floor(Math.random() * charset.length);

    code += charset.charAt(randomIndex);
  }

  return code;
}

/*
 * ============================================================
 * LOGOUT
 * ============================================================
 */

export async function logout(redirectUrl) {

  authDebug("logout() START");

  stopSessionKeepAlive();

  try {

    const res = await fetch(
      getApiPath("auth/logout"),
      {
        method: "POST",
        credentials: "same-origin",
      }
    );

    authDebug("logout() RESPONSE", {
      status: res.status,
      ok: res.ok,
    });

    if (res.ok) {

      const data = await res.json();

      let destination =
        data.logoutUrl ||
        `${globalVars.baseURL}login`;

      if (redirectUrl) {
        destination =
          sanitizeLogoutDestination(
            redirectUrl,
            destination
          );
      }

      sessionExpiresAt = null;

      void mutations.setCurrentUser(null);

      setTimeout(() => {
        window.location.href = destination;
      }, 100);

      return;
    }

    console.error(
      "Logout API call failed:",
      res.status,
      res.statusText
    );

  } catch (e) {

    console.error(
      "An error occurred during logout:",
      e
    );
  }
}

/*
 * ============================================================
 * SESSION EXPIRED
 * ============================================================
 */

export function sessionExpired() {

  authDebug("sessionExpired()");

  stopSessionKeepAlive();

  sessionExpiresAt = null;

  void mutations.setCurrentUser(null);

  if (
    window.location.pathname.endsWith("/login")
  ) {
    return;
  }

  const current =
    window.location.pathname +
    window.location.search;

  const safeRedirect =
    sanitizePostLoginRedirect(
      current,
      "/files/"
    );

  window.location.href =
    `${globalVars.baseURL}login?redirect=${encodeURIComponent(
      safeRedirect
    )}`;
}

/*
 * ============================================================
 * INIT
 * ============================================================
 */

export async function initAuth() {

  authDebug("initAuth() START", {
    isShare: getters.isShare?.(),
    sessionExpiresAt,
  });

  if (!getters.isShare()) {

    authDebug(
      "initAuth() -> validateLogin()"
    );

    await validateLogin();
  }

  if (globalVars.recaptcha) {

    await new Promise((resolve) => {

      const check = () => {

        if (
          typeof window.grecaptcha ===
          "undefined"
        ) {

          setTimeout(check, 100);

        } else {

          resolve();
        }
      };

      check();
    });
  }

  authDebug("initAuth() END");
}
