import { adjustedData } from "./utils";
import { getApiPath, extractSourceFromPath } from "@/utils/url.js";
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
  const adjusted = adjustedData(data, `/share/${hash}${path}`);
  return adjusted
}

// Download files with given parameters
export function download(format, files) {
  if (format !== 'zip') {
    format = 'tar.gz'
  }
  let fileargs = ''
  if (files.length === 1) {
    fileargs = file + '||'
  } else {
    for (let file of files) {
      fileargs += file + '||'
    }
  }
  fileargs = fileargs.slice(0, -2) // remove trailing "||"
  const apiPath = getApiPath('api/public/dl', {
    files: encodeURIComponent(fileargs),
    algo: format
  })
  const url = window.origin + apiPath
  // Create a temporary <a> element to trigger the download
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', '') // Ensures it triggers a download
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link) // Clean up

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
export function getDownloadURL(share,files) {
  const params = {
    path: share.path,
    files: files,
    hash: share.hash,
    token: share.token,
    ...(share.inline && { inline: 'true' })
  }
  const apiPath = getApiPath("api/public/dl", params);
  return window.origin+apiPath
}
