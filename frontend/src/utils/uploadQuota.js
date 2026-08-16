import { fetchJSON } from "@/api/utils";
import { getters, mutations, state } from "@/store";
import { notify } from "@/notify";
import i18n from "@/i18n";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { notifyOperationError } from "@/utils/appNotifications";

/**
 * @param {{ file?: File; relativePath?: string }[]} items
 */
export function sumUploadItemBytes(items) {
  return items.reduce((sum, item) => sum + (item.file?.size ?? 0), 0);
}

/**
 * @param {string} path
 */
function normalizeQuotaPath(path) {
  if (!path || path === "/") {
    return "/";
  }
  const trimmed = path.replace(/\/+$/, "");
  return trimmed || "/";
}

/**
 * @param {{ usedBytes?: number; reservedBytes?: number; limitBytes?: number }} snapshot
 * @param {number} uploadBytes
 */
function wouldExceedQuota(snapshot, uploadBytes) {
  const limit = snapshot.limitBytes ?? 0;
  if (limit <= 0) {
    return false;
  }
  const used = snapshot.usedBytes ?? 0;
  const reserved = snapshot.reservedBytes ?? 0;
  return used + reserved + uploadBytes > limit;
}

/**
 * @param {string} source
 * @param {string} destinationPath
 * @returns {Promise<{ usedBytes?: number; reservedBytes?: number; limitBytes?: number } | null>}
 */
async function fetchFolderQuotaSnapshot(source, destinationPath) {
  if (!source) {
    return null;
  }
  const path = normalizeQuotaPath(destinationPath);
  try {
    const params = { source };
    if (path !== "/") {
      params.path = path;
    }
    const raw = await fetchJSON(getApiPath("quotas", params));
    const data = Array.isArray(raw) ? raw[0] : raw;
    if (!data?.limitBytes || data.limitBytes <= 0) {
      return null;
    }
    return {
      usedBytes: data.usedBytes,
      reservedBytes: data.reservedBytes,
      limitBytes: data.limitBytes,
    };
  } catch {
    return null;
  }
}

/**
 * Folder quotas applicable to this share root (from share/info).
 * @returns {{ usedBytes?: number; reservedBytes?: number; limitBytes?: number }[]}
 */
function getShareFolderQuotaSnapshots() {
  if (!getters.isShare()) {
    return [];
  }
  /** @type {{ folderQuotas?: { limitBytes?: number; usedBytes?: number; reservedBytes?: number }[] }} */
  const share = state.shareInfo || {};
  const quotas = share.folderQuotas;
  if (!Array.isArray(quotas)) {
    return [];
  }
  return quotas.filter((q) => (q.limitBytes ?? 0) > 0);
}

/**
 * @returns {{ usedBytes?: number; reservedBytes?: number; limitBytes?: number } | null}
 */
function getShareLinkQuotaSnapshot() {
  if (!getters.isShare()) {
    return null;
  }
  /** @type {{ quotaLimitBytes?: number; quotaUsedBytes?: number; quotaAvailableBytes?: number }} */
  const share = state.shareInfo || {};
  const limit = share.quotaLimitBytes ?? 0;
  if (limit <= 0) {
    return null;
  }
  const used = share.quotaUsedBytes ?? 0;
  const available = share.quotaAvailableBytes;
  const reserved =
    typeof available === "number"
      ? Math.max(0, limit - used - available)
      : 0;
  return {
    limitBytes: limit,
    usedBytes: used,
    reservedBytes: reserved,
  };
}

/**
 * Refresh share quota fields when the page was loaded before folderQuotas existed.
 * @returns {Promise<void>}
 */
async function refreshShareQuotaSnapshotsIfNeeded() {
  if (!getters.isShare()) {
    return;
  }
  const share = state.shareInfo || {};
  if (Array.isArray(share.folderQuotas) && share.folderQuotas.length > 0) {
    return;
  }
  const hash = share.hash;
  if (!hash) {
    return;
  }
  try {
    const response = await fetch(getPublicApiPath("share/info", { hash }));
    if (!response.ok) {
      return;
    }
    const info = await response.json();
    if (!info) {
      return;
    }
    mutations.setShareInfo({
      folderQuotas: info.folderQuotas,
      quotaLimitBytes: info.quotaLimitBytes,
      quotaUsedBytes: info.quotaUsedBytes,
      quotaAvailableBytes: info.quotaAvailableBytes,
    });
  } catch {
    // Best-effort; uploads proceed when quota data is unavailable.
  }
}

/**
 * @param {string} destinationPath
 * @returns {Promise<{ usedBytes?: number; reservedBytes?: number; limitBytes?: number }[]>}
 */
async function resolveFolderQuotaSnapshots(destinationPath) {
  if (getters.isShare()) {
    await refreshShareQuotaSnapshotsIfNeeded();
  }

  const shareFolderQuotas = getShareFolderQuotaSnapshots();
  if (shareFolderQuotas.length > 0) {
    return shareFolderQuotas;
  }

  if (getters.isShare()) {
    return [];
  }

  const source = state.req?.source;
  if (!source) {
    return [];
  }

  const snap = await fetchFolderQuotaSnapshot(source, destinationPath);
  return snap ? [snap] : [];
}

/**
 * @param {unknown} err
 */
export function parseUploadErrorPayload(err) {
  /** @type {{ response?: { responseText?: string } }} */
  const wrapped = err;
  const text = wrapped?.response?.responseText || "";
  const start = text.indexOf("{");
  if (start < 0) {
    return null;
  }
  try {
    return JSON.parse(text.slice(start));
  } catch {
    return null;
  }
}

/**
 * @param {unknown} err
 */
export function isQuotaExceededError(err) {
  /** @type {{ status?: number; response?: { status?: number }; message?: string }} */
  const wrapped = err;
  const status = wrapped?.status ?? wrapped?.response?.status;
  if (status === 507) {
    return true;
  }
  const payload = parseUploadErrorPayload(err);
  if (payload?.code === "quota_exceeded") {
    return true;
  }
  const message = String(wrapped?.message || payload?.message || "").toLowerCase();
  return message.includes("quota exceeded");
}

/**
 * @param {unknown} err
 */
export function extractQuotaErrorMessage(err) {
  const t = i18n.global.t;
  const payload = parseUploadErrorPayload(err);
  if (payload?.code === "quota_exceeded") {
    return t("quotas.errors.exceeded");
  }
  if (typeof payload?.message === "string" && payload.message.trim()) {
    const lower = payload.message.toLowerCase();
    if (lower.includes("quota exceeded")) {
      return t("quotas.errors.exceeded");
    }
    return payload.message;
  }
  /** @type {{ message?: string }} */
  const wrapped = err;
  if (typeof wrapped?.message === "string") {
    const lower = wrapped.message.toLowerCase();
    if (lower.includes("quota exceeded")) {
      return t("quotas.errors.exceeded");
    }
  }
  return t("quotas.errors.exceeded");
}

/**
 * Block upload when known batch size exceeds folder or share quota.
 * @param {string} destinationPath
 * @param {{ file?: File; relativePath?: string }[]} items
 * @returns {Promise<boolean>} true when upload was rejected
 */
export async function rejectUploadIfQuotaExceeded(destinationPath, items) {
  const uploadBytes = sumUploadItemBytes(items);
  if (uploadBytes <= 0) {
    return false;
  }

  const folderSnapshots = await resolveFolderQuotaSnapshots(destinationPath);
  for (const snapshot of folderSnapshots) {
    if (wouldExceedQuota(snapshot, uploadBytes)) {
      notifyQuotaExceeded();
      return true;
    }
  }

  const shareSnapshot = getShareLinkQuotaSnapshot();
  if (shareSnapshot && wouldExceedQuota(shareSnapshot, uploadBytes)) {
    notifyQuotaExceeded();
    return true;
  }

  return false;
}

function notifyQuotaExceeded(message) {
  const text = message || i18n.global.t("quotas.errors.exceeded");
  notify.showError(text);
  notifyOperationError(text);
}
