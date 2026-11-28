export interface ShareInfoObject {
  isShare: boolean;
  disableThumbnails: boolean;
  hash: string;
  token: string;
  subPath: string;
  passwordValid: boolean;
  enforceDarkLightMode: string;
  disableSidebar: boolean;
  isValid: boolean;
  shareType: string;
  hasPassword?: boolean;
  title: string;
  description: string;
  viewMode?: string;
  singleFileShare?: boolean;
  disableFileViewer?: boolean;
  allowModify?: boolean;
  allowCreate?: boolean;
  allowReplacements?: boolean;
  allowDelete?: boolean;
  canEditShare?: boolean;
  disableDownload?: boolean;
  showHidden?: boolean;
  quotaLimitBytes?: number;
  quotaUsedBytes?: number;
  quotaAvailableBytes?: number;
  /** Applicable folder storage quotas for this share root (from share/info). */
  folderQuotas?: Array<{
    limitBytes: number;
    usedBytes?: number;
    reservedBytes?: number;
  }>;
}
