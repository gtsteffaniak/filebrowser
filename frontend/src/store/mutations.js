import * as i18n from "@/i18n";
import { state } from "./state.js";
import { getters } from "./getters.js";
import { emitStateChanged } from './eventBus'; // Import the function from eventBus.js
import { usersApi } from "@/api";
import { notify } from "@/notify";
import { sortedItems } from "@/utils/sort.js";
import { serverHasMultipleSources } from "@/utils/constants.js";

export const mutations = {
  setMultiButtonState: (value) => {
    if (state.multiButtonLastState != value) {
      state.multiButtonLastState = state.multiButtonState;
    }
    state.multiButtonState = value;
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
    state.popupPreviewSource = value;
    emitStateChanged();
  },
  updateListing: (value) => {
    state.listing = value;
    emitStateChanged();
  },
  setCurrentSource: (value) => {
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
        pathPrefix: sources.count == 1 ? "" : source.name,
        used: 0,
        total: 0,
        usedPercentage: 0,
      };
    }
    state.sources = sources;
    emitStateChanged();
  },
  setGallerySize: (value) => {
    state.user.gallerySize = value
    emitStateChanged();
    usersApi.update(state.user, ['gallerySize']);
  },
  setActiveSettingsView: (value) => {
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
    if (state.user.stickySidebar) {
      localStorage.setItem("stickySidebar", "false");
      mutations.updateCurrentUser({ "stickySidebar": false }); // turn off sticky when closed
      state.showSidebar = false;
    } else {
      state.showSidebar = !state.showSidebar;
    }
    if (state.showSidebar) {
      state.multiButtonState = "back";
    } else {
      state.multiButtonState = "menu";
    }
    emitStateChanged();
  },
  closeSidebar() {
    if (state.showSidebar) {
      state.showSidebar = false;
      emitStateChanged();
    }
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
    const previousState = state.multiButtonLastState;
    state.multiButtonLastState = mutations.multiButtonState;
    state.multiButtonState = previousState;
    state.prompts = [];
    if (!state.stickySidebar) {
      state.showSidebar = false;
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
      if (value.username != "publicUser") {
        mutations.setSources(value);
      }
      // Ensure locale exists and is valid
      if (!value.locale) {
        value.locale = i18n.detectLocale();  // Default to detected locale if missing
      }
      state.user = value;
    } catch (error) {
      console.log(error);
    }
    emitStateChanged();
  },
  setJWT: (value) => {
    state.jwt = value;
    emitStateChanged();
  },
  setSession: (value) => {
    state.sessionId = value;
    emitStateChanged();
  },
  setMultiple: (value) => {
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

    // Update localStorage if stickySidebar exists
    if ('stickySidebar' in state.user) {
      localStorage.setItem("stickySidebar", state.user.stickySidebar);
      if (state.user.stickySidebar && getters.currentView() == "listingView") {
        state.multiButtonState = "menu";
      } else if (state.showSidebar) {
        state.multiButtonState = "back";
      }
    }

    // Update users if there's any change in state.user
    if (JSON.stringify(state.user) !== JSON.stringify(previousUser)) {
      usersApi.update(state.user, [
        "locale",
        "dateFormat",
        "themeColor",
        "quickDownload",
        "disableOnlyOfficeExt",
        "preview",
        "stickySidebar",
        "darkMode",
        "showHidden",
        "sorting",
        "gallerySize",
        "viewMode",
      ]);
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
    state.user.sorting.by = field;
    state.user.sorting.asc = asc;
    emitStateChanged();
  },
  updateListingItems: () => {
    state.req.items = sortedItems(state.req.items, state.user.sorting.by)
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
    state.isSearchActive = value;
    emitStateChanged();
  },
};

