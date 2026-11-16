import type { PlainMessageKey } from "@/i18n/index.ts";

export const MEDIA_TRANSCODE_PROFILES: { id: string; label: PlainMessageKey }[] = [
  { id: "quality", label: "general.quality" },
  { id: "balanced", label: "general.balanced" },
  { id: "datasaver", label: "general.dataSaver" },
];
