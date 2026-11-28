import { bytesFromCustomAmount, customAmountFromBytes } from "@/utils/quotaUnits.js";
import type {
  ShareDefaults,
  ShareDefaultsEnforcement,
  ShareDefaultsForm,
  ShareFormTarget,
  SidebarLink,
} from "@/types/settings.ts";

export function emptyShareDefaultsEnforced(): ShareDefaultsEnforcement {
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
    downloadsLimit: false,
    quotaLimitBytes: false,
    hideFileExt: false,
    banner: false,
  };
}

export function emptyShareDefaultsValues(): ShareDefaultsForm {
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
    disableLoginOption: false,
    sidebarLinks: [],
    quotaEnabled: false,
    quotaCustomAmount: 10,
    quotaCustomUnit: "gb",
    maxBandwidth: 0,
  };
}

export function shareDefaultsFromApi(values: ShareDefaults = {}): ShareDefaultsForm {
  const { allowedUsernames, ...rest } = values;
  const out: ShareDefaultsForm = { ...emptyShareDefaultsValues(), ...rest };
  const quota = values.quotaLimitBytes || 0;
  out.quotaEnabled = quota > 0;
  if (quota > 0) {
    const custom = customAmountFromBytes(quota);
    out.quotaCustomAmount = custom.amount;
    out.quotaCustomUnit = custom.unit;
  }
  out.enableAllowedUsernames = Array.isArray(allowedUsernames) && allowedUsernames.length > 0;
  out.allowedUsernames = out.enableAllowedUsernames ? (allowedUsernames ?? []).join(", ") : "";
  out.downloadsLimit = values.downloadsLimit || 0;
  out.maxBandwidth = values.maxBandwidth || 0;
  out.shareType ||= "normal";
  out.shareTheme ||= "default";
  out.enforceDarkLightMode ||= "default";
  out.viewMode ||= "normal";
  if (!Array.isArray(out.sidebarLinks)) {
    out.sidebarLinks = [];
  }
  return out;
}

export function shareDefaultsToApiPayload(form: ShareDefaultsForm): ShareDefaults {
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
      ? (form.allowedUsernames || "").split(",").map((u) => u.trim()).filter(Boolean)
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
    disableLoginOption: form.disableLoginOption,
    sidebarLinks: Array.isArray(form.sidebarLinks) ? form.sidebarLinks : [],
    quotaLimitBytes: form.quotaEnabled
      ? bytesFromCustomAmount(form.quotaCustomAmount, form.quotaCustomUnit)
      : 0,
  };
}

const SHARE_INFO_LINK: SidebarLink = {
  name: "Share QR Code and Info",
  category: "shareInfo",
  target: "#",
  icon: "qr_code",
};

const DOWNLOAD_LINK: SidebarLink = {
  name: "Download",
  category: "download",
  target: "#",
  icon: "download",
};

export function defaultSidebarLinksForShareType(shareType?: string): SidebarLink[] {
  const links = shareType === "upload" ? [SHARE_INFO_LINK] : [SHARE_INFO_LINK, DOWNLOAD_LINK];
  return links.map((link) => ({ ...link }));
}

/** Apply admin share defaults onto a share prompt form object (mutates target). */
export function applyShareDefaultsToForm(
  target: ShareFormTarget,
  values?: ShareDefaults | null,
  { titleDefault, descriptionDefault }: { titleDefault?: string; descriptionDefault?: string } = {},
): void {
  if (!values) {
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
    disableLoginOption: mapped.disableLoginOption,
    quotaEnabled: mapped.quotaEnabled,
    quotaCustomAmount: mapped.quotaCustomAmount,
    quotaCustomUnit: mapped.quotaCustomUnit,
    title: mapped.title || titleDefault || "",
    description: mapped.description || descriptionDefault || "",
    sidebarLinks: mapped.sidebarLinks?.length
      ? mapped.sidebarLinks.map((link) => ({ ...link }))
      : defaultSidebarLinksForShareType(mapped.shareType),
  } satisfies ShareFormTarget);
}
