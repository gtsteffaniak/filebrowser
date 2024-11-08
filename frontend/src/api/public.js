import { createURL } from "./utils";
import { baseURL } from "@/utils/constants";

export async function fetchPub(path, hash, password = "") {
  const res = await fetch(
      `/api/public/share?path=${path}&hash=${hash}`,
      {
        headers: {
          "X-SHARE-PASSWORD": encodeURIComponent(password),
        },
      }
  );
  if (res.status != 200) {
    const error = new Error("000 No connection");
    error.status = res.status;
    throw error;
  }
  return res.json();
}

export function download(path, hash, token, format, ...files) {
  let url = `${baseURL}/api/public/dl?path=${path}&hash=${hash}`;
  if (files.length === 1) {
    url += encodeURIComponent(files[0]) + "?";
  } else {
    let arg = "";
    for (let file of files) {
      arg += encodeURIComponent(file) + ",";
    }

    arg = arg.substring(0, arg.length - 1);
    arg = encodeURIComponent(arg);
    url += `/?files=${arg}&`;
  }

  if (format) {
    url += `&algo=${format}`;
  }

  if (token) {
    url += `&token=${token}`;
  }

  window.open(url);
}

export function getPublicUser() {
  return fetch("/api/public/publicUser")
    .then(response => {
      if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
      }
      return response.json();
    })
    .catch(error => {
      console.error("Error fetching public user:", error);
      throw error;
    });
}

export function getDownloadURL(path,hash, token, inline = false) {
  const params = {
    path: path,
    hash: hash,
    ...(inline && { inline: "true" }),
    ...(token && { token: token }),
  };
  return createURL(`api/public/dl`, params, false);
}
