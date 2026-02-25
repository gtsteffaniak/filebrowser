import { fetchURL } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";

// GET /api/tools/search
export async function search(base, sources, query, largest = false) {
  try {
    const sourcesArray = Array.isArray(sources) ? sources : [sources];
    
    const params = {
      query: query,
      sources: sourcesArray.join(",")
    };

    // Only include scope if searching a single source
    if (sourcesArray.length === 1 && base) {
      if (!base.endsWith("/")) {
        base += "/";
      }
      params.scope = base;
    }

    if (largest) {
      params.largest = "true";
    }

    const apiPath = getApiPath("tools/search", params);
    const res = await fetchURL(apiPath);
    let data = await res.json();

    return data
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
export async function fileWatcher(source, path) {
  try {
    const params = { source, path };
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
