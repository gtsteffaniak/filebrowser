import { notify } from "@/notify/index.ts";
import type { DeepPartial, MediaSettings, MediaSettingsResponse, SharedMediaSettings } from "@/types/media.ts";
import type { SourceInfoUpdate } from "@/types/sources.ts";
import type {
  AnalyticsStatus,
  GlobalSettings,
  GlobalSettingsProperties,
  JsonValue,
  ShadowIncompatibleEntry,
  ShadowOpStats,
  ShadowRegressionEntry,
  ShareDefaultsPatch,
  ShareDefaultsResponse,
  SidebarLinkDefaults,
  SourceSettings,
  SourceSettingsPatch,
  ToolAccessDefaults,
  UserDefaultsPatch,
  UserDefaultsResponse,
} from "@/types/settings.ts";
import { normalizeSharedMediaSettings } from "@/utils/config/mediaSettings.ts";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL, HttpError, requestTimeoutSignal } from "./utils.ts";

const analyticsRequestTimeoutMs = 5000;

export function get(): Promise<GlobalSettings>;
export function get<P extends keyof GlobalSettingsProperties>(property: P): Promise<GlobalSettingsProperties[P]>;
export function get(property = ""): Promise<GlobalSettings | GlobalSettingsProperties[keyof GlobalSettingsProperties]> {
  const path = getApiPath("settings", { property });
  return fetchJSON<GlobalSettings | GlobalSettingsProperties[keyof GlobalSettingsProperties]>(path);
}

export async function update(settings: GlobalSettings): Promise<void> {
  await fetchURL("api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function config(showFull = false, showComments = false): Promise<Response> {
  const params: Record<string, string> = {};
  if (showFull) params.full = "true";
  if (showComments) params.comments = "true";
  const path = getApiPath("settings/config", params);
  return fetchURL(path);
}

export async function sources(): Promise<Record<string, SourceInfoUpdate>> {
  try {
    const apiPath = getApiPath('settings/sources')
    const res = await fetchURL(apiPath)
    const data: Record<string, SourceInfoUpdate> | null = await res.json()
    // Return empty object if no sources are available - this is not an error
    return data || {}
  } catch (err) {
    // Only show error for actual network/server errors, not for empty sources
    if (err instanceof HttpError && err.status && err.status !== 200) {
      notify.showError(err.message || 'Error fetching usage sources')
    }
    throw err
  }
}

export function getAnalytics(): Promise<AnalyticsStatus> {
  return fetchJSON<AnalyticsStatus>(getApiPath("settings/analytics"), {
    signal: requestTimeoutSignal(analyticsRequestTimeoutMs),
  });
}

export async function updateAnalytics({ enabled }: { enabled: boolean }): Promise<AnalyticsStatus> {
  return fetchJSON<AnalyticsStatus>(getApiPath("settings/analytics"), {
    method: "PUT",
    body: JSON.stringify({ enabled }),
    signal: requestTimeoutSignal(analyticsRequestTimeoutMs),
  });
}

export function getAnalyticsPreview(): Promise<JsonValue> {
  return fetchJSON<JsonValue>(getApiPath("settings/analytics/preview"));
}

export function getShadowRegressions(): Promise<ShadowRegressionEntry[]> {
  return fetchJSON<ShadowRegressionEntry[]>(getApiPath("settings/shadow-regressions"));
}

export function getShadowIncompatible(): Promise<ShadowIncompatibleEntry[]> {
  return fetchJSON<ShadowIncompatibleEntry[]>(getApiPath("settings/shadow-incompatible"));
}

export function getShadowRegressionStats(): Promise<ShadowOpStats[]> {
  return fetchJSON<ShadowOpStats[]>(getApiPath("settings/shadow-regressions/stats"));
}

export function clearShadowRegressions(): Promise<{ cleared: boolean }> {
  return fetchJSON<{ cleared: boolean }>(getApiPath("settings/shadow-regressions"), {
    method: "DELETE",
  });
}

export function getUserDefaults(): Promise<UserDefaultsResponse> {
  return fetchJSON<UserDefaultsResponse>(getApiPath("settings/user-defaults"));
}

/** Enforcement flags for profile UI (works on public routes behind proxy basic auth). */
export function getEnforcedUserDefaults(): Promise<UserDefaultsResponse> {
  return fetchJSON<UserDefaultsResponse>(getPublicApiPath("settings/user-defaults"));
}

/** Sidebar link defaults policy for the current user (enforcement UI). */
export function getSidebarLinkDefaultsPolicy(): Promise<SidebarLinkDefaults> {
  return fetchJSON<SidebarLinkDefaults>(getPublicApiPath("settings/sidebar-link-defaults"));
}

export async function patchUserDefaults(partial: UserDefaultsPatch): Promise<void> {
  await fetchURL(getApiPath("settings/user-defaults"), {
    method: "PATCH",
    body: JSON.stringify(partial),
    headers: { "Content-Type": "application/json" },
  });
}

export function getSidebarLinkDefaults(): Promise<SidebarLinkDefaults> {
  return fetchJSON<SidebarLinkDefaults>(getApiPath("settings/sidebar-link-defaults"));
}

export async function patchSidebarLinkDefaults(doc: SidebarLinkDefaults): Promise<SidebarLinkDefaults> {
  return fetchJSON<SidebarLinkDefaults>(getApiPath("settings/sidebar-link-defaults"), {
    method: "PATCH",
    body: JSON.stringify(doc),
    headers: { "Content-Type": "application/json" },
  });
}

/** Tool access defaults policy for the current user (enforcement UI). */
export function getToolAccessDefaultsPolicy(): Promise<ToolAccessDefaults> {
  return fetchJSON<ToolAccessDefaults>(getPublicApiPath("settings/tool-access-defaults"));
}

export function getToolAccessDefaults(): Promise<ToolAccessDefaults> {
  return fetchJSON<ToolAccessDefaults>(getApiPath("settings/tool-access-defaults"));
}

export async function patchToolAccessDefaults(doc: ToolAccessDefaults): Promise<ToolAccessDefaults> {
  return fetchJSON<ToolAccessDefaults>(getApiPath("settings/tool-access-defaults"), {
    method: "PATCH",
    body: JSON.stringify(doc),
    headers: { "Content-Type": "application/json" },
  });
}

export function getMediaSettings(): Promise<MediaSettingsResponse> {
  return fetchJSON<MediaSettingsResponse>(getApiPath("settings/media"));
}

/**
 * Shared runtime media state (player + general UI), normalized to a flat shape.
 * Non-admins receive the PUBLIC UserMedia payload; admins get the full payload on the
 * same endpoint, which carries the same transcode and imagePreview keys.
 */
export async function getSharedMediaSettings(): Promise<SharedMediaSettings> {
  const data = await fetchJSON<MediaSettingsResponse>(getApiPath("settings/media"));
  return normalizeSharedMediaSettings(data);
}

export async function patchMediaSettings(partial: DeepPartial<MediaSettings>): Promise<DeepPartial<MediaSettings>> {
  return fetchJSON<DeepPartial<MediaSettings>>(getApiPath("settings/media"), {
    method: "PATCH",
    body: JSON.stringify(partial),
    headers: { "Content-Type": "application/json" },
  });
}

export function getSourceSettings(): Promise<SourceSettings> {
  return fetchJSON<SourceSettings>(getApiPath("settings/source"));
}

export async function patchSourceSettings(partial: SourceSettingsPatch): Promise<SourceSettings> {
  return fetchJSON<SourceSettings>(getApiPath("settings/source"), {
    method: "PATCH",
    body: JSON.stringify(partial),
  });
}

export function getShareDefaults(): Promise<ShareDefaultsResponse> {
  return fetchJSON<ShareDefaultsResponse>(getApiPath("settings/share-defaults"));
}

/** Share defaults and enforcement policy (works on public routes behind proxy basic auth). */
export function getShareDefaultsPolicy(): Promise<ShareDefaultsResponse> {
  return fetchJSON<ShareDefaultsResponse>(getPublicApiPath("settings/share-defaults"));
}

export async function patchShareDefaults(partial: ShareDefaultsPatch): Promise<void> {
  await fetchURL(getApiPath("settings/share-defaults"), {
    method: "PATCH",
    body: JSON.stringify(partial),
    headers: { "Content-Type": "application/json" },
  });
}
