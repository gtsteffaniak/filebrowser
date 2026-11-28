export type QuotaMeter = "index_size" | "index_scope" | "accounted";

export type QuotaUnit = "mb" | "gb";

/** from backend web.folderQuotaResponse */
export interface QuotaSnapshot {
  limitBytes?: number;
  usedBytes?: number;
  reservedBytes?: number;
  meter?: QuotaMeter;
  configuredMeter?: QuotaMeter;
  effectiveMeter?: QuotaMeter;
  measurementStatus?: "ready" | "accounted_fallback";
}

export interface FolderQuota extends QuotaSnapshot {
  source: string;
  path: string;
  limitBytes: number;
}
