import { fetchURL, adjustedData } from "./utils";
import { removePrefix, getApiPath } from "@/utils/url.js";
import { state } from "@/store";
import { notify } from "@/notify";
import { externalUrl } from "@/utils/constants";

// Notify if errors occur
export async function fetchFiles(url, content = false) {
  try {
    let path = encodeURIComponent(removePrefix(url, "files"));
    const apiPath = getApiPath("api/resources",{path: path, content: content});
    const res = await fetchURL(apiPath);
    const data = await res.json();
    const adjusted = adjustedData(data, url);
    return adjusted;
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
    let path = encodeURIComponent(removePrefix(url, "files"));
    const apiPath = getApiPath("api/resources", { path: path });
    const res = await fetchURL(apiPath, opts);
    return res;
  } catch (err) {
    notify.showError(err.message || "Error performing resource action");
    throw err;
  }
}

export async function remove(url) {
  try {
    let path = encodeURIComponent(removePrefix(url, "files"));
    return await resourceAction(path, "DELETE");
  } catch (err) {
    notify.showError(err.message || "Error deleting resource");
    throw err;
  }
}

export async function put(url, content = "") {
  try {
    let path = encodeURIComponent(removePrefix(url, "files"));
    return await resourceAction(path, "PUT", content);
  } catch (err) {
    notify.showError(err.message || "Error putting resource");
    throw err;
  }
}

export function download(format, files) {
  if (format != "zip") {
    format = "tar.gz"
  }
  try {
    let fileargs = "";
    if (files.length === 1) {
      fileargs = decodeURI(removePrefix(files[0], "files"))
    } else {
      for (let file of files) {
        fileargs += decodeURI(removePrefix(file,"files")) + ",";
      }
      fileargs = fileargs.substring(0, fileargs.length - 1);
    }
    const apiPath = getApiPath("api/raw", { files: encodeURIComponent(fileargs), algo: format });
    const url = window.origin+apiPath
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
  let params = {
    overwrite: overwrite,
    action: action,
    rename: rename,
  };
  try {
    // Create an array of fetch calls
    let promises = items.map((item) => {
      let toPath = encodeURIComponent(removePrefix(decodeURI(item.to), "files"));
      let fromPath = encodeURIComponent(removePrefix(decodeURI(item.from), "files"));
      let localParams = { ...params, destination: toPath, from: fromPath };
      const apiPath = getApiPath("api/resources", localParams);
      return fetch(apiPath, { method: "PATCH" }).then((response) => {
        if (!response.ok) {
          // Throw an error if the fetch fails
          return response.text().then((text) => {
            throw new Error(`Failed to move/copy: ${text || response.statusText}`);
          });
        }
        return response;
      });
    });

    // Await all promises and ensure errors propagate
    await Promise.all(promises);
  } catch (err) {
    notify.showError(err.message || "Error moving/copying resources");
    throw err; // Re-throw the error to propagate it back to the caller
  }
}


export async function checksum(path, algo) {
  try {
    const params = {
      path: encodeURIComponent(removePrefix(path, "files")),
      checksum: algo,
    };
    const apiPath = getApiPath("api/resources", params);
    const res = await fetchURL(apiPath);
    const data = await res.json();
    return data.checksums[algo];
  } catch (err) {
    notify.showError(err.message || "Error fetching checksum");
    throw err;
  }
}

export function getDownloadURL(path, inline, useExternal) {
  try {
    const params = {
      files: encodeURIComponent(removePrefix(decodeURI(path),"files")),
      ...(inline && { inline: "true" }),
    };
    const apiPath = getApiPath("api/raw", params);
    if (externalUrl && useExternal) {
      return externalUrl+apiPath
    }
    return window.origin+apiPath
  } catch (err) {
    notify.showError(err.message || "Error getting download URL");
    throw err;
  }
}

export function getPreviewURL(path, size, modified) {
  try {
    const params = {
      path: encodeURIComponent(removePrefix(decodeURI(path),"files")),
      size: size,
      key: Date.parse(modified),
      inline: "true",
    };
    const apiPath = getApiPath("api/preview", params);
    return window.origin+apiPath
  } catch (err) {
    notify.showError(err.message || "Error getting preview URL");
    throw err;
  }
}

export function getSubtitlesURL(file) {
  try {
    const subtitles = [];
    for (const sub of file.subtitles) {
      const params = {
        inline: "true",
        path: encodeURIComponent(removePrefix(sub,"files"))
      };
      const apiPath = getApiPath("api/raw", params);
      return window.origin+apiPath
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
