import { fetchURL } from "./utils";
import { getApiPath } from "@/utils/url.js";
import { notify } from "@/notify";

/**
 * Create an archive on the server (server-side only).
 * @param {Object} opts
 * @param {string} opts.source - Source name for items (files/dirs to add)
 * @param {string} [opts.toSource] - Source where to write the archive (default: source)
 * @param {string[]} opts.items - Paths to add (files/dirs; folders are walked; access-denied skipped)
 * @param {string} opts.destination - Path where to write the archive (e.g. /folder/out.zip)
 * @param {string} [opts.format] - "zip" or "tar.gz"; default from destination extension
 * @param {number} [opts.compression] - 0-9 for tar.gz; 0 = default
 */
export async function createArchive(opts) {
  const { source, toSource, items, destination, format, compression } = opts;
  if (!source || !items?.length || !destination) {
    throw new Error("source, items, and destination are required");
  }
  const body = {
    source,
    items,
    destination,
    ...(toSource && toSource !== source && { toSource }),
    ...(format && { format }),
    ...(compression !== undefined && compression !== null && { compression }),
  };
  try {
    const apiPath = getApiPath("api/resources/archive");
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
    const apiPath = getApiPath("api/resources/unarchive");
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
