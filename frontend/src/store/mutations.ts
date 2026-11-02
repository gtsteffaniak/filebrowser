import { setActiveViewGrantScope } from "@/api/viewToken.js";
import { markRaw } from "vue";
import { resourcesApi, usersApi } from "@/api/index.js";
import {
  getEnforcedUserDefaults,
  getMediaPlayerSettings,
  getShareDefaultsPolicy,
  getSidebarLinkDefaultsPolicy,
  getToolAccessDefaultsPolicy,
} from "@/api/settings.js";
import { detectLocale, setLocale } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { url } from "@/utils/index.ts";
import { getTypeInfo } from "@/utils/mimetype.js";
import { getObjectProperty, setObjectProperty, omitObjectProperty } from '@/utils/object.js';
import { sortedItems } from "@/utils/sort.ts";
import { isSourceSidebarCategory } from "@/utils/sidebarCategory.ts";
import { updateManifestLink } from "@/utils/pwaManifest.js";
import { syncEventTheme } from "@/utils/theme.js";
import {
  cancelCloseUploadPrompt,
  confirmCloseUploadPrompt,
  hasCloseWithActiveUploadsPrompt,
} from "@/utils/uploadNavigationGuard.ts";
import { getters } from "./getters.ts";
import { state } from "./state.ts";
import { rawList, withEntry, withoutEntry } from "@/utils/listing/selection.ts";
import type {
  DisplayPreference,
  FileListItem,
  Prompt,
  ReqObject,
  RouteObject,
  ShareInfoObject,
  SourceInfo,
  SourceInfoUpdate,
  StoreState,
  UserObject,
  UserScope,
} from "./types.ts";

let navigationSortCache: {
  listing: Partial<FileListItem>[];
  length: number;
  by: string;
  asc: boolean;
  items: Partial<FileListItem>[];
} | null = null;

function getSortedNavigationItems(listing: Partial<FileListItem>[], by: string, asc: boolean): Partial<FileListItem>[] {
  const cache = navigationSortCache;
  if (cache && cache.listing === listing && cache.length === listing.length && cache.by === by && cache.asc === asc) {
    return cache.items;
  }
  const items: Partial<FileListItem>[] = sortedItems([...listing], by, asc);
  navigationSortCache = { listing, length: listing.length, by, asc, items };
  return items;
}

const uploadBeforeUnloadHandler = (event: BeforeUnloadEvent) => {
  if (!state.upload.isUploading) {
    return;
  }
  event.preventDefault();
  event.returnValue = "";
};

export const mutations = {
  disableEventThemes: () => {
    if (state.disableEventThemes) {
      return;
    }
    localStorage.setItem("disableEventThemes", "true");
    localStorage.removeItem("eventThemeOptIn");
    state.disableEventThemes = true;
    syncEventTheme(false);
    // Set theme color back to user's preference or default
    if (state.user.themeColor) {
      document.documentElement.style.setProperty("--primaryColor", state.user.themeColor);
    } else {
      // Remove the override to use the default CSS variable
      document.documentElement.style.removeProperty("--primaryColor");
    }
  },
  enableEventThemes: () => {
    localStorage.removeItem("disableEventThemes");
    localStorage.setItem("eventThemeOptIn", "halloween");
    state.disableEventThemes = false;
    if (getters.eventTheme() === "halloween") {
      syncEventTheme(true);
    }
  },
  removeFromSearchResults: (removed: { source?: string; path: string }[]) => {
    const search = state.searchResults;
    if (!search || !removed?.length) return;
    const targets = removed.map((r) => ({ source: r.source, path: url.trimSlashes(r.path) }));
    search.items = search.items.filter((item) => {
      const path = url.trimSlashes(item.path);
      return !targets.some(
        (t) => t.source === item.source && (path === t.path || path.startsWith(`${t.path}/`))
      );
    });
  },
  setPreviousHistoryItem: (value: StoreState['previousHistoryItem'] | null) => {
    if (value === state.previousHistoryItem) {
      return;
    }
    if (value !== null) {
      value.isShare = getters.isShare();
    }
    state.previousHistoryItem = value;
  },
  setContextMenuHasItems: (value: boolean) => {
    if (value === state.contextMenuHasItems) {
      return;
    }
    state.contextMenuHasItems = value;
  },
  setEditorDirty: (value: boolean) => {
    if (value === state.editor.dirty) {
      return;
    }
    state.editor.dirty = value;
  },
  setEditorSaveHandler: (handler: StoreState['editor']['saveHandler']) => {
    state.editor.saveHandler = handler;
  },
  setEditorJsonFormatted: (value: boolean) => {
    if (value === state.editor.jsonFormatted) return;
    state.editor.jsonFormatted = value;
  },
  setEditorStats: (stats: StoreState['editor']['stats']) => {
    if (JSON.stringify(state.editor.stats) === JSON.stringify(stats)) return;
    state.editor.stats = stats;
  },
  setEditorFontSize: (size: number) => {
    if (state.editor.fontSize === size) return;
    state.editor.fontSize = size;
    localStorage.setItem('editorFontSize', String(size));
  },
  setMarkdownSplitView: (value: boolean) => {
    if (value === state.editor.markdownSplitView) return;
    state.editor.markdownSplitView = value;
    sessionStorage.setItem("markdownSplitView", value ? "true" : "false");
  },
  toggleSplitView: () => {
    mutations.setMarkdownSplitView(!state.editor.markdownSplitView);
  },
  setEditorScrollRatio: (ratio: number, source: StoreState['editor']['scrollSource']) => {
    state.editor.scrollRatio = ratio;
    state.editor.scrollSource = source;
  },
  // Called by both Editor and MarkdownViewer on mounts
  resetEditorScrollRatio: (path: string) => {
    if (state.editor.scrollPath === path) return;
    state.editor.scrollPath = path;
    state.editor.scrollRatio = 0;
    state.editor.scrollSource = null;
  },
  setDeletedItem: (value: boolean) => {
    if (value === state.deletedItem) {
      return;
    }
    state.deletedItem = value;
  },
  setSeenUpdate: (value: string | null) => {
    if (value === state.seenUpdate) {
      return;
    }
    state.seenUpdate = value
    localStorage.setItem("seenUpdate", value ?? "");
  },
  toggleOverflowMenu: () => {
    state.showOverflowMenu = !state.showOverflowMenu;
  },
  setWatchDirChangeAvailable() {
    state.req.hasUpdate = true;
  },
  setPreviewSource: (value: string) => {
    if (!value && !state.popupPreviewSourceInfo) {
      return;
    }
    if (value === state.popupPreviewSourceInfo?.url) {
      return;
    }
    if (value) {
      state.popupPreviewSourceInfo = {
        ...state.popupPreviewSourceInfo,
        url: value,
        size: "xlarge",
      } as NonNullable<StoreState['popupPreviewSourceInfo']>;
    } else {
      state.popupPreviewSourceInfo = null;
    }
  },
  /** Sets (or clears with null) the full popup preview info, e.g. when an Icon is hovered. */
  setPopupPreview: (info: StoreState['popupPreviewSourceInfo']) => {
    if (!info && !state.popupPreviewSourceInfo) {
      return;
    }
    state.popupPreviewSourceInfo = info;
  },
  updateListing: (value: StoreState['listing']) => {
    if (value === state.listing) {
      return;
    }
    // Assign only the fields that changed instead of replacing the object, so components
    // reading aren't invalidated by a scrollRatio update.
    const listing = state.listing as unknown as Record<string, unknown>;
    for (const [key, next] of Object.entries(value)) {
      if (!Object.is(listing[key], next)) {
        listing[key] = next;
      }
    }
  },
  setCurrentSource: (value: string) => {
    if (value === state.sources.current) {
      return;
    }
    state.sources.current = value;
  },
  updateSource: (sourcename: string, value: SourceInfo) => {
    if (getObjectProperty(state.sources.info, sourcename)) {
      state.sources.info = setObjectProperty(state.sources.info, sourcename, value) as Record<string, SourceInfo>;
    }
  },
  updateSourceInfo: (value: Record<string, SourceInfoUpdate> | "error") => {
    if (value === "error") {
      state.realtimeActive = false;
      let info = state.sources.info;
      for (const k of Object.keys(info)) {
        info = setObjectProperty(info, k, { ...(getObjectProperty(info, k) as SourceInfo), status: "error" }) as Record<string, SourceInfo>;
      }
      state.sources.info = info;
    } else {
      let info = state.sources.info;
      let hasAny = false;
      for (const [k, source] of Object.entries(value)) {
        const existing = getObjectProperty(info, k) as SourceInfo | undefined;
        if (existing) {
          const used = Number(source.used) || 0;
          const total = Number(source.total) || 0;
          const updated: SourceInfo = {
            ...existing,
            used,
            usedAlt: source.usedAlt || 0,
            totalRoot: source.totalRoot || 0,
            usedAltRoot: source.usedAltRoot || 0,
            total,
            usedPercentage: total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0,
            usageScopeMismatch:
              source.usageScopeMismatch === true || (total > 0 && used > total),
            status: source.status || "unknown",
            name: source.name || k,
            files: source.numFiles || 0,
            folders: source.numDirs || 0,
            lastIndex: source.lastIndexedUnixTime || 0,
            quickScanDurationSeconds: source.quickScanDurationSeconds || 0,
            fullScanDurationSeconds: source.fullScanDurationSeconds || 0,
            complexity: source.complexity || 0,
            scanners: source.scanners || [],
            readOnly: source.readOnly || false,
            private: source.private || false,
            indexingDisabled: source.indexingDisabled || false,
          };
          info = setObjectProperty(info, k, updated) as Record<string, SourceInfo>;
          if (updated.total > 0 || updated.used > 0 || updated.usedAlt > 0) hasAny = true;
        }
      }
      state.sources.info = info;
      state.sources.hasSourceInfo = hasAny;
    }
  },
  setRealtimeActive: (value: boolean) => {
    if ( value === false ) {
      state.realtimeDownCount = state.realtimeDownCount + 1;
    } else {
      state.realtimeDownCount = 0;
    }
    if (value === state.realtimeActive) {
      return;
    }
    state.realtimeActive = value;
  },
  setSources: (user: UserObject) => {
    // Rebuilding sources from /api/users must not wipe stats already merged from
    // /api/settings/sources, SSE, or a prior validateLogin — otherwise the sidebar
    // shows "unknown"/red until a full page reload (common after OTP: initAuth then
    // router beforeResolve both refresh the user).
    const sameUser = user.username === state.user?.username;
    const prevInfo = sameUser && state.sources.info ? state.sources.info : {};
    let currentSource = user.scopes[0]?.name ?? "";
    if (
      sameUser &&
      state.sources.current &&
      user.scopes.some((s: UserScope) => s.name === state.sources.current)
    ) {
      currentSource = state.sources.current;
    }
    const sources: {
      info: Record<string, SourceInfo>;
      current: string;
      count: number;
      hasSourceInfo: boolean;
      defaultSource?: string;
    } = { info: {}, current: currentSource, count: user.scopes.length, hasSourceInfo: false };
    for (const source of user.scopes) {
      const prev = prevInfo[source.name];
      sources.info[source.name] = {
        pathPrefix: sources.count === 1 ? "" : encodeURIComponent(source.name),
        used: prev ? prev.used : 0,
        total: prev ? prev.total : 0,
        usedAlt: prev ? prev.usedAlt : 0,
        totalRoot: prev ? prev.totalRoot : 0,
        usedAltRoot: prev ? prev.usedAltRoot : 0,
        usedPercentage: prev ? prev.usedPercentage : 0,
        usageScopeMismatch: prev ? prev.usageScopeMismatch : false,
        status: prev ? prev.status : "unknown",
        name: prev ? prev.name : source.name,
        files: prev ? prev.files : 0,
        folders: prev ? prev.folders : 0,
        lastIndex: prev ? prev.lastIndex : 0,
        quickScanDurationSeconds: prev ? prev.quickScanDurationSeconds : 0,
        fullScanDurationSeconds: prev ? prev.fullScanDurationSeconds : 0,
        complexity: prev ? prev.complexity : 0,
        scanners: prev?.scanners ? [...prev.scanners] : [],
        readOnly: prev ? prev.readOnly : false,
        private: prev ? prev.private : false,
        indexingDisabled: prev ? prev.indexingDisabled : false,
      };
    }
    // Sidebar usage bar uses hasSourceInfo + per-source used/total; must survive object replace
    sources.hasSourceInfo = user.scopes.some((s: UserScope) => {
      const e = sources.info[s.name];
      return e && (e.total > 0 || e.used > 0 || e.usedAlt > 0);
    });
    // Check if user has custom sidebar links with sources
    let targetSource = sources.current;
    if (state.user?.sidebarLinks && state.user.sidebarLinks.length > 0) {
      // Find first source link in user's sidebar links
      const firstSourceLink = state.user.sidebarLinks.find(link =>
        isSourceSidebarCategory(link.category) && link.sourceName
      );
      if (firstSourceLink?.sourceName) {
        targetSource = firstSourceLink.sourceName;
      }
    }
    sources.defaultSource = targetSource;
    state.sources = sources;
  },
  setGallerySize: (value: number) => {
    if (value === state.user.gallerySize) {
      return;
    }
    state.user.gallerySize = value
    const isAnonymous = state.user.username === 'anonymous';
    if (!isAnonymous) {
      const encoded = url.base64Encode(state.user.username);
      localStorage.setItem(`GallerySize_${encoded}`, String(value));
    }
  },
  setActiveSettingsView: (value: string) => {
    if (value === state.activeSettingsView) {
      return;
    }
    state.activeSettingsView = value;
    // Update the hash in the URL without reloading or changing history state
    window.history.replaceState(history.state, "", `#${value}`);
    const container = document.getElementById("main");
    const element = document.getElementById(value);
    if (container && element) {
      const offset = 4 * parseFloat(getComputedStyle(document.documentElement).fontSize); // 4em in px
      const containerTop = container.getBoundingClientRect().top;
      const elementTop = element.getBoundingClientRect().top;
      const scrollOffset = elementTop - containerTop - offset;
      container.scrollTo({
        top: container.scrollTop + scrollOffset,
        behavior: "smooth",
      });
    }
  },
  setSettings: (value: StoreState['settings']) => {
    state.settings = value;
  },
  toggleDarkMode() {
    void mutations.updateCurrentUser({ "darkMode": !state.user.darkMode });
  },
  toggleSidebar() {
    state.showSidebar = !state.showSidebar;
  },
  closeSidebar() {
    if (!state.showSidebar) {
      return;
    }
    state.showSidebar = false;
  },
  setSidebarVisible(value: boolean) {
    if (value === state.showSidebar) {
      return;
    }
    state.showSidebar = value;
  },
  setIsUploading(value: boolean) {
    if (value === state.upload.isUploading) {
      return;
    }
    state.upload.isUploading = value;
    if (typeof window !== "undefined") {
      if (value) {
        window.addEventListener("beforeunload", uploadBeforeUnloadHandler);
      } else {
        window.removeEventListener("beforeunload", uploadBeforeUnloadHandler);
      }
    }
  },
  closeHovers: () => {
    // Close only specific lightweight/ephemeral hovers. Like ContextMenu, OverflowMenu, tooltip or sidebar (if not pinned and floating)
    const closeableHovers = ['ContextMenu', 'OverflowMenu'];
    state.prompts = state.prompts.filter(p => !closeableHovers.includes(p.name ?? ""));
    mutations.closeSidebar();
    mutations.hideTooltip();
  },
  closeTopPrompt: (id?: number, options?: { force?: boolean; skipCancel?: boolean }) => {
    if (!options?.force && state.prompts.length > 0) {
      const targetId =
        id ?? state.prompts[state.prompts.length - 1]?.id;
      const promptToClose = state.prompts.find((p) => p.id === targetId);
      if (
        promptToClose?.name === "upload" &&
        state.upload.isUploading &&
        !hasCloseWithActiveUploadsPrompt()
      ) {
        mutations.showPrompt({
          name: "CloseWithActiveUploads",
          pinned: true,
          confirm: () => {
            confirmCloseUploadPrompt(targetId);
          },
          cancel: () => {
            cancelCloseUploadPrompt();
          },
        });
        return;
      }
    }

    let closed: Prompt | undefined;
    if (id === undefined) {
      // close topmost prompt
      if (state.prompts.length === 0) return;
      mutations.closeHovers();
      closed = state.prompts[state.prompts.length - 1];
      state.prompts.pop();
    } else {
      // close prompt by ID
      const idx = state.prompts.findIndex(p => p.id === id);
      if (idx === -1) return;
      closed = state.prompts[idx];
      state.prompts.splice(idx, 1);
    }
    if (!options?.skipCancel) {
      const onCancel = closed?.cancel;
      if (typeof onCancel === "function") {
        onCancel();
      }
    }
    // Pre-existing: no top-level state.stickySidebar (only state.user.stickySidebar), so this
    // is always truthy-negated. Left as-is; see PR description.
    if (state.prompts.length === 0 && !(state as unknown as { stickySidebar?: boolean }).stickySidebar) {
      state.showSidebar = false;
    }
  },
  showPrompt: (value: Prompt | string) => {
    const prompt: Prompt = typeof value === "string" ? { name: value } : (value ?? {});
    state.promptIdCounter += 1;
    const id = state.promptIdCounter;
    // Set parentId to the current topmost prompt, unless the prompt is pinned.
    let parentId = prompt.parentId;
    if (!parentId && !prompt.pinned) {
      const topPrompt = state.prompts[state.prompts.length - 1];
      if (topPrompt) {
        parentId = topPrompt.id;
      }
    }
    const entry: Prompt = {
      id,
      name: prompt.name,
      parentId,
      pinned: prompt.pinned || false,
      confirm: prompt.confirm,
      action: prompt.action,
      props: prompt.props || {},
      discard: prompt.discard,
      cancel: prompt.cancel,
    };
    const pinnedCount = state.prompts.filter(p => p.pinned).length;
    const isContextMenu = entry.name === "ContextMenu" || entry.name === "OverflowMenu";
    if (entry.pinned || isContextMenu) {
      state.prompts.push(entry);
    } else {
      // Non‑pinned prompts go just before the first pinned prompt
      const insertIndex = state.prompts.length - pinnedCount;
      state.prompts.splice(insertIndex, 0, entry);
    }
    mutations.hideTooltip();
  },
  updatePromptTitle: (id: number, title: string) => {
    const prompt = state.prompts.find((p) => p.id === id);
    if (!prompt) {
      return;
    }
    if (prompt.props) {
      prompt.props.title = title;
    }
  },
  setLoading: (loadType: string, status: boolean) => {
    if (status === false) {
      state.loading = omitObjectProperty(state.loading, loadType);
    } else {
      state.loading = setObjectProperty(state.loading, loadType, true);
    }
  },
  setReload: (value: boolean) => {
    if (value === state.reload) {
      return;
    }
    state.reload = value;
  },
  setCurrentUser: async (value: Partial<UserObject> | null) => {
    try {
      // If value is null or undefined, emit state change and exit early
      if (!value) {
        state.user = null as unknown as UserObject;
        state.enforcedUserDefaults = {};
        state.sidebarLinkDefaultsPolicy = { items: [] };
        state.shareDefaultsPolicy = { values: {}, enforced: {} };
        state.toolAccessDefaultsPolicy = { items: [] };
        return;
      }
      if (value.username !== "anonymous") {
        mutations.setSources(value as UserObject);
      }
      // Ensure locale exists and is valid
      if (!value.locale) {
        value.locale = detectLocale();
      }
      await setLocale(value.locale);
      state.user = value as UserObject;
      state.user.sorting = {} as { by: string; asc: boolean };
      state.user.sorting.by = "name";
      state.user.sorting.asc = true;

      // Ensure fileLoading defaults are set
      if (!state.user.fileLoading) {
        state.user.fileLoading = {
          maxConcurrentUpload: 3,
          uploadChunkSizeMb: 5,
          downloadChunkSizeMb: 0,
          clearAll: false
        };
      } else {
        // Ensure each property has a default if missing
        if (state.user.fileLoading.maxConcurrentUpload === undefined) {
          state.user.fileLoading.maxConcurrentUpload = 3;
        }
        if (state.user.fileLoading.uploadChunkSizeMb === undefined) {
          state.user.fileLoading.uploadChunkSizeMb = 5;
        }
        if (state.user.fileLoading.clearAll === undefined) {
          state.user.fileLoading.clearAll = false;
        }
        if (state.user.fileLoading.downloadChunkSizeMb === undefined) {
          state.user.fileLoading.downloadChunkSizeMb = 0;
        }
      }

      // Load stored values or use defaults as fallback
      const isAnonymous = state.user.username === 'anonymous';
      const encoded = !isAnonymous ? url.base64Encode(state.user.username) : '';
      let viewMode = (!isAnonymous && localStorage.getItem(`ViewMode_${encoded}`)) || state.user.viewMode || 'normal';
      let gallerySize = (!isAnonymous && parseInt(localStorage.getItem(`GallerySize_${encoded}`) ?? '', 10)) || state.user.gallerySize || 3;
      gallerySize = Math.min(9, Math.max(1, gallerySize)); // ensure 1–9 since this is the slider min and max size

      // Normalize to use the view families too
      // for example if someone sets 'gallery' with size of 1 in the config file
      if (viewMode === 'gallery' || viewMode === 'icons') {
        viewMode = gallerySize <= 4 ? 'icons' : 'gallery';
      } else if (viewMode === 'list' || viewMode === 'compact') {
        viewMode = gallerySize <= 3 ? 'compact' : 'list';
      }

      state.user.viewMode = viewMode;
      state.user.gallerySize = gallerySize;

      if (!isAnonymous) {
        localStorage.setItem(`ViewMode_${encoded}`, viewMode);
        localStorage.setItem(`GallerySize_${encoded}`, String(gallerySize));
      }

      // Load display preferences for the current user
      const allPreferences = JSON.parse(localStorage.getItem("displayPreferences") || "{}");
      state.displayPreferences = allPreferences[state.user.username] || {};

    } catch (_error) {
      // Silently ignore errors when loading preferences
    }
  },
  syncEnforcedUserDefaults: async () => {
    if (
      !getters.isLoggedIn() ||
      getters.isShare() ||
      state.user?.username === "anonymous"
    ) {
      state.enforcedUserDefaults = {};
      return;
    }
    try {
      const data = await getEnforcedUserDefaults();
      state.enforcedUserDefaults = data.enforced || {};
    } catch {
      state.enforcedUserDefaults = {};
    }
  },
  /**
   * Load the effective media settings for playback. `globalVars` are static
   * page constants, so runtime changes to media settings are read from the API
   * (available to every authenticated user) instead.
   */
  syncMediaPlayerSettings: async () => {
    if (!getters.isLoggedIn() || getters.isShare() || state.user?.username === "anonymous") {
      state.mediaPlayerSettings = null;
      return;
    }
    try {
      const settings = await getMediaPlayerSettings();
      // The request can resolve after logout/share transition — don't write
      // media settings into a logged-out state.
      if (!getters.isLoggedIn() || getters.isShare() || state.user?.username === "anonymous") {
        return;
      }
      state.mediaPlayerSettings = settings;
    } catch {
      state.mediaPlayerSettings = null;
    }
  },
  syncSidebarLinkDefaultsPolicy: async () => {
    if (
      !getters.isLoggedIn() ||
      getters.isShare() ||
      getters.isAdmin() ||
      state.user?.username === "anonymous"
    ) {
      state.sidebarLinkDefaultsPolicy = { items: [] };
      return;
    }
    try {
      const data = await getSidebarLinkDefaultsPolicy();
      state.sidebarLinkDefaultsPolicy = {
        items: Array.isArray(data?.items) ? data.items : [],
        sources: Array.isArray(data?.sources) ? data.sources : [],
      };
    } catch {
      state.sidebarLinkDefaultsPolicy = { items: [] };
    }
  },
  applyToolAccessDefaultsPolicy: (items: unknown) => {
    state.toolAccessDefaultsPolicy = {
      items: Array.isArray(items) ? items : [],
    };
  },
  syncToolAccessDefaultsPolicy: async () => {
    if (
      !getters.isLoggedIn() ||
      getters.isShare() ||
      getters.isAdmin() ||
      state.user?.username === "anonymous"
    ) {
      state.toolAccessDefaultsPolicy = { items: [] };
      return;
    }
    try {
      const data = await getToolAccessDefaultsPolicy();
      state.toolAccessDefaultsPolicy = {
        items: Array.isArray(data?.items) ? data.items : [],
      };
    } catch {
      state.toolAccessDefaultsPolicy = { items: [] };
    }
  },
  syncShareDefaultsPolicy: async () => {
    if (
      !getters.isLoggedIn() ||
      getters.isShare() ||
      state.user?.username === "anonymous"
    ) {
      state.shareDefaultsPolicy = { values: {}, enforced: {} };
      return;
    }
    try {
      const data = await getShareDefaultsPolicy();
      state.shareDefaultsPolicy = {
        values: data?.values || {},
        enforced: data?.enforced || {},
      };
    } catch {
      state.shareDefaultsPolicy = { values: {}, enforced: {} };
    }
  },
  setShareData: (shareData: Partial<ShareInfoObject>) => {
    const newShare = { ...state.shareInfo, ...shareData };
    if (JSON.stringify(newShare) === JSON.stringify(state.shareInfo)) {
      return;
    }
    state.shareInfo = newShare;
    updateManifestLink();
  },
  clearShareData: () => {
    state.shareInfo = {
      isShare: false,
      disableThumbnails: false,
      hash: "",
      token: "",
      subPath: "",
      passwordValid: false,
      enforceDarkLightMode: "",
      disableSidebar: false,
      isValid: true,
      shareType: "",
      title: "",
      description: "",
    };
    updateManifestLink();
  },
  setSession: (value: string) => {
    if (value === state.sessionId) {
      return;
    }
    state.sessionId = value;
  },
  setMultiple: (value: boolean) => {
    if (value === state.multiple) return;
    state.multiple = value;
    if (value) {
      notify.showMultipleSelection();
    } else {
      notify.hideMultipleSelection();
    }
  },
  // The selection is a raw array that is replaced, never edited in place (see utils/selection.ts)
  addSelected: (value: number | FileListItem) => {
    state.selected = withEntry(state.selected, value);
  },
  /** Replace the whole selection in one go */
  setSelected: (values: (number | FileListItem)[]) => {
    state.selected = rawList(values);
    mutations.setMultiple(false);
  },
  removeSelected: (value: number | FileListItem) => {
    const next = withoutEntry(state.selected, value);
    if (next) state.selected = next;
  },
  resetSelected: () => {
    state.selected = rawList([]);
    state.lastSelectedIndex = null;
    mutations.setMultiple(false);
  },
  selectAllItems: (options = { multiple: true }) => {
    if (state.req?.items?.length > 0) {
      // Close hovers
      mutations.closeHovers();
      mutations.setSelected(state.req.items.map((_item, index) => index));
      if (options.multiple) {
        mutations.setMultiple(true);
      }
    }
  },
  setLastSelectedIndex: (index: number | null) => {
    if (index === state.lastSelectedIndex) {
      return;
    }
    state.lastSelectedIndex = index;
  },
  setRaw: (value: string) => {
    if (value === state.previewRaw) {
      return;
    }
    state.previewRaw = value;
  },
  updateCurrentUser: async (value: Partial<UserObject>) => {
    // Ensure the input is a valid object
    if (typeof value !== "object" || value === null) return;

    // Initialize state.user if it's null
    if (!state.user) {
      state.user = {} as UserObject;
    }
    // Store previous state for comparison
    const previousUser = { ...state.user };

    // Merge the new values into the current user state
    state.user = { ...state.user, ...value };

    const isAnonymous = state.user.username === 'anonymous';

    if (value.viewMode !== undefined && !isAnonymous) {
      const encoded = url.base64Encode(state.user.username);
      localStorage.setItem(`ViewMode_${encoded}`, value.viewMode);
    }
    if (value.gallerySize !== undefined && !isAnonymous) {
      const encoded = url.base64Encode(state.user.username);
      localStorage.setItem(`GallerySize_${encoded}`, String(value.gallerySize));
    }

    // Handle locale change
    if (state.user.locale !== previousUser.locale) {
      await setLocale(state.user.locale);
      localStorage.setItem("userLocale", state.user.locale);
    }
    // Update users if there's any change in state.user
    if (JSON.stringify(state.user) !== JSON.stringify(previousUser)) {
      const syncableProperties = [
        "locale",
        "dateFormat",
        "themeColor",
        "customTheme",
        "quickDownload",
        "preview",
        "stickySidebar",
        "singleClick",
        "darkMode",
        "showHidden",
        "sorting",
        "showFirstLogin",
        "sidebarLinks",
        "showToolsInSidebar",
        "fileLoading",
        "deleteAfterArchive",
        "deleteWithoutConfirming",
        "promptRightCloseButton",
        "preferEditorForMarkdown",
        "disablePreviewExt",
        "disableViewingExt",
        "disableOnlyOfficeExt",
        "hideFileExt",
        "disableQuickToggles",
        "disableSearchOptions",
        "hideSidebarFileActions",
        "showCopyPath",
        "showTypeColumn",
        "showCreationDateColumn",
        "hideFilesInTree",
        "showSelectMultiple",
        "debugOffice",
        "disableUpdateNotifications",
        "newFileTemplate",
        "viewMode",
        "gallerySize",
        "showAdvancedProfile",
      ];
      const updatedProperties = Object.keys(value).filter(
        (key) =>
          syncableProperties.includes(key) &&
          JSON.stringify(getObjectProperty(previousUser, key)) !==
            JSON.stringify(getObjectProperty(value, key)),
      );
      if (updatedProperties.length > 0) {
        try {
          await usersApi.update(state.user, updatedProperties);
        } catch (e) {
          state.user = { ...previousUser };
          if (
            value.locale !== undefined &&
            value.locale !== previousUser.locale
          ) {
            const prevLocale = previousUser.locale || "en";
            await setLocale(prevLocale);
            if (previousUser.locale) {
              localStorage.setItem("userLocale", previousUser.locale);
            } else {
              localStorage.removeItem("userLocale");
            }
          }
          notify.showError(e);
          throw e;
        }
      }
    }
    // Emit state change event
  },
  replaceRequest: (value: ReqObject) => {
    state.selected = rawList([]);
    state.lastSelectedIndex = null;
    mutations.setMultiple(false);
    // skip deep reactivity for parent dirs
    if (Array.isArray(value?.parentDirItems)) {
      value.parentDirItems = markRaw(value.parentDirItems);
    }
    if (!value?.items) {
      state.req = value;
      setActiveViewGrantScope(value?.source ?? "");
      return
    }
    const sorting = getters.sorting();
    const sortby = sorting.by;
    const asc = sorting.asc;
    // Separate directories and files
    const dirs = value.items.filter((item: FileListItem) => item.type === 'directory');
    const files = value.items.filter((item: FileListItem) => item.type !== 'directory');
    // Sort them separately
    const sortedDirs = sortedItems(dirs, sortby, asc);
    const sortedFiles = sortedItems(files, sortby, asc);
    value.items = markRaw([...sortedDirs, ...sortedFiles]);
    value.items.forEach((item: FileListItem & { index?: number }, index: number) => {
      item.index = index;
    });
    state.req = value;
    setActiveViewGrantScope(value?.source ?? "");
  },
  patchListingMetadata: (items: FileListItem[] | undefined, metadataMap: Map<string, unknown> | undefined, mergeForPath: string | null = null) => {
    if (!items?.length || !metadataMap?.size) {
      return;
    }
    const resolveMetadata = (item: FileListItem, metadata: unknown) =>
      item.path === mergeForPath
        ? { ...(metadata as Record<string, unknown>), ...(item.metadata as Record<string, unknown>) }
        : metadata;
    // state.req.items is markRaw so editing an item would not re-render its row.
    // For that, we swap the changed items for copies and replace the array
    if (items === state.req?.items) {
      let changed = false;
      const nextItems = items.map((item) => {
        if (!metadataMap.has(item.name)) return item;
        changed = true;
        return { ...item, metadata: resolveMetadata(item, metadataMap.get(item.name)) };
      });
      if (!changed) return;
      let sortedNext = nextItems;
      const { by, asc } = getters.sorting();
      if (by === "duration") {
        // sort once duration loads
        sortedNext = [
          ...sortedItems(nextItems.filter((i) => i.type === "directory"), by, asc),
          ...sortedItems(nextItems.filter((i) => i.type !== "directory"), by, asc),
        ];
        sortedNext.forEach((item: FileListItem & { index?: number }, index: number) => {
          item.index = index;
        });
        const newIndexByName = new Map(sortedNext.map((item, index) => [item.name, index]));
        const remap = (index: number) => {
          const name = items[index]?.name;
          return name !== undefined ? (newIndexByName.get(name) ?? index) : index;
        };
        state.selected = rawList(state.selected.map((entry) => (typeof entry === "number" ? remap(entry) : entry)));
        if (state.lastSelectedIndex !== null) {
          state.lastSelectedIndex = remap(state.lastSelectedIndex);
        }
      }
      state.req.items = markRaw(sortedNext);
      return;
    }
    // Other arrays (like preview navigation listing) are not reactive so in place is fine.
    for (const item of items) {
      if (!metadataMap.has(item.name)) continue;
      item.metadata = resolveMetadata(item, metadataMap.get(item.name));
    }
    navigationSortCache = null;
  },
  patchRequestMetadata: (metadataItems: Array<{ name: string; metadata: unknown }> | undefined) => {
    if (!state.req?.items || !metadataItems?.length) {
      return;
    }
    const byName = new Map();
    for (const e of metadataItems) {
      if (e.metadata !== null) {
        byName.set(e.name, e.metadata);
      }
    }
    let changed = false;
    const nextItems = state.req.items.map((item) => {
      if (!byName.has(item.name)) return item;
      changed = true;
      return { ...item, metadata: byName.get(item.name) };
    });
    if (!changed) return;
    state.req.items = markRaw(nextItems);
  },
  /** Merge media fields from GET /api/media/metadata (single file). Replace req object so Vue watchers see the update. */
  patchRequestFileMediaMetadata: (enriched: Partial<ReqObject> | null | undefined) => {
    if (
      !state.req ||
      state.req.type === "directory" ||
      !enriched ||
      enriched.type === "directory"
    ) {
      return;
    }
    const next = { ...state.req };
    if (enriched.metadata !== undefined) {
      next.metadata = enriched.metadata;
    }
    if (enriched.subtitles !== undefined) {
      next.subtitles = enriched.subtitles;
    }
    if (enriched.hasPreview !== undefined) {
      next.hasPreview = enriched.hasPreview;
    }
    state.req = next;
  },
  setRequestContent: (content: string) => {
    if (!state.req || state.req.content === content) {
      return;
    }
    state.req = { ...state.req, content };
  },
  setRequestViewToken: (viewToken: string) => {
    if (!state.req || !viewToken || state.req.viewToken === viewToken) {
      return;
    }
    state.req = { ...state.req, viewToken };
  },
  clearRequest: () => {
    // Set req to null to prevent API calls with empty paths
    // Components should check for null req before accessing
    state.req = null as unknown as ReqObject;
    state.selected = rawList([]);
    mutations.setMultiple(false);
  },
  setRoute: (value: RouteObject) => {
    if (value === state.route) {
      return;
    }
    state.route = value;
  },
  updateListingSortConfig: ({ field, asc }: { field: string; asc: boolean }) => {
    if (!state.user.sorting) {
      state.user.sorting = {} as { by: string; asc: boolean };
    }
    state.user.sorting.by = field;
    state.user.sorting.asc = asc;
    mutations.updateDisplayPreferences({ sorting: { by: field, asc: asc } });
  },
  updateNavigationSortConfig: ({ field, asc }: { field: string; asc: boolean }) => {
    state.navigation.sorting = { by: field, asc };
  },
  updatePickerSortConfig: ({ field, asc }: { field: string; asc: boolean }) => {
    state.pickerSorting = { by: field, asc };
    try {
      localStorage.setItem("pickerSorting", JSON.stringify(state.pickerSorting));
    } catch (error) {
      // localStorage can throw in private mode; log but don't block sorting
      console.error('Failed to save picker sorting:', error);
    }
  },
  updateListingItems: () => {
    mutations.replaceRequest(state.req);
  },
  updateClipboard: (value: StoreState['clipboard']) => {
    if (value.key === state.clipboard.key &&
        JSON.stringify(value.items) === JSON.stringify(state.clipboard.items) &&
        value.path === state.clipboard.path) {
      return;
    }
    state.clipboard.key = value.key;
    state.clipboard.items = value.items;
    state.clipboard.path = value.path;
  },
  resetClipboard: () => {
    state.clipboard.key = "";
    state.clipboard.items = [];
  },
  setSharePassword: (value: string) => {
    if (value === state.sharePassword) {
      return;
    }
    state.sharePassword = value;
  },
  setSearch: (value: boolean) => {
    if (value === state.isSearchActive) {
      return;
    }
    state.isSearchActive = value;
  },
  resetAll: () => {
    state.isSearchActive = false;
    state.selected = rawList([]);
    mutations.setMultiple(false);
  },
  showTooltip(value: Partial<StoreState['tooltip']>) {
    const useComponent = Boolean(value.component);
    state.tooltip.content = useComponent ? "" : (value.content ?? "");
    state.tooltip.component = value.component ? markRaw(value.component) : null;
    state.tooltip.componentProps = useComponent ? (value.componentProps ?? {}) : null;
    state.tooltip.x = value.x ?? 0;
    state.tooltip.y = value.y ?? 0;
    state.tooltip.width = value.width ?? null;
    state.tooltip.pointerEvents = value.pointerEvents ?? false;
    state.tooltip.show = true;
  },
  hideTooltip() {
    if (!state.tooltip.show) return;
    state.tooltip.show = false;
    state.tooltip.content = "";
    state.tooltip.component = null;
    state.tooltip.componentProps = null;
    state.tooltip.width = null;
    state.tooltip.pointerEvents = false;
  },
  setMaxConcurrentUpload: (value: number) => {
    if (!state.user.fileLoading) {
      state.user.fileLoading = {};
    }
    if (value === state.user.fileLoading.maxConcurrentUpload) {
      return;
    }
    state.user.fileLoading.maxConcurrentUpload = value;
  },
  updateDisplayPreferences: (payload: Partial<DisplayPreference>) => {
    let source = state.sources.current;
    if (getters.isShare()) {
      source = getters.currentHash();
    }
    const path = state.route.path;

    if (!source || path === null || path === "") return;

    let prefs = state.displayPreferences || {};

    let sourceLevel = getObjectProperty(prefs, source);
    if (!sourceLevel) sourceLevel = {};

    let pathLevel = getObjectProperty(sourceLevel, path) as Record<string, unknown> | undefined;
    if (!pathLevel) pathLevel = {};

    const newPathLevel = { ...pathLevel, ...payload };
    const newSourceLevel = setObjectProperty(sourceLevel, path, newPathLevel);
    prefs = setObjectProperty(prefs, source, newSourceLevel);
    state.displayPreferences = prefs;

    const isAnonymous = state.user.username === 'anonymous';
    if (!isAnonymous) {
      let allPrefs = JSON.parse(localStorage.getItem("displayPreferences") || "{}");
      allPrefs = setObjectProperty(allPrefs, state.user.username, prefs);
      localStorage.setItem("displayPreferences", JSON.stringify(allPrefs));
    }
  },
  setNavigationEnabled: (enabled: boolean) => {
    if (state.navigation.enabled === enabled) {
      if (!enabled && !state.navigation.isTransitioning && state.navigation.sorting) {
        state.navigation.sorting = null;
      }
      return;
    }
    state.navigation.enabled = enabled;
    if (!enabled) {
      if (!state.navigation.isTransitioning) {
        state.navigation.sorting = null;
      }
      mutations.clearNavigation();
    }
  },
  setupNavigation: ({ listing, currentItem, directoryPath, useItemPaths = false }: { listing: unknown; currentItem: Partial<FileListItem>; directoryPath: string; useItemPaths?: boolean }) => {
    const searchItems = getters.searchNavigationItems(currentItem);
    if (searchItems) {
      listing = searchItems;
      useItemPaths = true;
    }
    // Cancel any pending auto-hide from a previous setupNavigation; the raw
    // setTimeout used to leak and could fire setNavigationShow(false) right
    // after opening a new image (nextPrevious flicker).
    mutations.clearNavigationTimeout();
    state.navigation.listing = listing;
    state.navigation.currentIndex = -1;
    state.navigation.previousItem = null;
    state.navigation.nextItem = null;
    state.navigation.previousLink = "";
    state.navigation.nextLink = "";
    state.navigation.previousRaw = "";
    state.navigation.nextRaw = "";

    if (!listing || !currentItem) {
      return;
    }

    // Sort listing according to sorting preferences
    const sorting = getters.navigationSorting();
    const items = getSortedNavigationItems(listing as Partial<FileListItem>[], sorting.by, sorting.asc);

    // Find current item index in the listing
    const trimSlash = (p?: string) => (p || "").replace(/\/+$/, "");
    for (let i = 0; i < items.length; i++) {
      const candidate = items.at(i);
      const isCurrent = useItemPaths
        ? candidate?.source === currentItem.source && trimSlash(candidate?.path) === trimSlash(currentItem.path)
        : candidate?.name === currentItem.name;
      if (isCurrent) {
        state.navigation.currentIndex = i;
        break;
      }
    }

    if (state.navigation.currentIndex === -1) {
      return;
    }

    // Find previous item (skip directories)
    for (let i = state.navigation.currentIndex - 1; i >= 0; i--) {
      const item = items.at(i);
      if (!item || item.type === 'directory') continue;
      if (!useItemPaths) {
        item.path = url.joinPath(directoryPath, item.name);
      }
      state.navigation.previousItem = item;
      state.navigation.previousLink = url.buildItemUrl(item.source, item.path);

      if (getTypeInfo(item.type).simpleType === "image") {
        const prefetch = mutations.getPrefetchUrl(item);
        if (prefetch) {
          state.navigation.previousRaw = prefetch;
        }
      }
      break;
    }

    // Find next item (skip directories)
    for (let i = state.navigation.currentIndex + 1; i < items.length; i++) {
      const item = items.at(i);
      if (!item || item.type === 'directory') continue;
      if (!useItemPaths) {
        item.path = url.joinPath(directoryPath, item.name);
      }
      state.navigation.nextItem = item;
      state.navigation.nextLink = url.buildItemUrl(item.source, item.path);

      if (getTypeInfo(item.type).simpleType === "image") {
        const prefetch = mutations.getPrefetchUrl(item);
        if (prefetch) {
          state.navigation.nextRaw = prefetch;
        }
      }
      break;
    }


    // Auto-show navigation when it's first set up (timer tracked so new opens clear it)
    if (state.navigation.enabled && (state.navigation.previousLink || state.navigation.nextLink)) {
      mutations.setNavigationShow(true);
      const hideTimer = setTimeout(() => {
        if (!state.navigation.hoverNav) {
          mutations.setNavigationShow(false);
        }
      }, 3000);
      mutations.setNavigationTimeout(hideTimer);
    }
  },
  getPrefetchUrl: (item: Partial<FileListItem>) => {
    const viewToken = item.viewToken;
    const typeHint = item.type || item.name;
    if (getters.isShare()) {
      return resourcesApi.getViewURL(
        item.source,
        item.path,
        viewToken,
        {
          path: item.path,
          hash: state.shareInfo.hash,
        },
        false,
        typeHint,
      );
    }
    return resourcesApi.getViewURL(item.source, item.path, viewToken, null, false, typeHint);
  },
  setNavigationShow: (show: boolean) => {
    if (state.navigation.show === show) {
      return;
    }
    state.navigation.show = show;
  },
  showNavigationChromePersistent: () => {
    if (!state.navigation.enabled) {
      return;
    }
    mutations.clearNavigationTimeout();
    mutations.setNavigationShow(true);
  },
  toggleNavigationChrome: () => {
    if (!state.navigation.enabled) {
      return;
    }
    mutations.clearNavigationTimeout();
    mutations.setNavigationShow(!state.navigation.show);
  },
  /** Briefly reveal prev/next chrome (e.g. edge tap on video while gestures capture the event). */
  peekNavigationChrome: (side: 'left' | 'right' | null = null) => {
    if (!state.navigation.enabled) {
      return;
    }
    const queueNav = getters.isPreviewPlaybackQueueNavMode();
    const hasPrevious = queueNav
      ? getters.playbackQueueCanGoPrevious()
      : Boolean(state.navigation.previousLink);
    const hasNext = queueNav
      ? getters.playbackQueueCanGoNext()
      : Boolean(state.navigation.nextLink);
    if (side === 'left' && !hasPrevious) {
      return;
    }
    if (side === 'right' && !hasNext) {
      return;
    }
    if (!side && !hasPrevious && !hasNext) {
      return;
    }
    mutations.setNavigationShow(true);
    mutations.clearNavigationTimeout();
    const hideTimer = setTimeout(() => {
      if (!state.navigation.hoverNav) {
        mutations.setNavigationShow(false);
      }
      mutations.clearNavigationTimeout();
    }, 3000);
    mutations.setNavigationTimeout(hideTimer);
  },
  setNavigationHover: (hover: boolean) => {
    if (state.navigation.hoverNav === hover) {
      return;
    }
    state.navigation.hoverNav = hover;
  },
  setNavigationGestureHint: (payload: { kind?: null | 'previous' | 'next' | 'close', commitReady?: boolean, flashClose?: boolean } = {}) => {
    const kind = payload.kind ?? null;
    const commitReady = !!payload.commitReady;
    const flashClose = !!payload.flashClose;
    if (
      state.navigation.gestureHint === kind &&
      state.navigation.gestureHintCommitReady === commitReady &&
      state.navigation.gestureHintFlashClose === flashClose
    ) {
      return;
    }
    state.navigation.gestureHint = kind;
    state.navigation.gestureHintCommitReady = commitReady;
    state.navigation.gestureHintFlashClose = flashClose;
  },
  setNavigationTimeout: (timeout: ReturnType<typeof setTimeout> | null) => {
    if (state.navigation.timeout) {
      clearTimeout(state.navigation.timeout);
    }
    state.navigation.timeout = timeout;
  },
  clearNavigationTimeout: () => {
    if (state.navigation.timeout) {
      clearTimeout(state.navigation.timeout);
      state.navigation.timeout = null;
    }
  },
  clearNavigation: () => {
    state.navigation.show = false;
    state.navigation.hoverNav = false;
    state.navigation.gestureHint = null;
    state.navigation.gestureHintCommitReady = false;
    state.navigation.gestureHintFlashClose = false;
    state.navigation.listing = null;
    state.navigation.currentIndex = -1;
    state.navigation.previousItem = null;
    state.navigation.nextItem = null;
    state.navigation.previousLink = "";
    state.navigation.nextLink = "";
    state.navigation.previousRaw = "";
    state.navigation.nextRaw = "";
    mutations.clearNavigationTimeout();
  },
  setNavigationTransitioning: (isTransitioning: boolean) => {
    if (isTransitioning === state.navigation.isTransitioning) {
      return;
    }
    state.navigation.isTransitioning = isTransitioning;
    if (isTransitioning) {
      state.navigation.transitionStartTime = Date.now();
      // Safety timeout: if transition takes more than 5 seconds, clear it
      setTimeout(() => {
        if (state.navigation.isTransitioning &&
            state.navigation.transitionStartTime &&
            Date.now() - state.navigation.transitionStartTime > 5000) {
          mutations.setNavigationTransitioning(false);
        }
      }, 5500);
    } else {
      state.navigation.transitionStartTime = null;
    }
  },
  setPlaybackQueue: (playback: Partial<StoreState['playbackQueue']>) => {
    state.playbackQueue.queue = playback.queue || [];
    state.playbackQueue.currentIndex = playback.currentIndex ?? -1;
    state.playbackQueue.mode = playback.mode === 'sequential' || playback.mode === 'shuffle' || playback.mode === 'single'
      ? playback.mode
      : 'single';
    state.playbackQueue.loop = playback.loop || 'off';
    try {
      sessionStorage.setItem('playbackQueue', JSON.stringify({
        queue: state.playbackQueue.queue,
        currentIndex: state.playbackQueue.currentIndex,
        mode: state.playbackQueue.mode,
        loop: state.playbackQueue.loop,
      }));
    } catch (_) { /* ignore */ }
  },
  setPlaybackState: (isPlaying: boolean) => {
    if (isPlaying === state.playbackQueue.isPlaying) {
      return;
    }
    state.playbackQueue.isPlaying = isPlaying;
  },
  navigateToQueueIndex: (index: number) => {
    if (index < 0 || index >= state.playbackQueue.queue.length) return;
    const item = state.playbackQueue.queue.at(index);
    state.playbackQueue.currentIndex = index;
    // Update the current request to trigger navigation
    mutations.replaceRequest(item as ReqObject);
  },
  togglePlayPause: () => {
    state.playbackQueue.shouldTogglePlayPause = !state.playbackQueue.shouldTogglePlayPause;
  },
  setShareInfo: (shareInfo: Partial<ShareInfoObject>) => {
    const merged = { ...state.shareInfo, ...shareInfo };
    if (shareInfo.passwordValid === undefined && state.shareInfo.passwordValid !== undefined) {
      merged.passwordValid = state.shareInfo.passwordValid;
    }
    if (JSON.stringify(merged) === JSON.stringify(state.shareInfo)) {
      return;
    }
    state.shareInfo = merged;
    updateManifestLink();
  },
  setSidebarWidth: (value: number) => {
    // Ensure width is within bounds
    const minWidth = state.sidebar.minWidth;
    const maxWidth = state.sidebar.maxWidth;
    const newWidth = Math.max(minWidth, Math.min(value, maxWidth));
    if (newWidth === state.sidebar.width) {
      return;
    }
    state.sidebar.width = newWidth;
  },
  persistSidebarWidth: () => {
    localStorage.setItem("sidebarWidth", state.sidebar.width.toString());
  },
  setSidebarResizing: (value: boolean) => {
    if (value === state.sidebar.isResizing) {
      return;
    }
    state.sidebar.isResizing = value;
  },
  setSidebarMode(value: string) {
    const newMode = value === 'navigation' ? 'navigation' : 'links';
    if (newMode === state.sidebar.mode) return;
    state.sidebar.mode = newMode;
    localStorage.setItem('sidebarMode', newMode);
  },
};
