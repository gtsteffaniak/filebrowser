import { bytesFromCustomAmount, customAmountFromBytes } from "@/utils/quotaUnits";

export function emptyShareDefaultsEnforced() {
  return {
    shareTheme: false,
    disableAnonymous: false,
    disableThumbnails: false,
    keepAfterExpiration: false,
    themeColor: false,
    title: false,
    description: false,
    favicon: false,
    quickDownload: false,
    hideNavButtons: false,
    disableSidebar: false,
    disableShareCard: false,
    enforceDarkLightMode: false,
    viewMode: false,
    enableOnlyOffice: false,
    shareType: false,
    allowDelete: false,
    allowCreate: false,
    allowModify: false,
    disableFileViewer: false,
    disableDownload: false,
    allowReplacements: false,
    sidebarLinks: false,
    showHidden: false,
    disableLoginOption: false,
    maxBandwidth: false,
    allowedUsernames: false,
    perUserDownloadLimit: false,
    extractEmbeddedSubtitles: false,
    downloadsLimit: false,
    quotaLimitBytes: false,
    hideFileExt: false,
    banner: false,
  };
}

export function emptyShareDefaultsValues() {
  return {
    shareTheme: "default",
    disableAnonymous: false,
    disableDownload: false,
    allowModify: false,
    allowDelete: false,
    allowCreate: false,
    allowReplacements: false,
    downloadsLimit: 0,
    perUserDownloadLimit: false,
    shareType: "normal",
    disableFileViewer: false,
    disableThumbnails: false,
    showHidden: false,
    hideFileExt: "",
    enableAllowedUsernames: false,
    allowedUsernames: "",
    keepAfterExpiration: false,
    themeColor: "",
    banner: "",
    title: "",
    description: "",
    favicon: "",
    quickDownload: false,
    hideNavButtons: false,
    disableShareCard: false,
    disableSidebar: false,
    enforceDarkLightMode: "default",
    viewMode: "normal",
    enableOnlyOffice: false,
    extractEmbeddedSubtitles: false,
    disableLoginOption: false,
    sidebarLinks: [],
    quotaEnabled: false,
    quotaCustomAmount: 10,
    quotaCustomUnit: "gb",
    maxBandwidth: 0,
  };
}

export function shareDefaultsFromApi(values = {}) {
  const out = { ...emptyShareDefaultsValues(), ...values };
  const quota = values.quotaLimitBytes || 0;
  out.quotaEnabled = quota > 0;
  if (quota > 0) {
    const custom = customAmountFromBytes(quota);
    out.quotaCustomAmount = custom.amount;
    out.quotaCustomUnit = custom.unit;
  }
  out.enableAllowedUsernames = Array.isArray(values.allowedUsernames) && values.allowedUsernames.length > 0;
  out.allowedUsernames = out.enableAllowedUsernames ? values.allowedUsernames.join(", ") : "";
  out.downloadsLimit = values.downloadsLimit || 0;
  out.maxBandwidth = values.maxBandwidth || 0;
  if (!out.shareType) {
    out.shareType = "normal";
  }
  if (!out.shareTheme) {
    out.shareTheme = "default";
  }
  if (!out.enforceDarkLightMode) {
    out.enforceDarkLightMode = "default";
  }
  if (!out.viewMode) {
    out.viewMode = "normal";
  }
  if (!Array.isArray(out.sidebarLinks)) {
    out.sidebarLinks = [];
  }
  return out;
}

export function shareDefaultsToApiPayload(form) {
  return {
    shareTheme: form.shareTheme,
    disableAnonymous: form.disableAnonymous,
    disableDownload: form.disableDownload,
    allowModify: form.allowModify,
    allowDelete: form.allowDelete,
    allowCreate: form.allowCreate,
    allowReplacements: form.allowReplacements,
    downloadsLimit: form.downloadsLimit ? Number(form.downloadsLimit) : 0,
    perUserDownloadLimit: form.perUserDownloadLimit,
    maxBandwidth: form.maxBandwidth ? Number(form.maxBandwidth) : 0,
    shareType: form.shareType,
    disableFileViewer: form.disableFileViewer,
    disableThumbnails: form.disableThumbnails,
    showHidden: form.showHidden,
    hideFileExt: form.hideFileExt || "",
    allowedUsernames: form.enableAllowedUsernames
      ? String(form.allowedUsernames || "").split(",").map((u) => u.trim()).filter(Boolean)
      : [],
    keepAfterExpiration: form.keepAfterExpiration,
    themeColor: form.themeColor || "",
    banner: form.banner || "",
    title: form.title || "",
    description: form.description || "",
    favicon: form.favicon || "",
    quickDownload: form.quickDownload,
    hideNavButtons: form.hideNavButtons,
    disableShareCard: form.disableShareCard,
    disableSidebar: form.disableSidebar,
    enforceDarkLightMode: form.enforceDarkLightMode,
    viewMode: form.viewMode,
    enableOnlyOffice: form.enableOnlyOffice,
    extractEmbeddedSubtitles: form.extractEmbeddedSubtitles,
    disableLoginOption: form.disableLoginOption,
    sidebarLinks: Array.isArray(form.sidebarLinks) ? form.sidebarLinks : [],
    quotaLimitBytes: form.quotaEnabled
      ? bytesFromCustomAmount(form.quotaCustomAmount, form.quotaCustomUnit)
      : 0,
  };
}

const DEFAULT_SIDEBAR_LINKS = [
  {
    name: "Share QR Code and Info",
    category: "shareInfo",
    target: "#",
    icon: "qr_code",
  },
  {
    name: "Download",
    category: "download",
    target: "#",
    icon: "download",
  },
];

export function defaultSidebarLinksForShareType(shareType) {
  if (shareType === "upload") {
    return [DEFAULT_SIDEBAR_LINKS[0]];
  }
  return DEFAULT_SIDEBAR_LINKS.map((link) => ({ ...link }));
}

/**
 * Apply admin share defaults onto a share prompt form object (mutates target).
 */
export function applyShareDefaultsToForm(target, values, { itemName, titleDefault, descriptionDefault } = {}) {
  if (!values || !target) {
    return;
  }
  const mapped = shareDefaultsFromApi(values);
  Object.assign(target, {
    shareTheme: mapped.shareTheme,
    disableAnonymous: mapped.disableAnonymous,
    disableDownload: mapped.disableDownload,
    allowModify: mapped.allowModify,
    allowDelete: mapped.allowDelete,
    allowCreate: mapped.allowCreate,
    allowReplacements: mapped.allowReplacements,
    downloadsLimit: mapped.downloadsLimit ? String(mapped.downloadsLimit) : "",
    perUserDownloadLimit: mapped.perUserDownloadLimit,
    maxBandwidth: mapped.maxBandwidth ? String(mapped.maxBandwidth) : "",
    shareType: mapped.shareType,
    disableFileViewer: mapped.disableFileViewer,
    disableThumbnails: mapped.disableThumbnails,
    showHidden: mapped.showHidden,
    hideFileExt: mapped.hideFileExt,
    enableAllowedUsernames: mapped.enableAllowedUsernames,
    allowedUsernames: mapped.allowedUsernames,
    keepAfterExpiration: mapped.keepAfterExpiration,
    themeColor: mapped.themeColor,
    banner: mapped.banner,
    favicon: mapped.favicon,
    quickDownload: mapped.quickDownload,
    hideNavButtons: mapped.hideNavButtons,
    disableShareCard: mapped.disableShareCard,
    disableSidebar: mapped.disableSidebar,
    enforceDarkLightMode: mapped.enforceDarkLightMode,
    viewMode: mapped.viewMode,
    enableOnlyOffice: mapped.enableOnlyOffice,
    extractEmbeddedSubtitles: mapped.extractEmbeddedSubtitles,
    disableLoginOption: mapped.disableLoginOption,
    quotaEnabled: mapped.quotaEnabled,
    quotaCustomAmount: mapped.quotaCustomAmount,
    quotaCustomUnit: mapped.quotaCustomUnit,
    title: mapped.title || titleDefault || "",
    description: mapped.description || descriptionDefault || "",
    sidebarLinks: Array.isArray(mapped.sidebarLinks) && mapped.sidebarLinks.length > 0
      ? mapped.sidebarLinks.map((link) => ({ ...link }))
      : defaultSidebarLinksForShareType(mapped.shareType),
  });
}
