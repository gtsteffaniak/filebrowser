import * as i18n from "@/i18n";
import { state } from "./state.js";
import { getters } from "./getters.js";
import { emitStateChanged } from './eventBus'; // Import the function from eventBus.js
import { usersApi } from "@/api";
import { notify } from "@/notify";
import { sortedItems } from "@/utils/sort.js";
import { serverHasMultipleSources } from "@/utils/constants.js";
import { url } from "@/utils";
import { getTypeInfo } from "@/utils/mimetype";
import { filesApi, publicApi } from "@/api";

export const mutations = {
  disableEventThemes: () => {
    if (state.disableEventThemes) {
      return;
    }
    localStorage.setItem("disableEventThemes", "true");
    state.disableEventThemes = true;
    // Set theme color back to user's preference or default
    if (state.user.themeColor) {
      document.documentElement.style.setProperty("--primaryColor", state.user.themeColor);
    } else {
      // Remove the override to use the default CSS variable
      document.documentElement.style.removeProperty("--primaryColor");
    }
    emitStateChanged();
  },
  setPreviousHistoryItem: (value) => {
    if (value == state.previousHistoryItem) {
      return;
    }
    state.previousHistoryItem = value;
    emitStateChanged();
  },
  setContextMenuHasItems: (value) => {
    if (value == state.contextMenuHasItems) {
      return;
    }
    state.contextMenuHasItems = value;
    emitStateChanged();
  },
  setEditorDirty: (value) => {
    if (value == state.editorDirty) {
      return;
    }
    state.editorDirty = value;
    emitStateChanged();
  },
  setEditorSaveHandler: (handler) => {
    state.editorSaveHandler = handler;
    emitStateChanged();
  },
  setDeletedItem: (value) => {
    if (value == state.deletedItem) {
      return;
    }
    state.deletedItem = value;
    emitStateChanged();
  },
  setSeenUpdate: (value) => {
    if (value == state.seenUpdate) {
      return;
    }
    state.seenUpdate = value
    localStorage.setItem("seenUpdate", value);
    emitStateChanged();
  },
  toggleOverflowMenu: () => {
    state.showOverflowMenu = !state.showOverflowMenu;
    emitStateChanged();
  },
  setWatchDirChangeAvailable() {
    state.req.hasUpdate = true;
  },
  setPreviewSource: (value) => {
    if (value === state.popupPreviewSource) {
      return;
    }
    state.popupPreviewSource = value;
    emitStateChanged();
  },
  updateListing: (value) => {
    if (value == state.listing) {
      return;
    }
    state.listing = value;
    emitStateChanged();
  },
  setCurrentSource: (value) => {
    if (value == state.sources.current) {
      return;
    }
    state.sources.current = value;
    emitStateChanged();
  },
  updateSource: (sourcename,value) => {
    if (state.sources.info[sourcename]) {
      state.sources.info[sourcename] = value;
    }
    emitStateChanged();
  },
  updateSourceInfo: (value) => {
    if (value == "error") {
      state.realtimeActive = false;
      for (const k of Object.keys(state.sources.info)) {
        state.sources.info[k].status = "error";
      }
    } else {
      for (const k of Object.keys(value)) {
        const source = value[k];
        if (state.sources.info[k]) {
          if (source.total == 0) {
            state.sources.hasSourceInfo = false
          } else {
            state.sources.hasSourceInfo = true
          }
          state.sources.info[k].used = source.used;
          state.sources.info[k].total = source.total;
          state.sources.info[k].usedPercentage = Math.round((source.used / source.total) * 100);
          state.sources.info[k].status = source.status;
          state.sources.info[k].name = source.name;
          state.sources.info[k].files = source.numFiles;
          state.sources.info[k].folders = source.numDirs;
          state.sources.info[k].lastIndex = source.lastIndexedUnixTime;
          state.sources.info[k].quickScanDurationSeconds = source.quickScanDurationSeconds;
          state.sources.info[k].fullScanDurationSeconds = source.fullScanDurationSeconds;
          state.sources.info[k].assessment = source.assessment;
        }
      }
    }
    emitStateChanged();
  },
  setRealtimeActive: (value) => {
    if ( value == false ) {
      state.realtimeDownCount = state.realtimeDownCount + 1;
    } else {
      state.realtimeDownCount = 0;
    }
    if (value === state.realtimeActive) {
      return;
    }
    state.realtimeActive = value;
    emitStateChanged();
  },
  setSources: (user) => {
    state.serverHasMultipleSources = serverHasMultipleSources;
    const currentSource = user.scopes.length > 0 ? user.scopes[0].name : "";
    let sources = {info: {}, current: currentSource, count: user.scopes.length};
    for (const source of user.scopes) {
      sources.info[source.name] = {
        pathPrefix: sources.count == 1 ? "" : encodeURIComponent(source.name),
        used: 0,
        total: 0,
        usedPercentage: 0,
      };
    }
    state.sources = sources;
    emitStateChanged();
  },
  setGallerySize: (value) => {
    if (value == state.user.gallerySize) {
      return;
    }
    state.user.gallerySize = value
    emitStateChanged();
    usersApi.update(state.user, ['gallerySize']);
  },
  setActiveSettingsView: (value) => {
    if (value == state.activeSettingsView) {
      return;
    }
    state.activeSettingsView = value;
    // Update the hash in the URL without reloading or changing history state
    window.history.replaceState(null, "", "#" + value);
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
    emitStateChanged();
  },
  setSettings: (value) => {
    state.settings = value;
    emitStateChanged();
  },
  setMobile() {
    const newValue = window.innerWidth <= 800;
    if (newValue === state.isMobile) {
      return;
    }
    state.isMobile = newValue;
    emitStateChanged();
  },
  toggleDarkMode() {
    mutations.updateCurrentUser({ "darkMode": !state.user.darkMode });
    emitStateChanged();
  },
  toggleSidebar() {
    state.showSidebar = !state.showSidebar;
    emitStateChanged();
  },
  closeSidebar() {
    if (!state.showSidebar) {
      return;
    }
    state.showSidebar = false;
    emitStateChanged();
  },
  setSidebarVisible(value) {
    if (value === state.showSidebar) {
      return;
    }
    state.showSidebar = value;
    emitStateChanged();
  },
  setIsUploading(value) {
    if (value === state.upload.isUploading) {
      return;
    }
    state.upload.isUploading = value;
    emitStateChanged();
  },
  setUsage: (source,value) => {
    state.usages[source] = value;
    emitStateChanged();
  },
  closeHovers: () => {
    // Check if uploads are active and we're not already showing the warning
    const hasActiveUploads = state.upload.isUploading;
    const hasUploadPrompt = state.prompts.some(p => p.name === "upload");
    const hasWarningPrompt = state.prompts.some(p => p.name === "CloseWithActiveUploads");

    if (hasActiveUploads && hasUploadPrompt && !hasWarningPrompt) {
      // Show warning prompt instead of closing
      mutations.showHover({
        name: "CloseWithActiveUploads",
        confirm: () => {
          // User confirmed to close anyway - force close all hovers
          state.prompts = [];
          if (!state.stickySidebar) {
            state.showSidebar = false;
          }
          emitStateChanged();
        },
        cancel: () => {
          // User cancelled - just close the warning prompt
          mutations.closeTopHover();
        },
      });
      return;
    }
    // Normal close behavior
    state.prompts = [];
    if (!state.stickySidebar) {
      state.showSidebar = false;
    }
    emitStateChanged();
  },
  closeTopHover: () => {
    state.prompts.pop();
    if (state.prompts.length === 0) {
      if (!state.stickySidebar) {
        state.showSidebar = false;
      }
    }
    emitStateChanged();
  },
  showHover: (value) => {
    if (typeof value === "object") {
      state.prompts.push({
        name: value?.name,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
        discard: value?.discard,
        cancel: value?.cancel,
      });
    } else {
      state.prompts.push({
        name: value,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
        discard: value?.discard,
        cancel: value?.cancel,
      });
    }
    emitStateChanged();
  },
  setLoading: (loadType, status) => {
    if (status === false) {
      delete state.loading[loadType];
    } else {
      state.loading = { ...state.loading, [loadType]: true };
    }
    emitStateChanged();
  },
  setReload: (value) => {
    if (value == state.reload) {
      return;
    }
    state.reload = value;
    emitStateChanged();
  },
  setCurrentUser: (value) => {
    try {
      // If value is null or undefined, emit state change and exit early
      if (!value) {
        state.user = value;
        emitStateChanged();
        return;
      }
      if (value.username !== "anonymous") {
        mutations.setSources(value);
      }
      // Ensure locale exists and is valid
      if (!value.locale) {
        value.locale = i18n.detectLocale();  // Default to detected locale if missing
      } else {
        i18n.setLocale(value.locale);
      }
      state.user = value;
      state.user.sorting = {};
      state.user.sorting.by = "name";
      state.user.sorting.asc = true;

      // Ensure fileLoading defaults are set
      if (!state.user.fileLoading) {
        state.user.fileLoading = {
          maxConcurrentUpload: 3,
          uploadChunkSizeMb: 5,
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
      }

      // Load display preferences for the current user
      const allPreferences = JSON.parse(localStorage.getItem("displayPreferences") || "{}");
      state.displayPreferences = allPreferences[state.user.username] || {};

    } catch (error) {
      // Silently ignore errors when loading preferences
    }
    emitStateChanged();
  },
  setShareData: (shareData) => {
    const newShare = { ...state.share, ...shareData };
    if (JSON.stringify(newShare) === JSON.stringify(state.share)) {
      return;
    }
    state.share = newShare;
    emitStateChanged();
  },
  clearShareData: () => {
    state.share = {
      hash: null,
      token: "",
      subPath: "",
      passwordValid: false,
    };
    emitStateChanged();
  },
  setSession: (value) => {
    if (value == state.sessionId) {
      return;
    }
    state.sessionId = value;
    emitStateChanged();
  },
  setMultiple: (value) => {
    if (value == state.multiple) {
      return;
    }
    state.multiple = value;
    if (value == true) {
      notify.showMultipleSelection()
    }
    emitStateChanged();
  },
  addSelected: (value) => {
    state.selected.push(value);
    emitStateChanged();
  },
  removeSelected: (value) => {
    let i = state.selected.indexOf(value);
    if (i === -1) return;
    state.selected.splice(i, 1);
    emitStateChanged();
  },
  resetSelected: () => {
    state.selected = [];
    mutations.setMultiple(false);
    emitStateChanged();
  },
  setLastSelectedIndex: (index) => {
    if (index === state.lastSelectedIndex) {
      return;
    }
    state.lastSelectedIndex = index;
    emitStateChanged();
  },
  setRaw: (value) => {
    if (value === state.previewRaw) {
      return;
    }
    state.previewRaw = value;
    emitStateChanged();
  },
  updateCurrentUser: (value) => {
    // Ensure the input is a valid object
    if (typeof value !== "object" || value === null) return;

    // Initialize state.user if it's null
    if (!state.user) {
      state.user = {};
    }
    // Store previous state for comparison
    const previousUser = { ...state.user };

    // Merge the new values into the current user state
    state.user = { ...state.user, ...value };
    // Handle locale change
    if (state.user.locale !== previousUser.locale) {
      //state.user.locale = i18n.detectLocale();
      i18n.setLocale(state.user.locale);
      i18n.default.locale = state.user.locale;
      localStorage.setItem("userLocale", state.user.locale);
    }
    // Update users if there's any change in state.user
    if (JSON.stringify(state.user) !== JSON.stringify(previousUser)) {
      // Only update the properties that were actually provided in the input
      const updatedProperties = Object.keys(value).filter(key =>
        [
          "locale",
          "dateFormat",
          "themeColor",
          "quickDownload",
          "preview",
          "stickySidebar",
          "singleClick",
          "darkMode",
          "showHidden",
          "sorting",
          "gallerySize",
          "viewMode",
        ].includes(key)
      );
      value.id = state.user.id;
      value.username = state.user?.username;
      if (updatedProperties.length > 0) {
        usersApi.update(value, updatedProperties);
      }
    }
    // Emit state change event
    emitStateChanged();
  },
  replaceRequest: (value) => {
    state.selected = [];
    if (!value?.items) {
      state.req = value;
      emitStateChanged();
      return
    }
    if (!state.user.showHidden) {
      value.items = value.items.filter((item) => !item.hidden);
    }
    let sortby = "name"
    let asc = true
    const sorting = getters.sorting();
    sortby = sorting.by;
    asc = sorting.asc;
    // Separate directories and files
    const dirs = value.items.filter((item) => item.type === 'directory');
    const files = value.items.filter((item) => item.type !== 'directory');

    // Sort them separately
    const sortedDirs = sortedItems(dirs, sortby, asc);
    const sortedFiles = sortedItems(files, sortby, asc);

    // Combine them and assign indices
    value.items = [...sortedDirs, ...sortedFiles];
    value.items.map((item, index) => {
      item.index = index;
      return item;
    })

    state.req = value;
    emitStateChanged();
  },
  clearRequest: () => {
    // Set req to null to prevent API calls with empty paths
    // Components should check for null req before accessing
    state.req = null;
    state.selected = [];
    emitStateChanged();
  },
  setRoute: (value) => {
    if (value === state.route) {
      return;
    }
    state.route = value;
    emitStateChanged();
  },
  updateListingSortConfig: ({ field, asc }) => {
    if (!state.user.sorting) {
      state.user.sorting = {};
    }
    mutations.updateDisplayPreferences({ sorting: { by: field, asc: asc } });
    emitStateChanged();
  },
  updateListingItems: () => {
    mutations.replaceRequest(state.req);
    emitStateChanged();
  },
  updateClipboard: (value) => {
    if (value.key === state.clipboard.key &&
        JSON.stringify(value.items) === JSON.stringify(state.clipboard.items) &&
        value.path === state.clipboard.path) {
      return;
    }
    state.clipboard.key = value.key;
    state.clipboard.items = value.items;
    state.clipboard.path = value.path;
    emitStateChanged();
  },
  resetClipboard: () => {
    state.clipboard.key = "";
    state.clipboard.items = [];
    emitStateChanged();
  },
  setSharePassword: (value) => {
    if (value === state.sharePassword) {
      return;
    }
    state.sharePassword = value;
    emitStateChanged();
  },
  setSearch: (value) => {
    if (value == state.isSearchActive) {
      return;
    }
    state.isSearchActive = value;
    emitStateChanged();
  },
  resetAll: () => {
    state.isSearchActive = false;
    state.selected = [];
    state.req = {};
    emitStateChanged();
  },
  showTooltip(value) {
    state.tooltip.content = value.content;
    state.tooltip.x = value.x;
    state.tooltip.y = value.y;
    state.tooltip.show = true;
    emitStateChanged();
  },
  hideTooltip() {
    if (!state.tooltip.show) {
      return;
    }
    state.tooltip.show = false;
    emitStateChanged();
  },
  setMaxConcurrentUpload: (value) => {
    if (!state.user.fileLoading) {
      state.user.fileLoading = {};
    }
    if (value === state.user.fileLoading.maxConcurrentUpload) {
      return;
    }
    state.user.fileLoading.maxConcurrentUpload = value;
    emitStateChanged();
  },
  updateViewModeHistory: ({ source, path, viewMode }) => {
    if (!source || !path) return;
    if (!state.viewModeHistory) {
      state.viewModeHistory = {};
    }
    if (!state.viewModeHistory[source]) {
      state.viewModeHistory[source] = {};
    }
    state.viewModeHistory[source][path] = viewMode;
    localStorage.setItem("viewModeHistory", JSON.stringify(state.viewModeHistory));
    emitStateChanged();
  },
  updateDisplayPreferences: (payload) => {
    let source = state.sources.current;
    if (getters.isShare()) {
      source = getters.currentHash();
    }
    const path = state.route.path;

    if (!source || !path) return;
    if (!state.displayPreferences) {
      state.displayPreferences = {};
    }
    if (!state.displayPreferences[source]) {
      state.displayPreferences[source] = {};
    }
    if (!state.displayPreferences[source][path]) {
      state.displayPreferences[source][path] = {};
    }

    state.displayPreferences[source][path] = {
      ...state.displayPreferences[source][path],
      ...payload,
    };

    const allPreferences = JSON.parse(localStorage.getItem("displayPreferences") || "{}");
    if (!allPreferences[state.user.username]) {
      allPreferences[state.user.username] = {};
    }
    allPreferences[state.user.username] = state.displayPreferences;
    localStorage.setItem("displayPreferences", JSON.stringify(allPreferences));

    emitStateChanged();
  },
  setNavigationEnabled: (enabled) => {
    if (state.navigation.enabled === enabled) {
      return;
    }
    state.navigation.enabled = enabled;
    if (!enabled) {
      mutations.clearNavigation();
    }
    emitStateChanged();
  },
  setupNavigation: ({ listing, currentItem, directoryPath }) => {
    state.navigation.listing = listing;
    state.navigation.currentIndex = -1;
    state.navigation.previousItem = null;
    state.navigation.nextItem = null;
    state.navigation.previousLink = "";
    state.navigation.nextLink = "";
    state.navigation.previousRaw = "";
    state.navigation.nextRaw = "";

    if (!listing || !currentItem) {
      emitStateChanged();
      return;
    }

    // Find current item index in the listing
    for (let i = 0; i < listing.length; i++) {
      if (listing[i].name === currentItem.name) {
        state.navigation.currentIndex = i;
        break;
      }
    }

    if (state.navigation.currentIndex === -1) {
      emitStateChanged();
      return;
    }

    // Find previous item (skip directories)
    for (let j = state.navigation.currentIndex - 1; j >= 0; j--) {
      let item = listing[j];
      if (item.type === 'directory') continue;

      item.path = url.joinPath(directoryPath, item.name);
      state.navigation.previousItem = item;
      state.navigation.previousLink = url.buildItemUrl(item.source, item.path);

      if (getTypeInfo(item.type).simpleType === "image") {
        state.navigation.previousRaw = mutations.getPrefetchUrl(item);
      }
      break;
    }

    // Find next item (skip directories)
    for (let j = state.navigation.currentIndex + 1; j < listing.length; j++) {
      let item = listing[j];
      if (item.type === 'directory') continue;

      item.path = url.joinPath(directoryPath, item.name);
      state.navigation.nextItem = item;
      state.navigation.nextLink = url.buildItemUrl(item.source, item.path);

      if (getTypeInfo(item.type).simpleType === "image") {
        state.navigation.nextRaw = mutations.getPrefetchUrl(item);
      }
      break;
    }

    emitStateChanged();

    // Auto-show navigation when it's first set up
    if (state.navigation.enabled && (state.navigation.previousLink || state.navigation.nextLink)) {
      mutations.setNavigationShow(true);
      setTimeout(() => {
        if (!state.navigation.hoverNav) {
          mutations.setNavigationShow(false);
        }
      }, 3000);
    }
  },
  getPrefetchUrl: (item) => {
    if (getters.isShare()) {
      return publicApi.getDownloadURL(
        {
          path: item.path,
          hash: state.share.hash,
          token: state.share.token,
        },
        [item.path],
        true,
      );
    }
    return filesApi.getDownloadURL(item.source, item.path, true);
  },
  setNavigationShow: (show) => {
    if (state.navigation.show === show) {
      return;
    }
    state.navigation.show = show;
    emitStateChanged();
  },
  setNavigationHover: (hover) => {
    if (state.navigation.hoverNav === hover) {
      return;
    }
    state.navigation.hoverNav = hover;
    emitStateChanged();
  },
  setNavigationTimeout: (timeout) => {
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
    state.navigation.listing = null;
    state.navigation.currentIndex = -1;
    state.navigation.previousItem = null;
    state.navigation.nextItem = null;
    state.navigation.previousLink = "";
    state.navigation.nextLink = "";
    state.navigation.previousRaw = "";
    state.navigation.nextRaw = "";
    mutations.clearNavigationTimeout();
    emitStateChanged();
  },
  setNavigationTransitioning: (isTransitioning) => {
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
    emitStateChanged();
  },
  setPlaybackQueue: (payload) => {
    state.playbackQueue.queue = payload.queue || [];
    state.playbackQueue.currentIndex = payload.currentIndex ?? -1;
    state.playbackQueue.mode = payload.mode || 'single';
    emitStateChanged();
  },
  setPlaybackState: (isPlaying) => {
    if (isPlaying === state.playbackQueue.isPlaying) {
      return;
    }
    state.playbackQueue.isPlaying = isPlaying;
    emitStateChanged();
  },
  navigateToQueueIndex: (index) => {
    if (index < 0 || index >= state.playbackQueue.queue.length) return;
    const item = state.playbackQueue.queue[index];
    state.playbackQueue.currentIndex = index;
    // Update the current request to trigger navigation
    mutations.replaceRequest(item);
    emitStateChanged();
  },
  togglePlayPause: () => {
    state.playbackQueue.shouldTogglePlayPause = !state.playbackQueue.shouldTogglePlayPause;
    emitStateChanged();
  },
};