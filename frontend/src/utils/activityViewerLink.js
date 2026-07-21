import { formatActivityViewerQueryString } from "@/utils/activityViewerQuery.js";
import { globalVars } from "@/utils/constants";
import { router } from "@/router";

/** @typedef {Record<string, string>} ActivityViewerQuery */

export const ACTIVITY_VIEWER_PATH = "tools/activityViewer";

/**
 * @param {ActivityViewerQuery} [params]
 * @returns {ActivityViewerQuery}
 */
export function buildActivityViewerQuery(params = {}) {
  /** @type {ActivityViewerQuery} */
  const query = {};
  if (params.scope && params.scope !== "all") {
    query.scope = params.scope;
  }
  if (params.eventType) {
    query.eventType = params.eventType;
  }
  if (params.source) {
    query.source = params.source;
  }
  if (params.path && params.path !== "/") {
    query.path = params.path;
  }
  if (params.shareHash) {
    query.shareHash = params.shareHash;
  }
  if (params.username) {
    query.username = params.username;
  }
  if (params.view && params.view !== "table") {
    query.view = params.view;
  }
  return query;
}

/**
 * @param {ActivityViewerQuery} [params]
 * @returns {string}
 */
export function activityViewerUrl(params = {}) {
  const query = buildActivityViewerQuery(params);
  const base = globalVars.baseURL || "/";
  const normalizedBase = base.endsWith("/") ? base : `${base}/`;
  const path = `${normalizedBase}${ACTIVITY_VIEWER_PATH}`;
  const qs = formatActivityViewerQueryString(query);
  return qs ? `${path}?${qs}` : path;
}

/**
 * Navigate to the activity viewer using vue-router (SPA navigation).
 * @param {string} href Full activity viewer URL from {@link activityViewerUrl}.
 */
export function navigateActivityViewerHref(href) {
  const url = new URL(href, window.location.origin);
  let path = url.pathname;
  const base = (globalVars.baseURL || "/").replace(/\/$/, "");
  if (base && base !== "/" && path.startsWith(base)) {
    path = path.slice(base.length) || "/";
  }
  if (!path.startsWith("/")) {
    path = `/${path}`;
  }
  /** @type {Record<string, string>} */
  const query = {};
  url.searchParams.forEach((value, key) => {
    query[key] = value;
  });
  void router.push({ path, query });
}

export const ACCESS_EVENT_TYPES = ["accessCreate", "accessUpdate", "accessDelete"];

export const ACCESS_ACTIVITY_EVENT_TYPES = ACCESS_EVENT_TYPES.join(",");

export const activityViewerPresets = {
  shares: () => activityViewerUrl({ scope: "shares" }),
  shareHash: (shareHash) => activityViewerUrl({ scope: "shares", shareHash }),
  sharePath: (source, path) =>
    activityViewerUrl({ scope: "shares", source, path }),
  users: () =>
    activityViewerUrl({
      eventType: "userCreate,userUpdate,userDelete",
    }),
  apiTokens: () =>
    activityViewerUrl({
      eventType: "tokenCreate,tokenDelete",
    }),
  access: (source, path) =>
    activityViewerUrl({
      scope: "access",
      eventType: ACCESS_ACTIVITY_EVENT_TYPES,
      source,
      path,
    }),
  files: (source, path) =>
    activityViewerUrl({
      scope: "files",
      source,
      path,
    }),
};
