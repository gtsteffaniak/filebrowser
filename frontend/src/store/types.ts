// Store type definitions

export interface ReqObject {
  // Base properties always present
  sorting: {
    by: string;
    asc: boolean;
  };
  items: unknown[];
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

  // Directory listing properties
  listing?: unknown[];
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
  permissions: unknown;
  darkMode: boolean;
  disableSettings: boolean;
  debugOffice: boolean;
  preferEditorForMarkdown: boolean;
  showCopyPath?: boolean;
  profile: {
    username: string;
    email: string;
    avatarUrl: string;
  };
  // Optional properties that may be added dynamically
  disableViewingExt?: string[];
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
    fbdata?: { name: string; path: string; source: string; size?: number; type: string };
  } | null;
  shareInfo: ShareInfoObject;
  sources: {
    current: string;
    count: number;
    hasSourceInfo: boolean;
    info: unknown;
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
  };
  sharePassword: string;
  loading: unknown[];
  reload: boolean;
  selected: unknown[];
  lastSelectedIndex: number | null;
  multiple: boolean;
  upload: {
    uploads: unknown;
    queue: unknown[];
    progress: unknown[];
    sizes: unknown[];
    isUploading: boolean;
  };
  prompts: unknown[];
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
    timeout: unknown;
    enabled: boolean;
    isTransitioning: boolean;
    transitionStartTime: unknown;
  };
  playbackQueue: {
    queue: unknown[];
    currentIndex: number;
    mode: string;
    isPlaying: boolean;
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
