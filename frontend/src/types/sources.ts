/** from GET /api/settings/sources (sidebar usage bar) */
export interface ScopeQuotaInfo {
  quotaKind?: string;
  limitBytes: number;
  usedBytes?: number;
  reservedBytes?: number;
  meter?: string;
  configuredMeter?: string;
  effectiveMeter?: string;
  measurementStatus?: string;
}

export interface SourceInfo {
  pathPrefix?: string | undefined;
  used: number;
  total: number;
  usedAlt: number;
  totalRoot: number;
  usedAltRoot: number;
  usedPercentage: number;
  usageScopeMismatch?: boolean | undefined;
  status: string;
  name: string;
  files: number;
  folders: number;
  lastIndex: number;
  quickScanDurationSeconds: number;
  fullScanDurationSeconds: number;
  complexity: number;
  scanners: unknown[];
  readOnly: boolean;
  private: boolean;
  indexingDisabled?: boolean | undefined;
  scopeQuota?: ScopeQuotaInfo | undefined;
}

/** Raw shape of a single source entry as sent by /api/settings/sources or SSE updates */
export interface SourceInfoUpdate {
  used?: number;
  total?: number;
  usedAlt?: number;
  totalRoot?: number;
  usedAltRoot?: number;
  usageScopeMismatch?: boolean;
  status?: string;
  name?: string;
  numFiles?: number;
  numDirs?: number;
  lastIndexedUnixTime?: number;
  quickScanDurationSeconds?: number;
  fullScanDurationSeconds?: number;
  complexity?: number;
  scanners?: unknown[];
  readOnly?: boolean;
  private?: boolean;
  indexingDisabled?: boolean;
  scopeQuota?: ScopeQuotaInfo;
}
