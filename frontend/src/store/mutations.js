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
import { filesApi } from "@/api";

export const mutations = {
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
    state.realtimeActive = value;
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
    state.isMobile = window.innerWidth <= 800
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
  setUpload(value) {
    state.upload = value;
    emitStateChanged();
  },
  setUsage: (source,value) => {
    state.usages[source] = value;
    emitStateChanged();
  },
  closeHovers: () => {
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
      });
    } else {
      state.prompts.push({
        name: value,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
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

      // Load display preferences for the current user
      const allPreferences = JSON.parse(localStorage.getItem("displayPreferences") || "{}");
      state.displayPreferences = allPreferences[state.user.username] || {};

    } catch (error) {
      console.log(error);
    }
    emitStateChanged();
  },
  setJWT: (value) => {
    if (value == state.jwt) {
      return;
    }
    state.jwt = value;
    emitStateChanged();
  },
  setShareData: (shareData) => {
    state.share = { ...state.share, ...shareData };
    emitStateChanged();
  },
  clearShareData: () => {
    state.share = {
      hash: null,
      token: "",
      subPath: "",
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
    state.lastSelectedIndex = index;
    emitStateChanged();
  },
  setRaw: (value) => {
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
      value.username = state.user.username;
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
  setRoute: (value) => {
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
    console.log("🔧 setNavigationEnabled:", enabled, "current:", state.navigation.enabled);
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
    console.log("🔧 setupNavigation called with:", { 
      listing: listing?.length, 
      currentItem: currentItem?.name, 
      directoryPath 
    });
    
    state.navigation.listing = listing;
    state.navigation.currentIndex = -1;
    state.navigation.previousItem = null;
    state.navigation.nextItem = null;
    state.navigation.previousLink = "";
    state.navigation.nextLink = "";
    state.navigation.previousRaw = "";
    state.navigation.nextRaw = "";

    if (!listing || !currentItem) {
      console.log("⚠️ setupNavigation: missing listing or currentItem");
      emitStateChanged();
      return;
    }

    // Find current item index in the listing
    for (let i = 0; i < listing.length; i++) {
      if (listing[i].name === currentItem.name) {
        state.navigation.currentIndex = i;
        console.log("📍 Found current item at index:", i, "name:", currentItem.name);
        break;
      }
    }

    if (state.navigation.currentIndex === -1) {
      console.log("⚠️ setupNavigation: current item not found in listing");
      emitStateChanged();
      return;
    }

    // Find previous item (skip directories)
    console.log("🔍 Looking for previous item from index:", state.navigation.currentIndex - 1);
    for (let j = state.navigation.currentIndex - 1; j >= 0; j--) {
      let item = listing[j];
      console.log("🔍 Checking item:", item.name, "type:", item.type);
      if (item.type === 'directory') continue;

      item.path = directoryPath + "/" + item.name;
      state.navigation.previousItem = item;
      state.navigation.previousLink = url.buildItemUrl(item.source, item.path);
      console.log("✅ Found previous item:", item.name, "link:", state.navigation.previousLink);

      if (getTypeInfo(item.type).simpleType === "image") {
        state.navigation.previousRaw = mutations.getPrefetchUrl(item);
      }
      break;
    }

    // Find next item (skip directories)
    console.log("🔍 Looking for next item from index:", state.navigation.currentIndex + 1);
    for (let j = state.navigation.currentIndex + 1; j < listing.length; j++) {
      let item = listing[j];
      console.log("🔍 Checking item:", item.name, "type:", item.type);
      if (item.type === 'directory') continue;

      item.path = directoryPath + "/" + item.name;
      state.navigation.nextItem = item;
      state.navigation.nextLink = url.buildItemUrl(item.source, item.path);
      console.log("✅ Found next item:", item.name, "link:", state.navigation.nextLink);

      if (getTypeInfo(item.type).simpleType === "image") {
        state.navigation.nextRaw = mutations.getPrefetchUrl(item);
      }
      break;
    }

    console.log("🎯 setupNavigation complete:", {
      previousLink: state.navigation.previousLink,
      nextLink: state.navigation.nextLink,
      enabled: state.navigation.enabled
    });

    emitStateChanged();
    
    // Auto-show navigation when it's first set up
    if (state.navigation.enabled && (state.navigation.previousLink || state.navigation.nextLink)) {
      console.log("🚀 Auto-showing navigation for 3 seconds");
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
      return filesApi.getDownloadURL(state.req.source, item.path, true);
    }
    return filesApi.getDownloadURL(state.req.source, item.path, true);
  },
  setNavigationShow: (show) => {
    console.log("🔧 setNavigationShow called:", show, "current:", state.navigation.show);
    if (state.navigation.show === show) {
      console.log("⏭️ setNavigationShow: no change needed");
      return;
    }
    state.navigation.show = show;
    console.log("✅ setNavigationShow: changed to", show);
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
};