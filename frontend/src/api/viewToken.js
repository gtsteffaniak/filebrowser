import { getters, state } from "@/store";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";

const REFRESH_BEFORE_MS = 10 * 60 * 1000;

function cacheKey(source) {
  if (getters.isShare()) {
    const hash = state.shareInfo?.hash || "";
    return `viewToken:${hash}:${source}`;
  }
  return `viewToken:${source}`;
}

function readCache(source) {
  try {
    const raw = sessionStorage.getItem(cacheKey(source));
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
  sessionStorage.setItem(
    cacheKey(source),
    JSON.stringify({ viewToken, expiresAt }),
  );
}

function viewTokenApiPath(source, existingToken) {
  const params = { source };
  if (existingToken) {
    params.viewToken = existingToken;
  }
  if (getters.isShare()) {
    params.hash = state.shareInfo?.hash;
    params.token = state.shareInfo?.token;
    return getPublicApiPath("resources/view-token", params);
  }
  return getApiPath("resources/view-token", params);
}

export function getCachedViewToken(source) {
  const cached = readCache(source);
  if (!cached) {
    return undefined;
  }
  if (cached.expiresAt * 1000 <= Date.now()) {
    sessionStorage.removeItem(cacheKey(source));
    return undefined;
  }
  return cached.viewToken;
}

export function rememberViewToken(source, viewToken, expiresAt) {
  if (!source || !viewToken || !expiresAt) {
    return;
  }
  writeCache(source, viewToken, expiresAt);
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
  const sources = new Set();
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
      if (msLeft <= 0 || msLeft > REFRESH_BEFORE_MS) {
        continue;
      }
      const source = key.includes(":") ? key.split(":").pop() : "";
      if (source) {
        sources.add(source);
      }
    } catch {
      // ignore malformed cache entries
    }
  }
  await Promise.all(
    [...sources].map((source) =>
      refreshViewToken(source, getCachedViewToken(source)).catch(() => {}),
    ),
  );
}
