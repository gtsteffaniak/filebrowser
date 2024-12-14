import { baseURL } from "@/utils/constants.js";

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
  encodeRFC5987ValueChars,
  removeLastDir,
  encodePath,
  removePrefix,
  getApiPath
};

export function removePrefix(path, prefix = "") {
  if (path === undefined) {
    return ""
  }
  if (prefix != "") {
    prefix = "/" + trimSlashes(prefix)
  }
  const combined = trimSlashes(baseURL) + prefix
  const combined2 = "/" + combined
  // Remove combined (baseURL + prefix) from the start of the path if present
  if (path.startsWith(combined)) {
    path = path.slice(combined.length);
  } else if (path.startsWith(combined2)) {
    path = path.slice(combined2.length);
  } else if (path.startsWith(prefix)) {
    // Fallback: remove only the prefix if the combined string isn't present
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
  path = `${baseURL}${path}`;
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

export function base64Encode(str) {
  return btoa(unescape(encodeURIComponent(str)));
}