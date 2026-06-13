import i18n from "@/i18n";
import { state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { url } from "@/utils";

const STORAGE_PREFIX = "appNotifications_";
const LEGACY_STORAGE_PREFIX = "desktopNotifications_";

function storageKey() {
  const username = state.user?.username;
  if (!username || username === "anonymous") {
    return `${STORAGE_PREFIX}anonymous`;
  }
  return `${STORAGE_PREFIX}${url.base64Encode(username)}`;
}

function legacyStorageKey() {
  const username = state.user?.username;
  if (!username || username === "anonymous") {
    return `${LEGACY_STORAGE_PREFIX}anonymous`;
  }
  return `${LEGACY_STORAGE_PREFIX}${url.base64Encode(username)}`;
}

export function isNotificationSupported() {
  return typeof window !== "undefined" && "Notification" in window;
}

export function getNotificationPermission() {
  if (!isNotificationSupported()) {
    return "unsupported";
  }
  return Notification.permission;
}

export async function requestNotificationPermission() {
  if (!isNotificationSupported()) {
    return "denied";
  }
  if (Notification.permission === "granted") {
    return "granted";
  }
  if (Notification.permission === "denied") {
    return "denied";
  }
  return Notification.requestPermission();
}

export function isAppNotificationsEnabled() {
  try {
    const key = storageKey();
    const value = localStorage.getItem(key);
    if (value !== null) {
      return value === "true";
    }
    return localStorage.getItem(legacyStorageKey()) === "true";
  } catch {
    return false;
  }
}

export function setAppNotificationsEnabled(enabled) {
  try {
    localStorage.setItem(storageKey(), enabled ? "true" : "false");
  } catch {
    // ignore — e.g. private browsing quota
  }
}

function notificationIcon() {
  const base = globalVars.baseURL || "/";
  const normalizedBase = base.endsWith("/") ? base : `${base}/`;
  return new URL(`${normalizedBase}public/static/icons/pwa-icon-192.png`, window.location.origin).toString();
}

export function formatNotificationCount(count) {
  const title = i18n.global.t("notifications.title");
  return `${count} ${title.toLowerCase()}`;
}

function shouldNotify() {
  if (!isNotificationSupported()) {
    return false;
  }
  if (Notification.permission !== "granted") {
    return false;
  }
  if (!document.hidden) {
    return false;
  }
  return isAppNotificationsEnabled();
}

function showNotification(title, body, tag) {
  if (!shouldNotify()) {
    return;
  }
  try {
    new Notification(title, {
      body,
      icon: notificationIcon(),
      tag,
    });
  } catch {
    // ignore — e.g. insecure context or blocked notifications
  }
}

export function notifyUploadComplete(upload) {
  const t = i18n.global.t;
  const name = upload.name || upload.path?.split("/").pop() || t("general.file", { suffix: "" });
  let body;
  if (upload.type === "directory" || !upload.size) {
    body = name;
  } else {
    body = `${name} (${getHumanReadableFilesize(upload.size)})`;
  }
  showNotification(
    t("notifications.uploadTitle"),
    body,
    `upload-${upload.id ?? name}`
  );
}

export function notifyUploadError(name, errorDetails) {
  const t = i18n.global.t;
  const fileName = name || t("general.file", { suffix: "" });
  const error = errorDetails || t("prompts.operationFailed");
  showNotification(
    t("notifications.uploadFailedTitle"),
    `${fileName}: ${error}`,
    `upload-error-${name}`
  );
}

export function notifyDownloadComplete(name, size) {
  const t = i18n.global.t;
  const body =
    size > 0 ? `${name} (${getHumanReadableFilesize(size)})` : name;
  showNotification(
    t("notifications.downloadTitle"),
    body,
    `download-${name}`
  );
}

export function notifyDownloadError(name, errorDetails) {
  const t = i18n.global.t;
  const fileName = name || t("general.file", { suffix: "" });
  const error = errorDetails || t("prompts.operationFailed");
  showNotification(
    t("notifications.downloadFailedTitle"),
    `${fileName}: ${error}`,
    `download-error-${name}`
  );
}

export function notifyMoveCopyComplete(operation, itemCount) {
  const t = i18n.global.t;
  if (operation === "move") {
    const title = t("notifications.moveTitle");
    showNotification(title, `${itemCount} ${title.toLowerCase()}`, "move-done");
  } else {
    const title = t("notifications.copyTitle");
    showNotification(title, `${itemCount} ${title.toLowerCase()}`, "copy-done");
  }
}

export function notifyOperationError(message) {
  const t = i18n.global.t;
  showNotification(
    t("notifications.operationFailedTitle"),
    message || t("prompts.operationFailed"),
    "operation-error"
  );
}
