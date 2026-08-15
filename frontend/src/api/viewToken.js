import { getters, state } from "@/store";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import {
  VIEW_REFRESH_BEFORE_MS,
  msUntilRefresh,
  shouldRefreshBeforeExpiry,
} from "@/utils/auth";

/** @deprecated use VIEW_REFRESH_BEFORE_MS — kept as alias for clarity in this module */
const REFRESH_BEFORE_MS = VIEW_REFRESH_BEFORE_MS;

let activeScope = "";
let refreshTimer = null;

/** Stable key for the current preview request (source, share, path). */
export function requestViewIdentity(req) {
  if (!req) {
    return "";
  }
  const shareHash = getters.isShare() ? (state.shareInfo?.hash ?? "") : "";
  return JSON.stringify([req.source ?? "", shareHash, req.path ?? ""]);
}

/** View token attached to the current request object, if any. */
export function getRequestViewToken(req) {
  return req?.viewToken;
}

function viewGrantScope(source) {
  if (getters.isShare()) {
    return state.shareInfo?.hash || "";
  }
  return source || "";
}

function cacheKeyForScope(scope) {
  return scope ? `viewToken:${scope}` : "";
}

function readCacheForScope(scope) {
  const key = cacheKeyForScope(scope);
  if (!key) {
    return null;
  }
  try {
    const raw = sessionStorage.getItem(key);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw);
    if (!parsed?.viewToken || !parsed?.expiresAt) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function writeCacheForScope(scope, viewToken, expiresAt) {
  const key = cacheKeyForScope(scope);
  if (!key) {
    return;
  }
  try {
    sessionStorage.setItem(
      key,
      JSON.stringify({ viewToken, expiresAt }),
    );
  } catch {
    // QuotaExceededError / SecurityError — caching is best-effort only.
  }
}

function clearCachedScopeByScope(scope) {
  try {
    sessionStorage.removeItem(cacheKeyForScope(scope));
  } catch {
    // ignore storage failures
  }
}

function viewTokenApiPath(source, existingToken) {
  const params = {};
  if (existingToken) {
    params.viewToken = existingToken;
  }
  if (getters.isShare()) {
    params.hash = state.shareInfo?.hash;
    params.token = state.shareInfo?.token;
    return getPublicApiPath("resources/view-token", params);
  }
  params.source = source;
  return getApiPath("resources/view-token", params);
}

function scheduleViewGrantRefresh(source, scope) {
  if (!scope || scope !== activeScope) {
    return;
  }
  const cached = readCacheForScope(scope);
  if (!cached?.expiresAt) {
    return;
  }
  // Same pattern as session keep-alive: refresh before expiry, not on request headers.
  const delay = msUntilRefresh(cached.expiresAt, REFRESH_BEFORE_MS);
  if (refreshTimer) {
    clearTimeout(refreshTimer);
  }
  refreshTimer = setTimeout(() => {
    refreshTimer = null;
    if (scope !== activeScope) {
      return;
    }
    refreshViewToken(source, getCachedViewToken(source), scope).catch(() => {});
  }, delay);
}

export function setActiveViewGrantScope(source) {
  const scope = viewGrantScope(source);
  if (refreshTimer) {
    clearTimeout(refreshTimer);
    refreshTimer = null;
  }
  activeScope = scope;
  if (scope) {
    scheduleViewGrantRefresh(source, scope);
  }
}

export function getCachedViewToken(source) {
  const scope = viewGrantScope(source);
  const cached = readCacheForScope(scope);
  if (!cached) {
    return undefined;
  }
  if (cached.expiresAt * 1000 <= Date.now()) {
    clearCachedScopeByScope(scope);
    return undefined;
  }
  return cached.viewToken;
}

export function rememberViewToken(source, viewToken, expiresAt) {
  const scope = viewGrantScope(source);
  if (!scope || !viewToken || !expiresAt) {
    return;
  }
  writeCacheForScope(scope, viewToken, expiresAt);
  if (scope === activeScope) {
    scheduleViewGrantRefresh(source, scope);
  }
}

export async function refreshViewToken(source, existingToken, requestScope = null) {
  const scope = requestScope ?? viewGrantScope(source);
  const url = viewTokenApiPath(source, existingToken);
  const res = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: {
      sessionId: state.sessionId,
    },
  });
  if (!res.ok) {
    if (res.status === 403 || res.status === 401) {
      clearCachedScopeByScope(scope);
    }
    throw new Error(await res.text());
  }
  const data = await res.json();
  if (scope !== activeScope) {
    return data;
  }
  writeCacheForScope(scope, data.viewToken, data.expiresAt);
  scheduleViewGrantRefresh(source, scope);
  return data;
}

export async function ensureViewToken(source) {
  const scope = viewGrantScope(source);
  const cached = readCacheForScope(scope);
  if (
    cached?.viewToken &&
    !shouldRefreshBeforeExpiry(cached.expiresAt, REFRESH_BEFORE_MS)
  ) {
    return cached.viewToken;
  }
  const data = await refreshViewToken(source, cached?.viewToken, scope);
  return data.viewToken;
}
