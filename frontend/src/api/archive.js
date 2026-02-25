import { fetchURL } from "./utils";
import { getApiPath } from "@/utils/url.js";
import { notify } from "@/notify";

/**
 * Create an archive on the server (server-side only).
 * @param {Object} opts
 * @param {string} opts.fromSource - Source name where paths to archive live
 * @param {string} [opts.toSource] - Source where to write the archive (default: fromSource)
 * @param {string[]} opts.paths - Paths to add (files/dirs; folders are walked; access-denied skipped)
 * @param {string} opts.destination - Path where to write the archive (e.g. /folder/out.zip)
 * @param {string} [opts.format] - "zip" or "tar.gz"; default from destination extension
 * @param {number} [opts.compression] - 0-9 for tar.gz; 0 = default
 * @param {boolean} [opts.deleteAfter] - Delete source files/folders after successful creation
 */
export async function createArchive(opts) {
  const { fromSource, toSource, paths, destination, format, compression, deleteAfter } = opts;
  if (!fromSource || !paths?.length || !destination) {
    throw new Error("fromSource, paths, and destination are required");
  }
  const body = {
    fromSource,
    paths,
    destination,
    ...(toSource && toSource !== fromSource && { toSource }),
    ...(format && { format }),
    ...(compression !== undefined && compression !== null && { compression }),
    ...(deleteAfter && { deleteAfter: true }),
  };
  try {
    const apiPath = getApiPath("resources/archive");
    const response = await fetchURL(apiPath, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return response.json();
  } catch (err) {
    notify.showError(err.message || "Error creating archive");
    throw err;
  }
}

/**
 * Unarchive (extract) an archive on the server (server-side only).
 * @param {Object} opts
 * @param {string} opts.fromSource - Source where the archive file lives
 * @param {string} [opts.toSource] - Source to extract to (default: fromSource)
 * @param {string} opts.path - Archive file path
 * @param {string} opts.destination - Directory path to extract into
 * @param {boolean} [opts.deleteAfter] - Delete archive after successful extract
 */
export async function unarchive(opts) {
  const { fromSource, toSource, path, destination, deleteAfter } = opts;
  if (!fromSource || !path || !destination) {
    throw new Error("fromSource, path, and destination are required");
  }
  const body = {
    fromSource,
    ...(toSource && toSource !== fromSource && { toSource }),
    path,
    destination,
    ...(deleteAfter && { deleteAfter: true }),
  };
  try {
    const apiPath = getApiPath("resources/unarchive");
    const response = await fetchURL(apiPath, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return response.json();
  } catch (err) {
    notify.showError(err.message || "Error extracting archive");
    throw err;
  }
}
