import { createURL, fetchURL, removePrefix, getApiPath,adjustedData} from "./utils";
import { baseURL } from "@/utils/constants";
import { state } from "@/store";
import { notify } from "@/notify";

// Notify if errors occur
export async function fetch(url, content = false) {
  try {
    url = removePrefix(url,"files");
    const apiPath = getApiPath("api/resources",{path: url, content: content});
    const res = await fetchURL(apiPath);
    const data = await res.json();
    return adjustedData(data,url);
  } catch (err) {
    notify.showError(err.message || "Error fetching data");
    throw err;
  }
}

async function resourceAction(url, method, content) {
  try {
    url = removePrefix(url,"files");
    let opts = { method };
    if (content) {
      opts.body = content;
    }
    const apiPath = getApiPath("api/resources", { path: url });
    const res = await fetchURL(apiPath, opts);
    return res;
  } catch (err) {
    notify.showError(err.message || "Error performing resource action");
    throw err;
  }
}

export async function remove(url) {
  try {
    return await resourceAction(url, "DELETE");
  } catch (err) {
    notify.showError(err.message || "Error deleting resource");
    throw err;
  }
}

export async function put(url, content = "") {
  try {
    return await resourceAction(url, "PUT", content);
  } catch (err) {
    notify.showError(err.message || "Error putting resource");
    throw err;
  }
}

export function download(format, ...files) {
  try {
    let url = `${baseURL}/api/raw`;
    if (files.length === 1) {
      url +=  "?path="+removePrefix(files[0], "files");
    } else {
      let arg = "";

      for (let file of files) {
        arg += removePrefix(file,"files") + ",";
      }

      arg = arg.substring(0, arg.length - 1);
      arg = encodeURIComponent(arg);
      url += `?files=${arg}`;
    }

    if (format) {
      url += `&algo=${format}`;
    }

    window.open(url);
  } catch (err) {
    notify.showError(err.message || "Error downloading files");
  }
}

export async function post(url, content = "", overwrite = false, onupload) {
  try {
    url = removePrefix(url,"files");

    let bufferContent;
    if (
      content instanceof Blob &&
      !["http:", "https:"].includes(window.location.protocol)
    ) {
      bufferContent = await new Response(content).arrayBuffer();
    }

    const apiPath = getApiPath("api/resources", { path: url, override: overwrite });
    return new Promise((resolve, reject) => {
      let request = new XMLHttpRequest();
      request.open(
        "POST",
        apiPath,
        true
      );
      request.setRequestHeader("X-Auth", state.jwt);

      if (typeof onupload === "function") {
        request.upload.onprogress = onupload;
      }

      request.onload = () => {
        if (request.status === 200) {
          resolve(request.responseText);
        } else if (request.status === 409) {
          reject(request.status);
        } else {
          reject(request.responseText);
        }
      };

      request.onerror = () => {
        reject(new Error("001 Connection aborted"));
      };

      request.send(bufferContent || content);
    });
  } catch (err) {
    notify.showError(err.message || "Error posting resource");
    throw err;
  }
}

function moveCopy(items, copy = false, overwrite = false, rename = false) {
  let promises = [];

  for (let item of items) {
    const from = item.from;
    const to = encodeURIComponent(removePrefix(item.to,"files"));
    const url = `${from}?action=${
      copy ? "copy" : "rename"
    }&destination=${to}&override=${overwrite}&rename=${rename}`;
    promises.push(resourceAction(url, "PATCH"));
  }

  return Promise.all(promises).catch((err) => {
    notify.showError(err.message || "Error moving/copying resources");
    throw err;
  });
}

export function move(items, overwrite = false, rename = false) {
  return moveCopy(items, false, overwrite, rename);
}

export function copy(items, overwrite = false, rename = false) {
  return moveCopy(items, true, overwrite, rename);
}

export async function checksum(url, algo) {
  try {
    const data = await resourceAction(`${url}?checksum=${algo}`, "GET");
    return (await data.json()).checksums[algo];
  } catch (err) {
    notify.showError(err.message || "Error fetching checksum");
    throw err;
  }
}

export function getDownloadURL(path, inline) {
  try {
    const params = {
      path: path,
      ...(inline && { inline: "true" }),
    };
    return createURL("api/raw", params);
  } catch (err) {
    notify.showError(err.message || "Error getting download URL");
    throw err;
  }
}

export function getPreviewURL(path, size) {
  try {
    const params = {
      path: path,
      size: size,
      inline: "true",
    };

    return createURL("api/preview", params);
  } catch (err) {
    notify.showError(err.message || "Error getting preview URL");
    throw err;
  }
}

export function getSubtitlesURL(file) {
  try {
    const params = {
      inline: "true",
    };

    const subtitles = [];
    for (const sub of file.subtitles) {
      subtitles.push(createURL("api/raw" + sub, params));
    }

    return subtitles;
  } catch (err) {
    notify.showError(err.message || "Error fetching subtitles URL");
    throw err;
  }
}

export async function usage(source) {
  try {
    const apiPath = getApiPath("api/usage", { source: source });
    const res = await fetchURL(apiPath);
    return await res.json();
  } catch (err) {
    notify.showError(err.message || "Error fetching usage data");
    throw err;
  }
}
