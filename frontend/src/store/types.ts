// Store type definitions

export interface ReqObject {
  // Base properties always present
  sorting: {
    by: string;
    asc: boolean;
  };
  items: any[];
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
  subtitles?: any[];

  // Directory listing properties
  listing?: any[];
}

export interface ShareObject {
  hash: string | null;
  token: string;
  subPath: string;
}

export interface UserObject {
  preview: {
    video: boolean;
    image: boolean;
    popup: boolean;
    highQuality: boolean;
    autoplayMedia?: boolean;
    defaultMediaPlayer?: boolean;
  };
  loginType: string;
  username: string;
  quickDownloadEnabled: boolean;
  gallarySize: number;
  singleClick: boolean;
  stickySidebar: boolean;
  locale: string;
  viewMode: string;
  showHidden: boolean;
  scopes: any[];
  permissions: any;
  darkMode: boolean;
  disableSettings: boolean;
  debugOffice: boolean;
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
  rules?: any[];
  lockPassword?: boolean;
  hideDotfiles?: boolean;
  commands?: string[];
  sorting?: {
    by: string;
    asc: boolean;
  };
  dateFormat?: boolean;
  perm?: any;
  email?: string;
  avatarUrl?: string;
}

export interface RouteObject {
  name?: string;
  path?: string;
  params?: any;
  query?: any;
}

export interface StoreState {
  tooltip: {
    show: boolean;
    content: string;
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
  displayPreferences: any;
  usages: any;
  editor: any;
  serverHasMultipleSources: boolean;
  realtimeActive: boolean | undefined;
  realtimeDownCount: number;
  popupPreviewSource: string;
  share: ShareObject;
  sources: {
    current: string;
    count: number;
    hasSourceInfo: boolean;
    info: any;
  };
  user: UserObject;
  req: ReqObject;
  listing: {
    category: string;
    letter: string;
    scrolling: boolean;
    scrollRatio: number;
  };
  previewRaw: string;
  oldReq: any;
  clipboard: {
    key: string;
    items: any[];
  };
  sharePassword: string;
  loading: any[];
  reload: boolean;
  selected: any[];
  lastSelectedIndex: number | null;
  multiple: boolean;
  upload: {
    uploads: any;
    queue: any[];
    progress: any[];
    sizes: any[];
  };
  prompts: any[];
  show: any;
  showConfirm: any;
  route: RouteObject;
  settings: {
    signup: boolean;
    createUserDir: boolean;
    userHomeBasePath: string;
    rules: any[];
    frontend: {
      disableExternal: boolean;
      name: string;
      files: string;
    };
  };
}
