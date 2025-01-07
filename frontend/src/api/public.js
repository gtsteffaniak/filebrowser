import { adjustedData } from "./utils";
import { getApiPath, removePrefix } from "@/utils/url.js";
import { notify } from "@/notify";

// Fetch public share data
export async function fetchPub(path, hash, password = "") {
  const params = { path, hash }
  const apiPath = getApiPath("api/public/share", params);
  const response = await fetch(apiPath, {
    headers: {
      "X-SHARE-PASSWORD": password ? encodeURIComponent(password) : "",
    },
  });

  if (!response.ok) {
    const error = new Error("Failed to connect to the server.");
    error.status = response.status;
    throw error;
  }
  let data = await response.json()
  const adjusted = adjustedData(data, getApiPath(`share/${hash}${path}`));
  return adjusted
}

// Download files with given parameters
export function download(share, ...files) {
  try {
    console.log(files)
    let fileInfo = files[0]
    if (files.length > 1) {
      fileInfo = files.map(encodeURIComponent).join(",");
    }
    const params = {
      "path": removePrefix(share.path, "share"),
      "hash": share.hash,
      "token": share.token,
      "inline": share.inline,
      "files": fileInfo,
    };
    const apiPath = getApiPath("api/public/dl", params);
    window.open(window.origin+apiPath)
  } catch (err) {
    notify.showError(err.message || "Error downloading files");
    throw err;
  }
}

// Get the public user data
export async function getPublicUser() {
  try {
    const apiPath = getApiPath("api/public/publicUser");
    const response = await fetch(apiPath);
    return response.json();
  } catch (err) {
    notify.showError(err.message || "Error fetching public user");
    throw err;
  }
}

// Generate a download URL
export function getDownloadURL(share) {
  const apiPath = getApiPath("api/public/dl", share);
  return window.origin+apiPath
}
