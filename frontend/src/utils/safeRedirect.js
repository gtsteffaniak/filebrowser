/**
 * @param {string} str
 * @returns {boolean}
 */
function hasUnsafeControlChars(str) {
  for (let i = 0; i < str.length; i++) {
    const code = str.charCodeAt(i);
    if (code === 0 || code === 10 || code === 13) {
      return true;
    }
  }
  return false;
}

/**
 * @param {unknown} redirect
 * @param {string} defaultPath
 * @returns {string}
 */
export function sanitizePostLoginRedirect(redirect, defaultPath = '/files/') {
  if (redirect === '' || redirect === undefined || redirect === null) {
    return defaultPath;
  }
  const path = String(redirect).trim();
  if (path.includes('://')) {
    return defaultPath;
  }
  if (!path.startsWith('/')) {
    return defaultPath;
  }
  if (path.startsWith('//') || path.startsWith('/\\')) {
    return defaultPath;
  }
  if (hasUnsafeControlChars(path)) {
    return defaultPath;
  }
  return path;
}

/**
 * @param {unknown} destination
 * @param {string} fallback
 * @returns {string}
 */
export function sanitizeLogoutDestination(destination, fallback) {
  if (destination === '' || destination === undefined || destination === null) {
    return fallback;
  }
  const trimmed = String(destination).trim();
  if (trimmed.startsWith('/') && !trimmed.startsWith('//') && !trimmed.startsWith('/\\')) {
    if (!hasUnsafeControlChars(trimmed)) {
      return new URL(trimmed, window.location.origin).href;
    }
  }
  try {
    const resolved = new URL(trimmed, window.location.origin);
    if (resolved.origin === window.location.origin) {
      return resolved.href;
    }
  } catch {
    // fall through
  }
  return fallback;
}

/**
 * Absolute path for server-driven redirects (e.g. OIDC), including Http baseURL.
 * @param {string} safePath
 * @param {string} baseURL
 * @returns {string}
 */
export function postLoginRedirectForServer(safePath, baseURL = '/') {
  const base = baseURL || '/';
  if (base === '/') {
    return safePath;
  }
  const baseNoTrail = base.endsWith('/') ? base.slice(0, -1) : base;
  if (safePath === baseNoTrail || safePath.startsWith(`${baseNoTrail}/`)) {
    return safePath;
  }
  const suffix = safePath.startsWith('/') ? safePath.slice(1) : safePath;
  return base.endsWith('/') ? `${base}${suffix}` : `${base}/${suffix}`;
}

