import { createURL } from "./utils";
import { notify } from "@/notify";

// Fetch public share data
export async function fetchPub(path, hash, password = "") {
  try {
    const params = {
      path,
      hash,
    }
    const url = createURL(`api/public/share`, params, false);
    const response = await fetch(url, {
      headers: {
        "X-SHARE-PASSWORD": password ? encodeURIComponent(password) : "",
      },
    });

    if (!response.ok) {
      const error = new Error("Failed to connect to the server.");
      error.status = response.status;
      throw error;
    }
    return response.json();


  } catch (err) {
    notify.showError(err.message || "Error fetching public share data");
    throw err;
  }
}

// Download files with given parameters
export function download(path, hash, token, format, ...files) {
  try {
    let fileInfo = files[0]
    if (files.length > 1) {
      fileInfo = files.map(encodeURIComponent).join(",");
    }
    const params = {
      path,
      hash,
      ...(format && { format}),
      ...(token && { token }),
      fileInfo
    };
    const url = createURL(`api/public/dl`, params, false);
    window.open(url);
  } catch (err) {
    notify.showError(err.message || "Error downloading files");
    throw err;
  }


}

// Get the public user data
export async function getPublicUser() {
  try {
    const url = createURL(`api/public/publicUser`, {}, false);
    const response = await fetch(url);
    return response.json();
  } catch (err) {
    notify.showError(err.message || "Error fetching public user");
    throw err;
  }
 
}

// Generate a download URL
export function getDownloadURL(path, hash, token, inline = false) {
  const params = {
    path,
    hash,
    ...(inline && { inline: "true" }),
    ...(token && { token }),
  };
  return createURL(`api/public/dl`, params, false);
}
