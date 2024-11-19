import { createURL, fetchURL, adjustedData} from "./utils";
import { baseURL } from "@/utils/constants";
import { removePrefix,getApiPath } from "@/utils/url.js";
import { state } from "@/store";
import { notify } from "@/notify";

// Notify if errors occur
export async function fetchFiles(url, content = false) {
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

export async function moveCopy(items, action = "copy", overwrite = false, rename = false) {
  let promises = [];
  let params = {
    overwrite: overwrite,
    action: action,
    rename: rename,
  }
  try {
    for (let item of items) {
      let localParams = { ...params };
      localParams.destination = item.to;
      localParams.from = item.from;
      const apiPath = getApiPath("api/resources", localParams);
      promises.push(fetch(apiPath, { method: "PATCH" }));
    }
    return promises;

  } catch (err) {
    console.log("errorsss", err);
    notify.showError(err.message || "Error moving/copying resources");
    throw err;
  }
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

export function getPreviewURL(path, size, modified) {
  try {
    const params = {
      path: path,
      size: size,
      key: Date.parse(modified),
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
