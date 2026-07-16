// Store type definitions

export interface FileListItem {
  name: string;
  path: string;
  size?: number;
  type?: string;
  source?: string;
  modified?: string;
  hasPreview?: boolean;
  viewToken?: string;
  isShared?: boolean;
  pinned?: boolean;
  hidden?: boolean;
}

export interface ReqObject {
  // Base properties always present
  sorting: {
    by: string;
    asc: boolean;
  };
  items: FileListItem[];
  numDirs: number;
  numFiles: number;

  // File-specific properties (added dynamically)
  name?: string;
  path?: string;
  size?: number;
  type?: string;
  source?: string;
  content?: string;
  modified?: string;
  hasPreview?: boolean;
  subtitles?: unknown[];
  viewToken?: string;
  parentDirItems?: FileListItem[];
  onlyOfficeId?: string;
  hasUpdate?: boolean;
  metadata?: unknown;

  // Directory listing properties
  listing?: FileListItem[];
}

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
  title: string;
  description: string;
  viewMode?: string;
  singleFileShare?: boolean;
  disableFileViewer?: boolean;
  allowModify?: boolean;
  allowCreate?: boolean;
  allowDelete?: boolean;
  disableDownload?: boolean;
  showHidden?: boolean;
}

export interface Permissions {
  share?: boolean;
  modify?: boolean;
  create?: boolean;
  delete?: boolean;
  download?: boolean;
  admin?: boolean;
  api?: boolean;
  archive?: boolean;
  realtime?: boolean;
}

export interface DisplayPreference {
  viewMode?: string;
  sorting?: {
    by: string;
    asc: boolean;
  };
}

export interface SidebarLink {
  category?: string;
  sourceName?: string;
  [key: string]: unknown;
}

export interface Prompt {
  id?: number;
  name?: string;
  parentId?: number;
  pinned?: boolean;
  confirm?: unknown;
  action?: unknown;
  props?: Record<string, unknown>;
  discard?: unknown;
  cancel?: unknown;
}

export interface SourceInfo {
  pathPrefix?: string;
  used: number;
  total: number;
  usedAlt: number;
  usedPercentage: number;
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
}

/** Raw shape of a single source entry as sent by /api/settings/sources or SSE updates. */
export interface SourceInfoUpdate {
  used?: number;
  total?: number;
  usedAlt?: number;
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
}

export interface UserObject {
  preview: {
    video: boolean;
    audio: boolean;
    image: boolean;
    models: boolean;
    popup: boolean;
    autoplayMedia?: boolean;
    defaultMediaPlayer?: boolean;
    office?: boolean;
    folder?: boolean;
    motionVideoPreview?: boolean;
    disableHideSidebar?: boolean;
  };
  loginType: string;
  username: string;
  quickDownloadEnabled: boolean;
  gallerySize: number;
  singleClick: boolean;
  stickySidebar: boolean;
  hideFilesInTree: boolean,
  deleteAfterArchive: boolean,
  locale: string;
  viewMode: string;
  showHidden: boolean;
  scopes: unknown[];
  permissions: Permissions;
  darkMode: boolean;
  disableSettings: boolean;
  debugOffice: boolean;
  preferEditorForMarkdown: boolean;
  showCopyPath?: boolean;
  hideFileExt?: string;
  themeColor?: string;
  sidebarLinks?: SidebarLink[];
  profile: {
    username: string;
    email: string;
    avatarUrl: string;
  };
  // Optional properties that may be added dynamically
  disableViewingExt?: string;
  disableOnlyOfficeExt?: string;
  displayNames?: string[];
  id?: number;
  password?: string;
  scope?: string;
  rules?: unknown[];
  lockPassword?: boolean;
  hideDotfiles?: boolean;
  sorting?: {
    by: string;
    asc: boolean;
  };
  dateFormat?: boolean;
  perm?: unknown;
  email?: string;
  avatarUrl?: string;
  fileLoading?: {
    maxConcurrentUpload?: number;
    uploadChunkSizeMb?: number;
    clearAll?: boolean;
    downloadChunkSizeMb?: number;
  };
}

export interface RouteObject {
  name?: string;
  path?: string;
  params?: unknown;
  query?: unknown;
}

export interface StoreState {
  disableEventThemes: boolean;
  tooltip: {
    show: boolean;
    content: string;
    component: import("vue").Component | null;
    componentProps: Record<string, unknown> | null;
    x: number;
    y: number;
    pointerEvents: boolean;
    width: number | null;
  };
  previousHistoryItem: {
    name: string;
    source: string;
    path: string;
    isShare?: boolean;
  };
  contextMenuHasItems: boolean;
  deletedItem: boolean;
  showOverflowMenu: boolean;
  sessionId: string;
  isSafari: boolean;
  activeSettingsView: string;
  isMobile: boolean;
  isSearchActive: boolean;
  showSidebar: boolean;
  displayPreferences: unknown;
  usages: unknown;
  editor: unknown;
  editorDirty: boolean;
  editorSaveHandler: unknown;
  realtimeActive: boolean | undefined;
  realtimeDownCount: number;
  popupPreviewSourceInfo: {
    source: string;
    path: string;
    size?: string;
    url?: string;
    modified?: string;
    type?: "3d";
    fbdata?: {
      name: string;
      path: string;
      source: string;
      size?: number;
      type: string;
      viewToken?: string;
      parentDirItems?: FileListItem[];
    };
  } | null;
  shareInfo: ShareInfoObject;
  seenUpdate?: string | null;
  sources: {
    current: string;
    count: number;
    hasSourceInfo: boolean;
    info: Record<string, SourceInfo>;
    defaultSource?: string;
  };
  user: UserObject;
  req: ReqObject;
  listing: {
    category: string;
    letter: string;
    scrolling: boolean;
    scrollRatio: number;
    listingScrollTop: number;
  };
  previewRaw: string;
  oldReq: unknown;
  clipboard: {
    key: string;
    items: unknown[];
    path?: string;
  };
  sharePassword: string;
  loading: Record<string, unknown>;
  reload: boolean;
  selected: (number | FileListItem)[];
  lastSelectedIndex: number | null;
  multiple: boolean;
  upload: {
    uploads: Record<string, { id: string | number; type: string; file: { name: string; type: string; size?: number } }>;
    queue: unknown[];
    progress: number[];
    sizes: number[];
    isUploading: boolean;
  };
  prompts: Prompt[];
  promptIdCounter: number;
  show: unknown;
  showConfirm: unknown;
  route: RouteObject;
  settings: {
    signup: boolean;
    createUserDir: boolean;
    userHomeBasePath: string;
    rules: unknown[];
    frontend: {
      disableExternal: boolean;
      name: string;
      files: string;
    };
  };
  navigation: {
    show: boolean;
    hoverNav: boolean;
    listing: unknown;
    currentIndex: number;
    previousItem: unknown;
    nextItem: unknown;
    previousLink: string;
    nextLink: string;
    previousRaw: string;
    nextRaw: string;
    timeout: ReturnType<typeof setTimeout> | null;
    enabled: boolean;
    isTransitioning: boolean;
    transitionStartTime: number | null;
    gestureHint: 'previous' | 'next' | 'close' | null;
    gestureHintCommitReady: boolean;
    gestureHintFlashClose: boolean;
  };
  playbackQueue: {
    queue: unknown[];
    currentIndex: number;
    mode: 'sequential' | 'shuffle';
    isPlaying: boolean;
    loop: 'off' | 'all' | 'single';
    shouldTogglePlayPause?: boolean;
  };
  notificationHistory: unknown[];
  sidebar: {
    width: number;
    mode: string;
    isResizing: boolean;
    minWidth: number;
    maxWidth: number;
  };
  editorStats: {
    lines: number | null;
    words: number | null;
    chars: number | null;
  };
  editorFontSize: number;
}
