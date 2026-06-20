import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { fetchURL, fetchJSON } from "./utils";

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
    const data = await res.json();

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

function buildActivityParams({
  from,
  to,
  scope,
  eventType,
  username,
  source,
  path,
  pathGlob,
  shareHash,
  page,
  limit,
  interval,
  splitBy,
  groupBy,
}) {
  const params = {};
  if (from !== undefined && from !== null) params.from = String(from);
  if (to !== undefined && to !== null) params.to = String(to);
  if (scope && scope !== "all") params.scope = scope;
  if (eventType) params.eventType = eventType;
  if (username) params.username = username;
  if (source) params.source = source;
  if (path) params.path = path;
  if (pathGlob) params.pathGlob = pathGlob;
  if (shareHash) params.shareHash = shareHash;
  if (page !== undefined && page !== null) params.page = String(page);
  if (limit !== undefined && limit !== null) params.limit = String(limit);
  if (interval) params.interval = interval;
  if (splitBy) params.splitBy = splitBy;
  if (groupBy) params.groupBy = groupBy;
  return params;
}

// GET /api/tools/activity — ungrouped event list
export async function activityList(options = {}) {
  return fetchJSON(getApiPath("tools/activity", buildActivityParams(options)));
}

// GET /api/tools/activity/grouped — chart/stat buckets
export async function activityGrouped(options = {}) {
  return fetchJSON(getApiPath("tools/activity/grouped", buildActivityParams(options)));
}

/** @deprecated Use activityGrouped */
export const activityStats = activityGrouped;

// GET /api/tools/activity/export — returns URL for download
export function activityExportUrl(options = {}) {
  return getApiPath("tools/activity/export", buildActivityParams(options));
}
