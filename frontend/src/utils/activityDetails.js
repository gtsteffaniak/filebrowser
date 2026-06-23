/**
 * Label for an activity event type.
 */
export function activityEventLabel(eventType, $t) {
  switch (eventType) {
    case "download":
    case "shareDownload":
      return $t("general.download");
    case "move":
      return $t("general.move");
    case "copy":
      return $t("general.copy");
    case "rename":
      return $t("general.rename");
    case "upload":
      return $t("general.upload");
    case "delete":
      return $t("general.delete");
    case "login":
      return $t("general.login");
    case "logout":
      return $t("general.logout");
    case "signup":
      return $t("general.signup");
    case "archive":
      return $t("prompts.archive");
    case "unarchive":
      return $t("prompts.unarchive");
    case "duplicateFinder":
      return $t("tools.duplicateFinder.name");
    case "bulkDelete":
      return $t("tools.activityViewer.eventBulkDelete");
    case "shareCreate":
      return $t("tools.activityViewer.eventShareCreate");
    case "shareUpdate":
      return $t("tools.activityViewer.eventShareUpdate");
    case "shareDelete":
      return $t("tools.activityViewer.eventShareDelete");
    case "userCreate":
      return $t("tools.activityViewer.eventUserCreate");
    case "userUpdate":
      return $t("tools.activityViewer.eventUserUpdate");
    case "userDelete":
      return $t("tools.activityViewer.eventUserDelete");
    case "accessCreate":
      return $t("tools.activityViewer.eventAccessCreate");
    case "accessUpdate":
      return $t("tools.activityViewer.eventAccessUpdate");
    case "accessDelete":
      return $t("tools.activityViewer.eventAccessDelete");
    case "tokenCreate":
      return $t("tools.activityViewer.eventTokenCreate");
    case "tokenDelete":
      return $t("tools.activityViewer.eventTokenDelete");
    case "passkeyRegister":
      return $t("tools.activityViewer.passkeyRegister");
    case "passkeyDelete":
      return $t("tools.activityViewer.passkeyDelete");
    default:
      return eventType;
  }
}

const DELETE_EVENT_TYPES = new Set([
  "delete",
  "bulkDelete",
  "shareDelete",
  "userDelete",
  "accessDelete",
]);

const CREATE_EVENT_TYPES = new Set([
  "upload",
  "shareCreate",
  "userCreate",
  "accessCreate",
]);

const CHANGE_EVENT_TYPES = new Set([
  "move",
  "copy",
  "rename",
  "shareUpdate",
  "userUpdate",
  "accessUpdate",
  "archive",
  "unarchive",
]);

const AUTH_EVENT_TYPES = new Set([
  "login",
  "logout",
  "signup",
  "passkeyRegister",
  "passkeyDelete",
  "tokenCreate",
  "tokenDelete",
]);

/**
 * CSS modifier class for an activity event type badge in the table.
 * @returns {string}
 */
export function activityEventTypeBadgeClass(eventType) {
  if (DELETE_EVENT_TYPES.has(eventType)) {
    return "event-type-badge--delete";
  }
  if (AUTH_EVENT_TYPES.has(eventType)) {
    return "event-type-badge--auth";
  }
  if (CHANGE_EVENT_TYPES.has(eventType)) {
    return "event-type-badge--change";
  }
  if (CREATE_EVENT_TYPES.has(eventType)) {
    return "event-type-badge--create";
  }
  return "event-type-badge--default";
}

/**
 * Chart title for grouped activity views.
 * @param {{ viewType: string, splitBy: string }} options
 */
export function activityChartTitle({ viewType, splitBy }, $t) {
  const eventTypeSubject = $t("tools.activityViewer.subjectEventType");
  const userSubject = $t("general.user").toLowerCase();
  const outcomeSubject = $t("general.outcome");

  if (viewType === "pie") {
    if (splitBy === "outcome") {
      return $t("tools.activityViewer.activityBy", { subject: outcomeSubject });
    }
    return $t("tools.activityViewer.activityBy", {
      subject: splitBy === "user" ? userSubject : eventTypeSubject,
    });
  }
  if (viewType === "summary") {
    if (splitBy === "outcome") {
      return $t("tools.activityViewer.activityTotalsBy", { subject: outcomeSubject });
    }
    return splitBy === "user"
      ? $t("tools.activityViewer.activityTotalsBy", { subject: userSubject })
      : $t("tools.activityViewer.eventTotals");
  }
  if (splitBy === "user") {
    return $t("tools.activityViewer.activityOverTimeBy", { subject: userSubject });
  }
  if (splitBy === "outcome") {
    return $t("tools.activityViewer.activityOverTimeBy", { subject: outcomeSubject });
  }
  if (splitBy === "none") {
    return $t("tools.activityViewer.activityOverTime");
  }
  return $t("tools.activityViewer.activityOverTimeBy", { subject: eventTypeSubject });
}

/** Toggleable activity table columns (time, username, and event type are always shown). */
export const ACTIVITY_OPTIONAL_ROW_KEYS = [
  "source",
  "path",
  "shareHash",
  "tokenName",
  "details",
  "ipAddress",
  "status",
];

const OPTIONAL_ROW_KEY_SET = new Set(ACTIVITY_OPTIONAL_ROW_KEYS);

/** @returns {Record<string, boolean>} */
export function parseActivityRowsParam(rowsParam) {
  if (!rowsParam) {
    return defaultActivityOptionalRows();
  }
  const enabled = new Set();
  for (const part of String(rowsParam).split(",")) {
    const key = part.trim();
    if (OPTIONAL_ROW_KEY_SET.has(key)) {
      enabled.add(key);
    }
  }
  return Object.fromEntries(
    ACTIVITY_OPTIONAL_ROW_KEYS.map((key) => [key, enabled.has(key)]),
  );
}

/** @param {Record<string, boolean>} rowsState */
export function formatActivityRowsParam(rowsState) {
  const enabled = [];
  if (rowsState.source) enabled.push("source");
  if (rowsState.path) enabled.push("path");
  if (rowsState.shareHash) enabled.push("shareHash");
  if (rowsState.tokenName) enabled.push("tokenName");
  if (rowsState.details) enabled.push("details");
  if (rowsState.ipAddress) enabled.push("ipAddress");
  if (rowsState.status) enabled.push("status");
  return enabled.join(",");
}

/** Default toggle state: all optional table columns visible. */
export function defaultActivityOptionalRows() {
  return Object.fromEntries(ACTIVITY_OPTIONAL_ROW_KEYS.map((key) => [key, true]));
}

/** True when every optional column toggle is enabled. */
export function allActivityOptionalRowsEnabled(rowsState) {
  return (
    rowsState.source &&
    rowsState.path &&
    rowsState.shareHash &&
    rowsState.tokenName &&
    rowsState.details &&
    rowsState.ipAddress &&
    rowsState.status
  );
}

/** @param {Record<string, boolean>} rowsState */
export function optionalColumnsFromRowsState(rowsState) {
  return ACTIVITY_OPTIONAL_ROW_KEYS.filter((key) => isOptionalRowEnabled(rowsState, key));
}

function isOptionalRowEnabled(rowsState, key) {
  if (!OPTIONAL_ROW_KEY_SET.has(key)) {
    return false;
  }
  switch (key) {
    case "source":
      return !!rowsState.source;
    case "path":
      return !!rowsState.path;
    case "shareHash":
      return !!rowsState.shareHash;
    case "tokenName":
      return !!rowsState.tokenName;
    case "details":
      return !!rowsState.details;
    case "ipAddress":
      return !!rowsState.ipAddress;
    case "status":
      return !!rowsState.status;
    default:
      return false;
  }
}

/** @param {string[]} columns */
export function rowsStateFromOptionalColumns(columns) {
  const enabled = new Set(Array.isArray(columns) ? columns : []);
  return Object.fromEntries(
    ACTIVITY_OPTIONAL_ROW_KEYS.map((key) => [key, enabled.has(key)]),
  );
}

/** @param {string[]} columns */
export function allOptionalColumnsSelected(columns) {
  if (!Array.isArray(columns)) {
    return false;
  }
  return ACTIVITY_OPTIONAL_ROW_KEYS.every((key) => columns.includes(key));
}

/** @param {string[]} columns */
export function formatOptionalColumnsParam(columns) {
  if (allOptionalColumnsSelected(columns)) {
    return "";
  }
  return ACTIVITY_OPTIONAL_ROW_KEYS.filter((key) => columns.includes(key)).join(",");
}

/** @returns {string} */
export function activityRowSource(row) {
  return row?.source || row?.details?.source || "";
}

/** @returns {string} */
export function activityRowPath(row) {
  return row?.path || row?.details?.path || "";
}

/** @returns {string} */
export function activityRowShareHash(row) {
  return row?.shareHash || row?.details?.shareHash || "";
}

/** @returns {string} */
export function activityRowTokenName(row) {
  return row?.tokenName || row?.details?.tokenName || "";
}

/**
 * Normalize API rows so first-class fields are populated for table display and sorting.
 */
export function normalizeActivityTableRow(row) {
  if (!row) {
    return row;
  }
  return {
    ...row,
    source: activityRowSource(row),
    path: activityRowPath(row),
    shareHash: activityRowShareHash(row),
    tokenName: activityRowTokenName(row),
    authMethod: row.authMethod || row.details?.authMethod || "",
  };
}

/**
 * Human-readable token label for the activity table.
 */
export function activityTokenDisplay(row, $t) {
  const name = activityRowTokenName(row);
  const authMethod = row?.authMethod || row?.details?.authMethod || "";
  if (name) {
    if (authMethod === "webToken" && name.startsWith("WEB_TOKEN_")) {
      const suffix = name.slice("WEB_TOKEN_".length);
      return $t("tools.activityViewer.webSessionNamed", { name: suffix });
    }
    return name;
  }
  if (authMethod === "webToken") {
    return $t("tools.activityViewer.webSession");
  }
  return "";
}

/** @returns {string} */
function formatActivityFieldChange(change, eventType) {
  const fromRaw = change?.from;
  const toRaw = change?.to;
  const hasFrom = fromRaw !== null && fromRaw !== undefined && fromRaw !== "";
  const hasTo = toRaw !== null && toRaw !== undefined && toRaw !== "";

  if (eventType === "accessCreate" || eventType === "accessDelete") {
    if (hasTo) {
      return String(toRaw);
    }
    return "—";
  }

  if (eventType === "accessUpdate") {
    const from = hasFrom ? String(fromRaw) : "—";
    const to = hasTo ? String(toRaw) : "—";
    if (hasFrom || hasTo) {
      return `${from} → ${to}`;
    }
    return "—";
  }

  const from = hasFrom ? String(fromRaw) : "—";
  const to = hasTo ? String(toRaw) : "—";
  if (hasFrom || hasTo) {
    return `${from} → ${to}`;
  }
  return "—";
}

/**
 * Build label/value rows for activity detail tooltips (admin).
 * @returns {{ id: string, label: string, value: string }[]}
 */
export function buildActivityDetailRows(row, $t) {
  if (!row) return [];

  const d = row.details || {};
  const rows = [];

  const targetUser = d.targetUsername;
  if (targetUser) {
    rows.push({
      id: "targetUser",
      label: $t("general.user"),
      value: targetUser,
    });
  }

  if (Array.isArray(d.scopes) && d.scopes.length > 0) {
    const scopeLines = d.scopes.map((s) => {
      const source = s.source || "";
      const path = s.path || "/";
      return path && path !== "/" ? `${source}: ${path}` : source;
    });
    rows.push({
      id: "scopes",
      label: $t("general.sources"),
      value: scopeLines.join("\n"),
    });
  }

  if (Array.isArray(d.changes) && d.changes.length > 0) {
    for (const change of d.changes) {
      const field = change?.field || "";
      if (!field || field === "hash") continue;
      rows.push({
        id: `change-${field}`,
        label: field,
        value: formatActivityFieldChange(change, row.eventType),
      });
    }
  }

  if (d.affectedTokenName) {
    rows.push({
      id: "affectedToken",
      label: $t("tools.activityViewer.affectedToken"),
      value: d.affectedTokenName,
    });
  }

  const tokenName = activityRowTokenName(row);
  if (tokenName && !row?.tokenName) {
    rows.push({
      id: "token",
      label: $t("tools.activityViewer.actorToken"),
      value: tokenName,
    });
  }
  if (d.loginMethod) {
    rows.push({ id: "loginMethod", label: $t("settings.loginMethod"), value: d.loginMethod });
  }
  if (d.passkeyName) {
    rows.push({ id: "passkey", label: $t("profileSettings.passkeyDefaultName"), value: d.passkeyName });
  }
  if (d.cached) {
    rows.push({ id: "cached", label: $t("tools.activityViewer.cachedResult"), value: $t("general.yes") });
  }

  const paths = Array.isArray(d.paths) ? d.paths : [];
  if (paths.length > 1) {
    rows.push({
      id: "paths",
      label: $t("general.paths"),
      value: paths.join("\n"),
    });
  }

  if (d.targetPath || row.targetPath) {
    rows.push({
      id: "targetPath",
      label: $t("general.path"),
      value: d.targetPath || row.targetPath,
    });
  }
  if (d.fileCount > 1 && paths.length <= 1) {
    rows.push({ id: "fileCount", label: $t("general.files"), value: String(d.fileCount) });
  }
  if (d.bytes > 0) {
    rows.push({ id: "bytes", label: $t("general.size"), value: String(d.bytes) });
  }
  if (d.durationMs > 0) {
    rows.push({
      id: "durationMs",
      label: $t("files.duration"),
      value: `${d.durationMs} ms`,
    });
  }
  if (d.error) {
    rows.push({ id: "error", label: $t("general.error"), value: d.error });
  }

  return rows;
}

/**
 * Compact badges for the table details column (admin).
 * @returns {{ id: string, text: string }[]}
 */
export function buildActivityDetailBadges(row) {
  if (!row) return [];

  const d = row.details || {};
  const badges = [];

  const targetUser = d.targetUsername;
  if (targetUser) {
    badges.push({ id: "targetUser", text: targetUser });
  }

  const tokenName = activityRowTokenName(row);
  if (tokenName && !row?.tokenName) {
    badges.push({ id: "token", text: tokenName });
  }
  if (d.affectedTokenName) {
    badges.push({ id: "affectedToken", text: d.affectedTokenName });
  }
  if (d.loginMethod) {
    badges.push({ id: "loginMethod", text: d.loginMethod });
  }
  if (d.passkeyName) {
    badges.push({ id: "passkey", text: d.passkeyName });
  }

  if (Array.isArray(d.changes) && d.changes.length > 0) {
    for (const change of d.changes) {
      const field = change?.field || "";
      if (!field || field === "hash") continue;
      badges.push({
        id: `change-${field}`,
        text: `${field}: ${formatActivityFieldChange(change, row.eventType)}`,
      });
    }
  }

  const paths = Array.isArray(d.paths) ? d.paths : [];
  if (paths.length > 1) {
    badges.push({
      id: "paths",
      text: `${paths.length} paths`,
    });
  }

  if (d.targetPath) {
    badges.push({ id: "target", text: d.targetPath });
  }

  return badges;
}

export function hasActivityDetails(row) {
  return buildActivityDetailRows(row, (key) => key).length > 0;
}
