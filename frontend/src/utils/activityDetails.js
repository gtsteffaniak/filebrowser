function activitySharedI18nKey(eventType) {
  switch (eventType) {
    case "download":
      return "general.download";
    case "move":
      return "general.move";
    case "copy":
      return "general.copy";
    case "rename":
      return "general.rename";
    case "upload":
      return "general.upload";
    case "delete":
      return "general.delete";
    case "login":
      return "general.login";
    case "logout":
      return "general.logout";
    case "signup":
      return "general.signup";
    case "archive":
      return "fileTypes.archive";
    case "duplicateFinder":
      return "tools.duplicateFinder.name";
    case "shareDownload":
      return "general.download";
    default:
      return "";
  }
}

/**
 * Label for an activity event type, preferring shared i18n keys when available.
 */
export function activityEventLabel(eventType, $t) {
  const sharedKey = activitySharedI18nKey(eventType);
  if (sharedKey) {
    return $t(sharedKey);
  }
  return $t(`tools.activityViewer.events.${eventType}`, eventType);
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

  if (d.tokenName) {
    rows.push({
      id: "token",
      label: $t("prompts.token"),
      value: d.tokenName,
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

  const source = d.source;
  const paths = Array.isArray(d.paths) && d.paths.length > 0
    ? d.paths
    : (d.path ? [d.path] : []);

  if (source) {
    rows.push({ id: "source", label: $t("general.source"), value: source });
  }

  if (paths.length === 1) {
    rows.push({ id: "path", label: $t("general.path"), value: paths[0] });
  } else if (paths.length > 1) {
    rows.push({
      id: "paths",
      label: $t("general.paths"),
      value: paths.join("\n"),
    });
  }

  if (d.targetPath) {
    rows.push({ id: "targetPath", label: $t("general.path"), value: d.targetPath });
  }
  if (d.shareHash) {
    rows.push({ id: "shareHash", label: $t("onlyoffice.shareHash"), value: d.shareHash });
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
export function buildActivityDetailBadges(row, $t) {
  if (!row) return [];

  const d = row.details || {};
  const badges = [];

  const targetUser = d.targetUsername;
  if (targetUser) {
    badges.push({ id: "targetUser", text: targetUser });
  }

  if (d.tokenName) {
    badges.push({ id: "token", text: d.tokenName });
  }
  if (d.loginMethod) {
    badges.push({ id: "loginMethod", text: d.loginMethod });
  }
  if (d.passkeyName) {
    badges.push({ id: "passkey", text: d.passkeyName });
  }

  const source = d.source;
  const paths = Array.isArray(d.paths) && d.paths.length > 0
    ? d.paths
    : (d.path ? [d.path] : []);

  if (source) {
    badges.push({ id: "source", text: source });
  }

  if (paths.length === 1) {
    badges.push({ id: "path", text: paths[0] });
  } else if (paths.length > 1) {
    badges.push({
      id: "paths",
      text: `${paths.length} paths`,
    });
  }

  if (d.shareHash) {
    const hash = d.shareHash.length > 12 ? `${d.shareHash.slice(0, 10)}…` : d.shareHash;
    badges.push({
      id: "shareHash",
      text: `${$t("general.share", { suffix: "" })} ${hash}`,
    });
  }
  if (d.targetPath) {
    badges.push({ id: "target", text: d.targetPath });
  }

  return badges;
}

export function hasActivityDetails(row) {
  if (!row) return false;
  return Boolean(row.details && Object.keys(row.details).length > 0);
}
