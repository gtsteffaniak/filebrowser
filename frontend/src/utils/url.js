import { router } from "@/router";
import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants.js";

export default {
  pathsMatch,
  removeTrailingSlash,
  removeLeadingSlash,
  encodeRFC5987ValueChars,
  removeLastDir,
  getParentDir,
  encodePath,
  removePrefix,
  getApiPath,
  extractSourceFromPath,
  base64Encode,
  joinPath,
  goToItem,
  buildItemUrl,
  encodedPath,
  trimSlashes,
  getPublicApiPath,
  resolveRelativePath,
};

export function removeLastDir(url) {
  const arr = url.split("/");
  if (arr.pop() === "") {
    arr.pop();
  }

  return arr.join("/");
}

export function getParentDir(path) {
  if (!path || path === "/") {
    return "/";
  }
  const parent = removeLastDir(path);
  return parent && parent !== "" ? parent : "/";
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
  return removeTrailingSlash(url1) === removeTrailingSlash(url2);
}

export function removePrefix(path, prefix = "") {
  if (path === undefined) {
    return ""
  }
  path = removeLeadingSlash(path)
  if (prefix !== "") {
    prefix = trimSlashes(prefix)
  }
  // Remove combined (globalVars.baseURL + prefix) from the start of the path if present
  if (path.startsWith(prefix)) {
    path = path.slice(prefix.length);
  }

  // Ensure path starts with '/'
  if (!path.startsWith('/')) {
    path = `/${path}`;
  }
  return path;
}


// get path with parameters
// Supports array values for repeated parameters (e.g., file=a&file=b&file=c)
export function getApiPath(path, params = {}, skipEncode = false, isPublic = false) {
  if (path.startsWith("/")) {
    path = path.slice(1);
  }
  const prefix = isPublic ? "public/api/" : "api/";
  path = `${globalVars.baseURL}${prefix}${path}`;

  const entries = Object.entries(params);
  if (entries.length > 0) {
    if (!skipEncode) {
      const encodedParams = [];
      for (const [key, value] of entries) {
        if (value === undefined || value === null || value === "") continue;
        // Handle array values for repeated parameters
        if (Array.isArray(value)) {
          for (const v of value) {
            encodedParams.push(`${encodeURIComponent(key)}=${encodeURIComponent(v)}`);
          }
        } else {
          encodedParams.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
        }
      }
      if (encodedParams.length > 0) {
        path += `?${encodedParams.join('&')}`;
      }
    } else {
      const queryParams = [];
      for (const [key, value] of entries) {
        if (value === undefined || value === null || value === "") continue;
        // Handle array values for repeated parameters
        if (Array.isArray(value)) {
          for (const v of value) {
            queryParams.push(`${key}=${v}`);
          }
        } else {
          queryParams.push(`${key}=${value}`);
        }
      }
      if (queryParams.length > 0) {
        path += `?${queryParams.join('&')}`;
      }
    }
  }
  return path;
}

// get path with parameters
// relative path so it can be used behind proxy
export function getPublicApiPath(path, params = {}, skipEncode = false) {
  return getApiPath(path, params, skipEncode, true);
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
      result += `/${segment.replace(/^\/+/, '')}`;
    }
  }
  return result;
}

/** Resolve a relative or root-relative path against a base file path (POSIX-style). */
export function resolveRelativePath(baseFilePath, refPath) {
  if (!baseFilePath || !refPath) {
    return refPath;
  }

  let pathPart = refPath;
  let suffix = "";
  const queryIndex = refPath.indexOf("?");
  const hashIndex = refPath.indexOf("#");
  const cutIndex = Math.min(
    queryIndex === -1 ? refPath.length : queryIndex,
    hashIndex === -1 ? refPath.length : hashIndex,
  );
  if (cutIndex < refPath.length) {
    suffix = refPath.slice(cutIndex);
    pathPart = refPath.slice(0, cutIndex);
  }

  const baseDir = getParentDir(baseFilePath);
  const combined = pathPart.startsWith("/")
    ? pathPart
    : joinPath(baseDir, pathPart);

  const parts = combined.split("/").filter(Boolean);
  const resolved = [];
  for (const part of parts) {
    if (part === ".") {
      continue;
    }
    if (part === "..") {
      if (resolved.length > 0) {
        resolved.pop();
      }
      continue;
    }
    resolved.push(part);
  }

  return `/${resolved.join("/")}${suffix}`;
}

export function base64Encode(str) {
  return btoa(unescape(encodeURIComponent(str)));
}

// expect url to include /files/ prefix
export function extractSourceFromPath(url) {
  let path = url;
  const source = path.split('/')[2];
  path = removePrefix(path, `/files/${source}`);
  return { source, path };
}

export function buildItemUrl(source, path, includeBaseURL = false) {
  path = removeLeadingSlash(path);
  const encodedPath = encodePath(path);
  let urlPath;
  if (getters.isShare()) {
    urlPath = `public/share/${state.shareInfo.hash}/${encodedPath}`;
  } else {
    urlPath = `files/${encodeURIComponent(source)}/${encodedPath}`;
  }
  if (includeBaseURL) {
    return `${globalVars.baseURL}${urlPath}`;
  }
  return `/${urlPath}`;
}

export function encodedPath(path) {
  if (path === undefined) {
    return "";
  }
  const parts = path.split("/");
  const encodedParts = parts.map(part => encodeURIComponent(part));
  return encodedParts.join("/").replace("//", "/");
}

// assume non-encoded input path and source
export function goToItem(source, path, previousHistoryItem, newTab = false, isShare = false) {
  const cv = getters.currentView();
  if (source === state.sources.current && path === state.req.path && cv === "listingView") {
    return;
  }
  if (previousHistoryItem && cv === "listingView") {
    mutations.setPreviousHistoryItem(previousHistoryItem);
  }
  mutations.resetAll()
  const newPath = encodedPath(path);
  let fullPath;
  if (isShare) {
    fullPath = `/public/share/${encodeURIComponent(source)}${newPath}`;
  } else {
    fullPath = `/files/${encodeURIComponent(source)}${newPath}`;
  }
  if (newTab) {
    // Use absolute URL for new tab to ensure proper navigation
    const absoluteUrl = `${window.location.origin}${globalVars.baseURL}${fullPath.startsWith('/') ? fullPath.slice(1) : fullPath}`;
    window.open(absoluteUrl, '_blank');
    return;
  }

  if (previousHistoryItem === undefined) {
    // When undefined will not create browser history
    void router.replace({ path: fullPath });
    return
  }
  void router.push({ path: fullPath });
  return
}
