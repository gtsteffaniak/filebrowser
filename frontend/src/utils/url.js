import { globalVars } from "@/utils/constants.js";
import { state, mutations, getters } from "@/store";
import { router } from "@/router";

export function removeLastDir(url) {
  var arr = url.split("/");
  if (arr.pop() === "") {
    arr.pop();
  }

  return arr.join("/");
}

// this code borrow from mozilla
// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/encodeURIComponent#Examples
export function encodeRFC5987ValueChars(str) {
  return (
    encodeURIComponent(str)
      // Note that although RFC3986 reserves "!", RFC5987 does not,
      // so we do not need to escape it
      .replace(/['()]/g, escape) // i.e., %27 %28 %29
      .replace(/\*/g, "%2A")
      // The following are not required for percent-encoding per RFC5987,
      // so we can allow for a little better readability over the wire: |`^
      .replace(/%(?:7C|60|5E)/g, unescape)
  );
}

export function encodePath(str) {
  return str
    .split("/")
    .map((v) => encodeURIComponent(v))
    .join("/");
}


export function pathsMatch(url1, url2) {
  return removeTrailingSlash(url1) == removeTrailingSlash(url2);
}

export default {
  pathsMatch,
  removeTrailingSlash,
  removeLeadingSlash,
  encodeRFC5987ValueChars,
  removeLastDir,
  encodePath,
  removePrefix,
  getApiPath,
  extractSourceFromPath,
  fixDownloadURL,
  base64Encode,
  joinPath,
  goToItem,
  buildItemUrl,
  encodedPath,
  doubleEncode,
  trimSlashes,
  getPublicApiPath,
};

export function removePrefix(path, prefix = "") {
  if (path === undefined) {
    return ""
  }
  path = removeLeadingSlash(path)
  if (prefix != "") {
    prefix = trimSlashes(prefix)
  }
  // Remove combined (globalVars.baseURL + prefix) from the start of the path if present
  if (path.startsWith(prefix)) {
    path = path.slice(prefix.length);
  }

  // Ensure path starts with '/'
  if (!path.startsWith('/')) {
    path = '/' + path;
  }
  return path;
}


// get path with parameters
export function getApiPath(path, params = {}) {
  if (path.startsWith("/")) {
    path = path.slice(1);
  }
  path = `${globalVars.baseURL}${path}`;
  if (Object.keys(params).length > 0) {
    path += "?";
  }
  for (const key in params) {
    if (params[key] === undefined) {
      continue;
    }
    path += `${key}=${params[key]}&`;
  }
  // remove trailing &
  if (path.endsWith("&")) {
    path = path.slice(0, -1);
  }
  return path;
}

// get path with parameters
// relative path so it can be used behind proxy
export function getPublicApiPath(path, params = {}) {
  return getApiPath(`/public/api/${path}`, params);
}

export function removeTrailingSlash(str) {
  if (str.endsWith('/')) {
    return str.slice(0, -1);
  }
  return str;
}

export function removeLeadingSlash(str) {
  if (str.startsWith('/')) {
    return str.slice(1);
  }
  return str;
}

export function trimSlashes(str) {
  return removeLeadingSlash(removeTrailingSlash(str))
}

export function joinPath(basePath, ...segments) {
  if (!basePath) {
    basePath = '/'
  }
  // Remove trailing slash from base path and leading slashes from segments
  let result = basePath.replace(/\/$/, '');
  for (const segment of segments) {
    if (segment) {
      result += '/' + segment.replace(/^\/+/, '');
    }
  }
  return result;
}

export function base64Encode(str) {
  return btoa(unescape(encodeURIComponent(str)));
}

// expect url to include /files/ prefix
export function extractSourceFromPath(url) {
  let source;
  let path = url;
  source = path.split('/')[2];
  path = removePrefix(path, `/files/${source}`);

  return { source, path };
}

export function buildItemUrl(source, path) {
  if (getters.isShare()) {
    return `/public/share/${state.shareInfo.hash}${path}`;
  }
  return `/files/${source}${path}`;
}

export function encodedPath(path) {
  if (path === undefined) {
    return "";
  }
  // break apart path into parts and url encode each part
  const parts = path.split("/");
  const encodedParts = parts.map(part => encodeURIComponent(part));
  return encodedParts.join("/").replace("//", "/");
}

// assume non-encoded input path and source
export function goToItem(source, path, previousHistoryItem) {
  if (source == state.sources.current && path == state.req.path) {
    return;
  }
  mutations.setPreviousHistoryItem(previousHistoryItem);
  mutations.resetAll()
  let newPath = encodedPath(path);
  let fullPath;
  if (getters.isShare()) {
    fullPath = `/public/share/${state.shareInfo?.hash}${newPath}`;
    router.push({ path: fullPath });
    return;
  }
  fullPath = `/files/${encodeURIComponent(source)}${newPath}`;
  router.push({ path: fullPath });
  return
}

export function doubleEncode(str) {
  return encodeURIComponent(encodeURIComponent(str));
}

/**
 * Fixes download URLs by replacing everything before /public/api
 * with the current window.location.origin + globalVars.baseURL
 * This is only needed when the backend didn't use externalUrl
 * @param {string} downloadUrl - The original download URL from backend
 * @returns {string} - The corrected URL using current client origin
 */
export function fixDownloadURL(downloadUrl) {
  if (!downloadUrl) {
    return downloadUrl;
  }
  // Find the position of /public/api in the URL
  const publicApiIndex = downloadUrl.indexOf('/public/api');
  if (publicApiIndex === -1) {
    // If /public/api is not found, return the original URL
    return downloadUrl;
  }

  // Extract the part from /public/api onwards
  const publicApiPath = downloadUrl.substring(publicApiIndex);

  // Build the corrected URL using current client origin and globalVars.baseURL
  const correctedBaseURL = removeTrailingSlash(globalVars.baseURL);
  return `${window.location.origin}${correctedBaseURL}${publicApiPath}`;
}