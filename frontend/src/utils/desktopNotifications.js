import i18n from "@/i18n";
import { state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { url } from "@/utils";

const STORAGE_PREFIX = "desktopNotifications_";

function storageKey() {
  const username = state.user?.username;
  if (!username || username === "anonymous") {
    return `${STORAGE_PREFIX}anonymous`;
  }
  return `${STORAGE_PREFIX}${url.base64Encode(username)}`;
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

export function isDesktopNotificationsEnabled() {
  try {
    return localStorage.getItem(storageKey()) === "true";
  } catch {
    return false;
  }
}

export function setDesktopNotificationsEnabled(enabled) {
  try {
    localStorage.setItem(storageKey(), enabled ? "true" : "false");
  } catch {
    // ignore — e.g. private browsing quota
  }
}

function notificationIcon() {
  const base = globalVars.baseURL || "/";
  return `${window.location.origin}${base}public/static/icons/pwa-icon-192.png`;
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
  return isDesktopNotificationsEnabled();
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
    body = t("desktopNotifications.uploadBody", {
      name,
      size: getHumanReadableFilesize(upload.size),
    });
  }
  showNotification(
    t("desktopNotifications.uploadTitle"),
    body,
    `upload-${upload.id ?? name}`
  );
}

export function notifyUploadError(name, errorDetails) {
  const t = i18n.global.t;
  showNotification(
    t("desktopNotifications.uploadFailedTitle"),
    t("desktopNotifications.errorBody", {
      name: name || t("general.file", { suffix: "" }),
      error: errorDetails || t("prompts.operationFailed"),
    }),
    `upload-error-${name}`
  );
}

export function notifyDownloadComplete(name, size) {
  const t = i18n.global.t;
  const body =
    size > 0
      ? t("desktopNotifications.downloadBody", {
          name,
          size: getHumanReadableFilesize(size),
        })
      : name;
  showNotification(
    t("desktopNotifications.downloadTitle"),
    body,
    `download-${name}`
  );
}

export function notifyDownloadError(name, errorDetails) {
  const t = i18n.global.t;
  showNotification(
    t("desktopNotifications.downloadFailedTitle"),
    t("desktopNotifications.errorBody", {
      name: name || t("general.file", { suffix: "" }),
      error: errorDetails || t("prompts.operationFailed"),
    }),
    `download-error-${name}`
  );
}

export function notifyMoveCopyComplete(operation, itemCount) {
  const t = i18n.global.t;
  let title;
  let body;

  if (operation === "move") {
    title = t("desktopNotifications.moveTitle");
    if (itemCount > 1) {
      body = t("desktopNotifications.moveBodyMultiple", { count: itemCount });
    } else {
      body = t("desktopNotifications.moveBodySingle");
    }
  } else {
    title = t("desktopNotifications.copyTitle");
    if (itemCount > 1) {
      body = t("desktopNotifications.copyBodyMultiple", { count: itemCount });
    } else {
      body = t("desktopNotifications.copyBodySingle");
    }
  }

  showNotification(title, body, `${operation}-done`);
}

export function notifyOperationError(message) {
  const t = i18n.global.t;
  showNotification(
    t("desktopNotifications.operationFailedTitle"),
    message || t("prompts.operationFailed"),
    "operation-error"
  );
}
