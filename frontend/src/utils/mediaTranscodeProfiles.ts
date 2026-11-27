import type { TranscodeProfile } from "@/api/transcode.ts";
import i18n from "@/i18n/index.ts";

/** `label` is translated on every call, so it follows the current locale */
export const MEDIA_TRANSCODE_PROFILES: { id: TranscodeProfile; label: () => string }[] = [
  { id: "quality", label: () => i18n.global.t("general.quality") },
  { id: "balanced", label: () => i18n.global.t("general.balanced") },
  { id: "datasaver", label: () => i18n.global.t("general.dataSaver") },
];
