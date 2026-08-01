import { getters, state } from "@/store";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";

const REFRESH_BEFORE_MS = 10 * 60 * 1000;

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

function cacheKey(source) {
  const scope = viewGrantScope(source);
  return scope ? `viewToken:${scope}` : "";
}

function readCache(source) {
  const key = cacheKey(source);
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

function writeCache(source, viewToken, expiresAt) {
  const key = cacheKey(source);
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

export function getCachedViewToken(source) {
  const cached = readCache(source);
  if (!cached) {
    return undefined;
  }
  if (cached.expiresAt * 1000 <= Date.now()) {
    try {
      sessionStorage.removeItem(cacheKey(source));
    } catch {
      // ignore storage failures
    }
    return undefined;
  }
  return cached.viewToken;
}

export function rememberViewToken(source, viewToken, expiresAt) {
  const scope = viewGrantScope(source);
  if (!scope || !viewToken || !expiresAt) {
    return;
  }
  writeCache(scope, viewToken, expiresAt);
}

export async function refreshViewToken(source, existingToken) {
  const url = viewTokenApiPath(source, existingToken);
  const res = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: {
      sessionId: state.sessionId,
    },
  });
  if (!res.ok) {
    throw new Error(await res.text());
  }
  const data = await res.json();
  rememberViewToken(source, data.viewToken, data.expiresAt);
  return data;
}

export async function ensureViewToken(source) {
  const cached = readCache(source);
  const now = Date.now();
  if (
    cached?.viewToken &&
    cached.expiresAt * 1000 - now > REFRESH_BEFORE_MS
  ) {
    return cached.viewToken;
  }
  const data = await refreshViewToken(source, cached?.viewToken);
  return data.viewToken;
}

export async function refreshCachedViewTokensIfNeeded() {
  const now = Date.now();
  const scopes = new Set();
  try {
    for (let i = 0; i < sessionStorage.length; i++) {
      const key = sessionStorage.key(i);
      if (!key?.startsWith("viewToken:")) {
        continue;
      }
      try {
        const parsed = JSON.parse(sessionStorage.getItem(key));
        if (!parsed?.expiresAt || !parsed?.viewToken) {
          continue;
        }
        const msLeft = parsed.expiresAt * 1000 - now;
        if (msLeft > REFRESH_BEFORE_MS) {
          continue;
        }
        const scope = key.slice("viewToken:".length);
        if (scope) {
          scopes.add(scope);
        }
      } catch {
        // ignore malformed cache entries
      }
    }
  } catch {
    return;
  }
  await Promise.all(
    [...scopes].map((scope) =>
      refreshViewToken(scope, getCachedViewToken(scope)).catch(() => {}),
    ),
  );
}
