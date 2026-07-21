import { notify } from "@/notify";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL, requestTimeoutSignal } from "./utils";

const analyticsRequestTimeoutMs = 5000;

export function get(property="") {
  const path = getApiPath("settings", { property });
  return fetchJSON(path);
}

export async function update(settings) {
  await fetchURL("api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function config(showFull = false, showComments = false) {
  const params = {};
  if (showFull) params.full = "true";
  if (showComments) params.comments = "true";
  const path = getApiPath("settings/config", params);
  return fetchURL(path);
}

export async function sources() {
  try {
    const apiPath = getApiPath('settings/sources')
    const res = await fetchURL(apiPath)
    const data = await res.json()
    // Return empty object if no sources are available - this is not an error
    return data || {}
  } catch (err) {
    // Only show error for actual network/server errors, not for empty sources
    if (err.status && err.status !== 200) {
      notify.showError(err.message || 'Error fetching usage sources')
    }
    throw err
  }
}

export function getAnalytics() {
  return fetchJSON(getApiPath("settings/analytics"), {
    signal: requestTimeoutSignal(analyticsRequestTimeoutMs),
  });
}

export async function updateAnalytics({ enabled }) {
  return fetchJSON(getApiPath("settings/analytics"), {
    method: "PUT",
    body: JSON.stringify({ enabled }),
    signal: requestTimeoutSignal(analyticsRequestTimeoutMs),
  });
}

export function getAnalyticsPreview() {
  return fetchJSON(getApiPath("settings/analytics/preview"));
}

export function getUserDefaults() {
  return fetchJSON(getApiPath("settings/user-defaults"));
}

/** Enforcement flags for profile UI (works on public routes behind proxy basic auth). */
export function getEnforcedUserDefaults() {
  return fetchJSON(getPublicApiPath("settings/user-defaults"));
}

export async function patchUserDefaults(partial) {
  await fetchURL(getApiPath("settings/user-defaults"), {
    method: "PATCH",
    body: JSON.stringify(partial),
    headers: { "Content-Type": "application/json" },
  });
}

export function getSourceSettings() {
  return fetchJSON(getApiPath("settings/source"));
}

export async function patchSourceSettings(partial) {
  return fetchJSON(getApiPath("settings/source"), {
    method: "PATCH",
    body: JSON.stringify(partial),
  });
}
