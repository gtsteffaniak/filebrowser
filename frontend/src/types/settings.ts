/** /api/settings payloads from backend structs (json tags) */

import type { DeepPartial, MediaTranscodeSettings } from "./media.ts";

export type DarkMode = "auto" | "dark" | "light";

// --- user defaults ---

export interface UserDefaultsPermissions {
  api: boolean;
  admin: boolean;
  share: boolean;
  realtime: boolean;
}

export interface UserDefaultsSidebar {
  disableQuickToggles: boolean;
  hideFileActions: boolean;
  disableHideOnPreview: boolean;
  sticky: boolean;
  hideFiles: boolean;
  showTools: boolean | null;
}

export interface UserDefaultsListing {
  deleteWithoutConfirming: boolean;
  promptRightCloseButton: boolean;
  dateFormat: boolean;
  showHidden: boolean;
  quickDownload: boolean;
  showSelectMultiple: boolean;
  singleClick: boolean;
  hideFileExt: string;
  showCopyPath: boolean;
  showTypeColumn: boolean;
  showCreationDateColumn: boolean;
  deleteAfterArchive: boolean;
  viewMode: string;
  gallerySize: number;
  newFileTemplate: string[] | null;
}

export interface UserDefaultsPreview {
  image: boolean | null;
  video: boolean | null;
  audio: boolean | null;
  motionVideoPreview: boolean | null;
  office: boolean | null;
  popup: boolean | null;
  disablePreviewExt: string;
  highQuality: boolean | null;
  folder: boolean | null;
  models: boolean | null;
}

export interface UserDefaultsFileViewer {
  autoplayMedia: boolean | null;
  disableViewingExt: string;
  disableOnlyOfficeExt: string;
  preferEditorForMarkdown: boolean;
  preferEditorForHtml: boolean;
  debugOffice: boolean;
}

export interface UserDefaultsSearch {
  disableOptions: boolean;
}

export interface UserDefaultsUI {
  darkMode: DarkMode;
  themeColor: string;
  customTheme: string;
  locale: string;
}

export interface UserDefaultsFileLoading {
  maxConcurrentUpload: number;
  uploadChunkSizeMb: number;
  clearAll: boolean;
  downloadChunkSizeMb: number;
  uploadStallTimeoutSeconds: number;
}

export interface UserDefaultsAccount {
  permissions: UserDefaultsPermissions;
  lockPassword: boolean;
  requirePasswordChange: boolean;
  disableSettings: boolean;
  loginMethod?: string;
  disableUpdateNotifications: boolean;
  showAdvancedProfile: boolean;
  allowCustomShareURL: boolean | null;
  maxConcurrentTranscodes: number;
}

export interface UserDefaults {
  sidebar: UserDefaultsSidebar;
  listing: UserDefaultsListing;
  preview: UserDefaultsPreview;
  fileViewer: UserDefaultsFileViewer;
  search: UserDefaultsSearch;
  ui: UserDefaultsUI;
  fileLoading: UserDefaultsFileLoading;
  account: UserDefaultsAccount;
}

/** Editable copy of {@link UserDefaults}: every section exists, its fields may be missing */
export type UserDefaultsDraft = Required<DeepPartial<UserDefaults>>;

/** Which user default fields are enforced for all users - absent means not enforced */
export interface UserDefaultsEnforcement {
  sidebar?: {
    disableQuickToggles?: boolean;
    hideFileActions?: boolean;
    disableHideOnPreview?: boolean;
    sticky?: boolean;
    hideFiles?: boolean;
    showTools?: boolean;
  };
  listing?: {
    deleteWithoutConfirming?: boolean;
    promptRightCloseButton?: boolean;
    dateFormat?: boolean;
    showHidden?: boolean;
    quickDownload?: boolean;
    showSelectMultiple?: boolean;
    singleClick?: boolean;
    hideFileExt?: boolean;
    showCopyPath?: boolean;
    showTypeColumn?: boolean;
    showCreationDateColumn?: boolean;
    deleteAfterArchive?: boolean;
    viewMode?: boolean;
    gallerySize?: boolean;
    newFileTemplate?: boolean;
  };
  preview?: {
    image?: boolean;
    video?: boolean;
    audio?: boolean;
    motionVideoPreview?: boolean;
    office?: boolean;
    popup?: boolean;
    disablePreviewExt?: boolean;
    folder?: boolean;
    models?: boolean;
  };
  fileViewer?: {
    autoplayMedia?: boolean;
    disableViewingExt?: boolean;
    disableOnlyOfficeExt?: boolean;
    preferEditorForMarkdown?: boolean;
    preferEditorForHtml?: boolean;
    debugOffice?: boolean;
  };
  search?: { disableOptions?: boolean };
  ui?: { darkMode?: boolean; themeColor?: boolean; customTheme?: boolean; locale?: boolean };
  fileLoading?: {
    maxConcurrentUpload?: boolean;
    uploadChunkSizeMb?: boolean;
    downloadChunkSizeMb?: boolean;
    uploadStallTimeoutSeconds?: boolean;
  };
  account?: {
    lockPassword?: boolean;
    requirePasswordChange?: boolean;
    disableSettings?: boolean;
    disableUpdateNotifications?: boolean;
    showAdvancedProfile?: boolean;
    allowCustomShareURL?: boolean;
    maxConcurrentTranscodes?: boolean;
    permissions?: { admin?: boolean; api?: boolean; share?: boolean; realtime?: boolean };
  };
}

export interface UserDefaultsResponse {
  /** Admins only */
  values?: UserDefaults;
  enforced: UserDefaultsEnforcement;
  lockedFromConfigPaths?: string[];
  lockMessage?: string;
}

// --- share defaults ---

export interface SidebarLink {
  name: string;
  category: string;
  target: string;
  icon: string;
  sourceName?: string;
}

export interface ShareDefaults {
  shareTheme?: string;
  disableAnonymous?: boolean;
  disableThumbnails?: boolean;
  keepAfterExpiration?: boolean;
  themeColor?: string;
  title?: string;
  description?: string;
  favicon?: string;
  quickDownload?: boolean;
  hideNavButtons?: boolean;
  disableSidebar?: boolean;
  disableShareCard?: boolean;
  enforceDarkLightMode?: string;
  viewMode?: string;
  enableOnlyOffice?: boolean;
  shareType?: string;
  allowDelete?: boolean;
  allowCreate?: boolean;
  allowModify?: boolean;
  disableFileViewer?: boolean;
  disableDownload?: boolean;
  allowReplacements?: boolean;
  sidebarLinks?: SidebarLink[];
  showHidden?: boolean;
  disableLoginOption?: boolean;
  maxBandwidth?: number;
  allowedUsernames?: string[];
  perUserDownloadLimit?: boolean;
  downloadsLimit?: number;
  quotaLimitBytes?: number;
  hideFileExt?: string;
  banner?: string;
}

/** Same keys as {@link ShareDefaults} - a flag is true when that default is enforced for non-admins */
export type ShareDefaultsEnforcement = { [K in keyof ShareDefaults]?: boolean };

export type QuotaUnit = "mb" | "gb";

/** Form-only fields that {@link ShareDefaults} stores as `allowedUsernames` and `quotaLimitBytes` */
export interface ShareDefaultsFormExtras {
  allowedUsernames: string;
  enableAllowedUsernames: boolean;
  quotaEnabled: boolean;
  quotaCustomAmount: number;
  quotaCustomUnit: QuotaUnit;
}

export type ShareDefaultsForm = Omit<ShareDefaults, "allowedUsernames"> & ShareDefaultsFormExtras;

/** Share prompt form the admin defaults are applied onto - numeric limits are kept as input strings */
export type ShareFormTarget = Partial<
  Omit<ShareDefaultsForm, "downloadsLimit" | "maxBandwidth"> & { downloadsLimit: string; maxBandwidth: string }
>;

export interface ShareDefaultsResponse {
  values?: ShareDefaults;
  enforced: ShareDefaultsEnforcement;
}

// --- source settings ---

export interface SourceFilePermissions {
  view: boolean;
  download: boolean;
  modify: boolean;
  delete: boolean;
  create: boolean;
  /** True when explicitly set */
  configured?: boolean;
}

export interface SourceFilePermissionsEnforcement {
  view?: boolean;
  download?: boolean;
  modify?: boolean;
  create?: boolean;
  delete?: boolean;
}

export interface SourceSettings {
  defaultPermissions: SourceFilePermissions;
  enforcedPermissions: SourceFilePermissionsEnforcement;
  lockedFromConfigPaths?: string[];
  lockMessage?: string;
}

// --- global config (GET /api/settings) ---

export interface ConditionalRule {
  neverWatchPath: string;
  includeRootItem: string;
  fileStartsWith: string;
  folderStartsWith: string;
  fileEndsWith: string;
  folderEndsWith: string;
  folderPath: string;
  filePath: string;
  fileName: string;
  folderName: string;
  viewable: boolean;
  ignoreHidden: boolean;
  ignoreZeroSizeFolders: boolean;
  ignoreSymlinks: boolean;
}

export interface SourceConfig {
  denyByDefault?: boolean;
  private: boolean;
  readOnly?: boolean;
  disabled?: boolean;
  rules: ConditionalRule[] | null;
  defaultUserScope: string;
  defaultEnabled: boolean;
  createUserDir: boolean;
  useLogicalSize: boolean;
  defaultPermissions?: SourceFilePermissions;
}

export interface ConfigSource {
  path: string;
  name: string;
  config?: SourceConfig;
}

export interface LogConfig {
  levels: string;
  apiLevels: string;
  output: string;
  noColors: boolean;
  json: boolean;
  utc: boolean;
  apiFilter: string;
}

export interface ServerConfig {
  searchResultsLimit: number;
  minSearchLength: number;
  disableUpdateCheck: boolean;
  numImageProcessors: number;
  disablePreviews: boolean;
  disablePreviewResize: boolean;
  disableTypeDetectionByHeader: boolean;
  logging: LogConfig[] | null;
  sources: ConfigSource[] | null;
  cacheDir: string;
  cacheDirCleanup: boolean;
  cacheDirMaxSizeMB: number;
  maxArchiveSize: number;
  filesystem: { createFilePermission: string; createDirectoryPermission: string };
  indexSqlConfig: {
    batchSize: number;
    cacheSizeMB: number;
    walMode: boolean;
    disableReuse: boolean;
    fts5: boolean;
    startupIntegrityCheck?: "quickCheck" | "probe" | "off";
  };
  database: {
    path: string;
    migrateFrom: string;
    activity: { disabled: boolean; retentionDays: number; flushIntervalSeconds: number; maxBufferSize: number };
    quotas: { flushIntervalSeconds: number; flushMaxBuffers: number };
  };
}

export interface AuthCommon {
  enabled: boolean;
  adminGroup: string;
  userGroups: string[] | null;
  groupsClaim: string;
  userIdentifier: string;
  disableVerifyTLS: boolean;
  logoutRedirectUrl: string;
  /** Deprecated: always true for supported authentication methods */
  createUser: boolean;
}

export interface AuthConfig {
  tokenExpirationHours: number;
  methods: {
    proxy: AuthCommon & { header: string };
    noauth: boolean;
    password: {
      enabled: boolean;
      adminUsername: string;
      adminPassword: string;
      minLength: number;
      signup: boolean;
      recaptcha: { host: string; key: string; secret: string };
      enforcedOtp: boolean;
    };
    oidc: AuthCommon & { clientId: string; clientSecret: string; issuerUrl: string; scopes: string };
    ldap: AuthCommon & {
      server: string;
      baseDN: string;
      userDN: string;
      userPassword: string;
      userFilter: string;
    };
    jwt: AuthCommon & { header: string; secret: string; algorithm: string };
    passkey: {
      enabled: boolean;
      rpDisplayName: string;
      rpId: string;
      rpOrigins: string[] | null;
      loginButtonText: string;
    };
  };
  key: string;
  totpSecret: string;
  /** Deprecated: use `methods.password.adminUsername` */
  adminUsername: string;
  /** Deprecated: use `methods.password.adminPassword` */
  adminPassword: string;
}

export interface CustomTheme {
  description: string;
  css?: string;
}

export interface FrontendConfig {
  name: string;
  disableDefaultLinks: boolean;
  disableUsedPercentage: boolean;
  externalLinks: { text: string; title: string; url: string }[] | null;
  disableNavButtons: boolean;
  styling: {
    disableEventThemes: boolean;
    customCSS: string;
    lightBackground: string;
    darkBackground: string;
    customThemes: Record<string, CustomTheme> | null;
  };
  favicon: string;
  description: string;
  loginIcon: string;
  loginButtonText: string;
  oidcLoginButtonText: string;
  disablePWAInstall: boolean;
}

export interface OfficeConfig {
  url: string;
  internalUrl: string;
  secret: string;
  product?: "onlyoffice" | "collabora";
  tokenExpirationHours: number;
  viewOnly: boolean;
}

/** `integrations.media`: the deprecated seed values plus the config-only ffmpeg options */
export interface MediaConfig {
  ffmpegPath: string;
  skipHwCheck: boolean;
  convert: {
    imagePreview: Record<string, boolean | null> | null;
    videoPreview: Record<string, boolean | null> | null;
  };
  debug: boolean;
  extractEmbeddedSubtitles: boolean;
  hardwareAcceleration: boolean;
  transcode: MediaTranscodeSettings & { maxResolution: number; enabledProfiles?: string[] };
  accelDevice: string;
  alwaysUseFfmpeg: boolean;
  alwaysUseOnlyOffice: boolean;
}

export interface HttpConfig {
  socket: string;
  tlsKey: string;
  tlsCert: string;
  port: number;
  listen: string;
  baseURL: string;
  externalUrl: string;
  internalUrl: string;
  disableWebDAV: boolean;
  trustProxyHeaders: boolean;
  trustedProxies: string[] | null;
  unixSocketTrustedUIDs: number[] | null;
  unixSocketTrustedGIDs: number[] | null;
  unixSocketTrustAllLocalPeers: boolean;
  socketFileMode: string;
  socketGroup: string;
  disableRateLimit: boolean;
}

/** GET /api/settings */
export interface GlobalSettings {
  server: ServerConfig;
  auth: AuthConfig;
  frontend: FrontendConfig;
  userDefaults: UserDefaults;
  shareDefaults: ShareDefaults;
  integrations: { office: OfficeConfig; media: MediaConfig };
  http: HttpConfig;
}

/** Sections that GET /api/settings?property=<key> returns on their own. */
export interface GlobalSettingsProperties {
  userDefaults: UserDefaults;
  frontend: FrontendConfig;
  server: ServerConfig;
  auth: AuthConfig;
  sources: ConfigSource[] | null;
}

// --- API helpers ---

export interface SidebarLinkDefaults {
  items?: Array<{
    enabled?: boolean;
    enforced?: boolean;
    link?: SidebarLink;
  }>;
  sources?: string[];
}

export interface ToolAccessDefaults {
  items?: Array<{
    toolId?: string;
    enabled?: boolean;
    enforced?: boolean;
  }>;
}

/** Arbitrary JSON, for payloads whose schema is not fixed. */
export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

export interface AnalyticsStatus {
  enabled: boolean;
  ready: boolean;
  publishSupported: boolean;
}

/** Per-operation go-media shadow comparison */
export interface ShadowOpStats {
  operation: string;
  runs: number;
  succeeded: number;
  regressions: number;
  pairedRuns: number;
  goAvgMs: number;
  ffmpegAvgMs: number;
}

/** A file go-media does not support that ffmpeg handled */
export interface ShadowIncompatibleEntry {
  firstSeen: string;
  createdAt: string;
  occurrences: number;
  operation: string;
  path: string;
  format: string;
  sizeBytes: number;
}

/** A file most recent shadow regression (no file name or path) */
export interface ShadowRegressionEntry {
  firstSeen: string;
  createdAt: string;
  occurrences: number;
  operation: string;
  label: string;
  detail: string;
  format: string;
  sizeBytes: number;
  codecs?: string;
}

/** Values and enforcement flags are patched in separate requests */
export type UserDefaultsPatch = DeepPartial<UserDefaults> | { enforced: DeepPartial<UserDefaultsEnforcement> };

/** Default permissions and their enforcement flags are patched in separate requests */
export type SourceSettingsPatch =
  | { defaultPermissions: Partial<SourceFilePermissions> }
  | { enforcedPermissions: SourceFilePermissionsEnforcement };

export type ShareDefaultsPatch = DeepPartial<ShareDefaults> & { enforced?: ShareDefaultsEnforcement };
