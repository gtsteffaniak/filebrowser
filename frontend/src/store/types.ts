import type { Component } from "vue";
import type { PlaybackLoop, PlaybackMode, SharedMediaSettings } from "@/types/media.ts";
import type { FileListItem, ReqObject } from "@/types/files.ts";
import type { ShareInfoObject } from "@/types/shares.ts";
import type { UserObject } from "@/types/users.ts";
import type { SourceInfo } from "@/types/sources.ts";
import type {
  GlobalSettings,
  ShareDefaults,
  ShareDefaultsEnforcement,
  SidebarLinkDefaults,
  ToolAccessDefaults,
  UserDefaultsEnforcement,
} from "@/types/settings.ts";

export interface Prompt {
  id?: number | undefined;
  name?: string | undefined;
  parentId?: number | undefined;
  pinned?: boolean | undefined;
  confirm?: unknown;
  action?: unknown;
  props?: Record<string, unknown> | undefined;
  discard?: unknown;
  cancel?: unknown;
}

export interface RouteObject {
  name?: string | symbol | null | undefined;
  path?: string;
  fullPath?: string;
  params?: unknown;
  query?: unknown;
}

export interface StoreState {
  disableEventThemes: boolean;
  // Reactive snapshot of the OS color-scheme, so "auto" re-resolves when the
  // system switches appearance. Kept in the reactive store on purpose.
  systemDark: boolean;
  tooltip: {
    show: boolean;
    content: string;
    component: Component | null;
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
    returnTo?: string;
  } | null;
  contextMenuHasItems: boolean;
  deletedItem: boolean;
  pendingHighlight: { path: string; source?: string } | null;
  searchResults: {
    kind: "quick" | "advanced";
    key?: string;
    items: FileListItem[];
    form?: Record<string, unknown>;
    sorting?: { by: string; asc: boolean };
    focus?: { source?: string; path: string };
  } | null;
  showOverflowMenu: boolean;
  sessionId: string;
  isSafari: boolean;
  activeSettingsView: string;
  isSearchActive: boolean;
  showSidebar: boolean;
  displayPreferences: unknown;
  enforcedUserDefaults: UserDefaultsEnforcement;
  /** Runtime media state for all UI (player, listings, previews); null until loaded. */
  mediaSettings: SharedMediaSettings | null;
  sidebarLinkDefaultsPolicy: SidebarLinkDefaults;
  toolAccessDefaultsPolicy: ToolAccessDefaults;
  shareDefaultsPolicy: {
    values?: ShareDefaults;
    enforced?: ShareDefaultsEnforcement;
  };
  usages: unknown;
  editor: {
    instance: unknown;
    dirty: boolean;
    saveHandler: (() => Promise<void>) | null;
    jsonFormatted: boolean;
    stats: {
      lines: number | null;
      words: number | null;
      chars: number | null;
    };
    fontSize: number;
    markdownSplitView: boolean;
    scrollRatio: number;
    scrollSource: 'editor' | 'viewer' | null;
    scrollPath: string;
  };
  realtimeActive: boolean | undefined;
  realtimeDownCount: number;
  popupPreviewSourceInfo: {
    source: string;
    path: string;
    size?: string;
    url?: string;
    modified?: string;
    type?: "3d" | "text";
    /** Text preview API URL, set when type is "text". */
    textUrl?: string;
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
  /** Sort config for destination picker dialogs (copy/move/archive). Independent of the main listing sort. */
  pickerSorting: {
    by: string;
    asc: boolean;
  } | null;
  clipboard: {
    key: string;
    items: unknown[];
    path?: string | undefined;
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
  settings: GlobalSettings | null;
  navigation: {
    show: boolean;
    hoverNav: boolean;
    listing: Partial<FileListItem>[] | null;
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
    sorting: { by: string; asc: boolean } | null;
  };
  playbackQueue: {
    queue: FileListItem[];
    currentIndex: number;
    mode: PlaybackMode;
    isPlaying: boolean;
    loop: PlaybackLoop;
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
}
