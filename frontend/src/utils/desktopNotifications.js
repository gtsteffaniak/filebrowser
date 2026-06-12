import i18n from "@/i18n";
import { state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";

const EVENT = {
  UPLOAD: "upload",
  DOWNLOAD: "download",
  MOVE_COPY: "moveCopy",
  ERROR: "errors",
};

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

export function defaultDesktopNotificationSettings() {
  return {
    enabled: false,
    upload: true,
    download: true,
    moveCopy: true,
    errors: true,
  };
}

function getSettings() {
  const prefs = state.user?.desktopNotifications ?? {};
  const defaults = defaultDesktopNotificationSettings();
  return {
    enabled: prefs.enabled === true,
    upload: prefs.upload ?? defaults.upload,
    download: prefs.download ?? defaults.download,
    moveCopy: prefs.moveCopy ?? defaults.moveCopy,
    errors: prefs.errors ?? defaults.errors,
  };
}

function notificationIcon() {
  const base = globalVars.baseURL || "/";
  return `${window.location.origin}${base}public/static/icons/pwa-icon-192.png`;
}

function shouldNotify(eventType) {
  if (!isNotificationSupported()) {
    return false;
  }
  if (Notification.permission !== "granted") {
    return false;
  }
  if (!document.hidden) {
    return false;
  }

  const settings = getSettings();
  if (!settings.enabled) {
    return false;
  }

  switch (eventType) {
    case EVENT.UPLOAD:
      return settings.upload;
    case EVENT.DOWNLOAD:
      return settings.download;
    case EVENT.MOVE_COPY:
      return settings.moveCopy;
    case EVENT.ERROR:
      return settings.errors;
    default:
      return false;
  }
}

function showNotification(eventType, title, body, tag) {
  if (!shouldNotify(eventType)) {
    return;
  }
  try {
    new Notification(title, {
      body,
      icon: notificationIcon(),
      tag: tag || eventType,
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
    EVENT.UPLOAD,
    t("desktopNotifications.uploadTitle"),
    body,
    `upload-${upload.id ?? name}`
  );
}

export function notifyUploadError(name, errorDetails) {
  const t = i18n.global.t;
  showNotification(
    EVENT.ERROR,
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
    EVENT.DOWNLOAD,
    t("desktopNotifications.downloadTitle"),
    body,
    `download-${name}`
  );
}

export function notifyDownloadError(name, errorDetails) {
  const t = i18n.global.t;
  showNotification(
    EVENT.ERROR,
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

  showNotification(EVENT.MOVE_COPY, title, body, `${operation}-done`);
}

export function notifyOperationError(message) {
  const t = i18n.global.t;
  showNotification(
    EVENT.ERROR,
    t("desktopNotifications.operationFailedTitle"),
    message || t("prompts.operationFailed"),
    "operation-error"
  );
}
