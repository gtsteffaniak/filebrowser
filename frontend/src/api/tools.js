import { fetchURL } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";

// GET /api/tools/search
// extraParams: optional { olderThan, newerThan, useWildcard, terms, termJoin, perSourceScopes }
// When perSourceScopes is a non-empty array of { source, path }, sends repeated scope=source:path and omits sources (multi-scope API).
// Otherwise legacy: sources + optional scope path for single source.
export async function search(base, sources, query, largest = false, extraParams = {}) {
  try {
    const params = {};

    const rawTerms = extraParams.terms;
    const terms =
      Array.isArray(rawTerms) ?
        rawTerms.map((t) => String(t).trim()).filter((t) => t !== "") :
        [];

    const prefixQuery = query === undefined || query === null ? "" : String(query).trim();

    if (terms.length > 0) {
      params.query = prefixQuery;
      params.terms = terms;
      if (extraParams.termJoin === "and") {
        params.termJoin = "and";
      }
    } else {
      params.query = prefixQuery;
    }

    const perSource = extraParams.perSourceScopes;
    if (Array.isArray(perSource) && perSource.length > 0) {
      params.scope = perSource.map(({ source, path }) => {
        const sourceName = String(source || "").trim();
        let scopedPath =
          path === undefined || path === null ? "/" : String(path).trim();
        if (scopedPath === "") {
          scopedPath = "/";
        }
        if (!scopedPath.startsWith("/")) {
          scopedPath = `/${scopedPath}`;
        }
        return `${sourceName}:${scopedPath}`;
      });
    } else {
      const sourcesArray = Array.isArray(sources) ? sources : [sources];
      params.sources = sourcesArray.join(",");

      if (sourcesArray.length === 1 && base) {
        let scopeBase = base;
        if (!scopeBase.endsWith("/")) {
          scopeBase += "/";
        }
        params.scope = scopeBase;
      }
    }

    if (largest) {
      params.largest = "true";
    }

    if (extraParams.olderThan !== undefined && extraParams.olderThan !== "") {
      params.olderThan = String(extraParams.olderThan);
    }
    if (extraParams.newerThan !== undefined && extraParams.newerThan !== "") {
      params.newerThan = String(extraParams.newerThan);
    }
    if (
      extraParams.useWildcard === true ||
      extraParams.useGlob === true ||
      extraParams.glob === true
    ) {
      params.useWildcard = "true";
    }

    const apiPath = getApiPath("tools/search", params);
    const res = await fetchURL(apiPath);
    let data = await res.json();

    return data;
  } catch (err) {
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}

// GET /api/tools/duplicateFinder
export async function duplicateFinder(base, source, minSizeMb, useChecksum = false) {
  try {
    if (!base.endsWith("/")) {
      base += "/";
    }
    const params = {
      scope: base,
      source: source,
      minSizeMb: minSizeMb.toString()
    };

    if (useChecksum) {
      params.useChecksum = "true";
    }

    const apiPath = getApiPath("tools/duplicateFinder", params);
    const res = await fetchURL(apiPath);
    const data = await res.json();

    return {
      groups: data.groups || data,
      incomplete: data.incomplete || false,
      reason: data.reason || ""
    };
  } catch (err) {
    notify.showError(err.message || "Error occurred while finding duplicates");
    throw err;
  }
}

// GET /api/tools/fileWatcher
export async function fileWatcherLatencyCheck() {
  try {
    const apiPath = getApiPath("tools/fileWatcher", { latencyCheck: "true" });
    await fetchURL(apiPath);
  } catch (err) {
    notify.showError(err.message || "Error occurred while checking latency");
    throw err;
  }
}

// GET /api/tools/fileWatcher
export async function fileWatcher(source, path, lines) {
  try {
    const params = { source, path, lines: lines };
    const apiPath = getApiPath("tools/fileWatcher", params);
    const res = await fetchURL(apiPath);
    return await res.json();
  } catch (err) {
    notify.showError(err.message || "Error watching files");
    throw err;
  }
}

// GET /api/tools/fileWatcher/sse
export function fileWatcherSSE(source, path, lines, interval, onMessage, onError) {
  try {
    const params = { 
      source, 
      path,
      lines: lines.toString(),
      interval: interval.toString()
    };
    const apiPath = getApiPath("tools/fileWatcher/sse", params);
    const eventSource = new EventSource(apiPath);
    
    eventSource.onmessage = onMessage;
    eventSource.onerror = (err) => {
      eventSource.close();
      if (onError) onError(err);
    };
    
    return eventSource;
  } catch (err) {
    notify.showError(err.message || "Error establishing file watch connection");
    throw err;
  }
}
